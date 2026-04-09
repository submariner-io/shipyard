#!/bin/bash
# End-to-end CVE fix: detect, scan, fix each CVE, agent review, summary.
# Usage: fix-all.sh [repo] [branch]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

# Clean up on exit: kill children, revert uncommitted changes, remove state file.
# Note: this only fires if fix-all.sh itself receives the signal. When a parent
# process (e.g., Claude agent shell) is killed without signal propagation, these
# children become orphans. That's a platform limitation — the script can't trap
# signals it never receives.
# shellcheck disable=SC2317,SC2329 # invoked via trap
cleanup() {
  kill -- -$$ 2>/dev/null || true
  # Dapper containers run in their own namespace and survive process group kill
  if [[ -n "${FIX_BRANCH:-}" ]]; then
    docker ps --format '{{.ID}} {{.Image}}' 2>/dev/null | grep -F ":${FIX_BRANCH}" | awk '{print $1}' | xargs -r docker kill 2>/dev/null || true
  fi
  git checkout -- . 2>/dev/null || true
  rm -f "${STATE_FILE:-}"
}
trap cleanup EXIT
trap 'exit 1' INT TERM

# --- Detect and Scan ---

# detect.sh prints state file path as last line (and its own banner)
STATE_FILE=$("$SCRIPT_DIR/detect.sh" "$@" --setup-branch | tee /dev/stderr | tail -1)
# shellcheck source=/dev/null
source "$STATE_FILE"
cd "$REPO"

echo ""
if [[ "$NEEDS_BUILD_FOR_SCAN" == "true" ]]; then
  echo "Cleaning build artifacts..."
  if ! make clean >/dev/null 2>&1; then
    echo "WARNING: make clean failed. Continuing with existing artifacts."
  fi
fi

echo "Scanning for CVEs..."
SCAN_JSON=$("$SCRIPT_DIR/scan.sh" "$STATE_FILE" --fresh --json || true)

if ! MATCH_COUNT=$(printf '%s\n' "$SCAN_JSON" | jq '.matches | length' 2>/dev/null) || [[ -z "$MATCH_COUNT" ]]; then
  echo "ERROR: Scan failed or produced invalid output." >&2
  echo "Check scanner availability (grype or docker/podman)." >&2
  exit 1
fi

if [[ "$MATCH_COUNT" -eq 0 ]]; then
  echo "No CVEs found. $(basename "$REPO")/$BRANCH is clean."
  if [[ -n "$FIX_BRANCH" ]]; then
    git checkout "$ORIGINAL_REF" 2>/dev/null
    git branch -D "$FIX_BRANCH" 2>/dev/null
  fi
  exit 0
fi

echo "Found $MATCH_COUNT CVE(s) in $(basename "$REPO")/$BRANCH."

# Show summary table from JSON
printf '%s\n' "$SCAN_JSON" | jq -r '
  ["NAME","INSTALLED","FIXED-IN","VULNERABILITY","SEVERITY"],
  (.matches[] | [.artifact.name, .artifact.version, (.vulnerability.fix.versions[0] // ""), .vulnerability.id, .vulnerability.severity]) |
  @tsv' 2>/dev/null | column -t || true

# --- Parse CVEs, group by package, pick highest fix version ---
banner "Fixing CVEs"

# Extract unique package+fixVersion+cveIDs groups from JSON
# jq outputs: PACKAGE TAB FIXED_IN TAB CVE_ID (one line per match)
CVE_LINES=$(printf '%s\n' "$SCAN_JSON" | jq -r '
  [.matches[] |
    select(.vulnerability.fix.versions != null and (.vulnerability.fix.versions | length) > 0) |
    {
      pkg: .artifact.name,
      fixedIn: .vulnerability.fix.versions[0],
      cve: .vulnerability.id,
      severity: .vulnerability.severity,
      type: (if .artifact.name == "stdlib" then "stdlib" else "package" end)
    }
  ] |
  group_by(.pkg) |
  map({
    pkg: .[0].pkg,
    type: .[0].type,
    fixedIn: (map(.fixedIn) | sort_by(split(".") | map(tonumber)) | last),
    cves: (map(.cve) | unique),
    severity: .[0].severity
  }) |
  .[] |
  "\(.type)\t\(.pkg)\t\(.fixedIn)\t\(.cves | join(","))\t\(.severity)"
' 2>/dev/null || echo "")

if [[ -z "$CVE_LINES" ]]; then
  echo "WARNING: Could not parse CVE data from JSON. All matches may lack fix versions."
  echo "Proceeding to agent review."
  CVE_LINES=""
fi

# Track results
FIX_SUMMARY=""
FIXED_COUNT=0
REVIEW_COUNT=0

while IFS=$'\t' read -r TYPE PKG FIX_VER CVE_CSV _SEVERITY; do
  [[ -z "$PKG" ]] && continue

  # Split CVE_CSV into array
  IFS=',' read -ra CVES <<< "$CVE_CSV"

  FIX_LOG=$(mktemp)
  if [[ "$TYPE" == "stdlib" ]]; then
    STDLIB_EXIT=0
    "$SCRIPT_DIR/fix-stdlib.sh" "$STATE_FILE" "$FIX_VER" "${CVES[@]}" 2>&1 | tee "$FIX_LOG" || STDLIB_EXIT=$?
    if [[ "$STDLIB_EXIT" -eq 0 ]]; then
      FIX_SUMMARY+="FIXED: stdlib go $FIX_VER for ${CVES[*]}"$'\n'
      FIXED_COUNT=$((FIXED_COUNT + 1))
    else
      FIX_SUMMARY+="NEEDS_REVIEW: stdlib — fix-stdlib.sh exited $STDLIB_EXIT"$'\n'
      REVIEW_COUNT=$((REVIEW_COUNT + 1))
    fi
  else
    EXIT_CODE=0
    "$SCRIPT_DIR/fix-package.sh" "$STATE_FILE" "$PKG" "$FIX_VER" "${CVES[@]}" 2>&1 | tee "$FIX_LOG" || EXIT_CODE=$?
    case $EXIT_CODE in
      0)
        FIX_SUMMARY+="FIXED: $PKG v$FIX_VER for ${CVES[*]}"$'\n'
        FIXED_COUNT=$((FIXED_COUNT + 1))
        ;;
      2|3)
        REASON=$(grep "^NEEDS_REVIEW:" "$FIX_LOG" || echo "NEEDS_REVIEW: $PKG — exit code $EXIT_CODE")
        FIX_SUMMARY+="$REASON"$'\n'
        REVIEW_COUNT=$((REVIEW_COUNT + 1))
        ;;
      *)
        FIX_SUMMARY+="ERROR: $PKG — fix-package.sh failed with exit code $EXIT_CODE"$'\n'
        REVIEW_COUNT=$((REVIEW_COUNT + 1))
        ;;
    esac
  fi
  rm -f "$FIX_LOG"
done <<< "$CVE_LINES"

# Verify
echo ""
echo "Running unit tests..."
if ! make unit; then
  FIX_SUMMARY+="NEEDS_REVIEW: unit tests failed after applying fixes"$'\n'
  REVIEW_COUNT=$((REVIEW_COUNT + 1))
fi
echo "Final scan..."
FINAL_SCAN=$("$SCRIPT_DIR/scan.sh" "$STATE_FILE" --no-update --skip-build 2>&1) || true
printf '%s\n' "$FINAL_SCAN"

# Agent review (pass scan output to avoid re-scanning)
banner "Agent Review"
"$SCRIPT_DIR/review.sh" "$STATE_FILE" "$FIX_SUMMARY" "$FINAL_SCAN" || true

# --- Summary ---

banner "Summary: $(basename "$REPO")/$BRANCH"

# shellcheck source=/dev/null
source "$STATE_FILE"

echo "Fixed: $FIXED_COUNT, Needs review: $REVIEW_COUNT"
printf '%s' "$FIX_SUMMARY"

COMMIT_COUNT=$(git --no-pager log "origin/$BRANCH"..HEAD --oneline 2>/dev/null | wc -l)
if [[ "$COMMIT_COUNT" -gt 0 ]]; then
  echo "Commits:"
  git --no-pager log "origin/$BRANCH"..HEAD --oneline

  # Generate PR command
  echo ""
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  BASE_BRANCH=$(echo "$CURRENT_BRANCH" | sed 's/fix-\([0-9.]*\)-.*/release-\1/; s/fix-devel-.*/devel/')
  PLURAL=$([[ "$COMMIT_COUNT" -eq 1 ]] && echo "" || echo "s")
  FORK_REMOTE=$(git remote -v | awk '!/submariner-io/ && /\(push\)/ { print $1; exit }')
  FORK_USER=$(git remote get-url "${FORK_REMOTE}" 2>/dev/null | sed -E 's#.*github.com[:/]+([^/]+)/.*#\1#')

  echo "PR command:"
  echo "git push $FORK_REMOTE $CURRENT_BRANCH && \\"
  echo "gh pr create \\"
  echo "  --title \"Fix CVE${PLURAL} in ${BASE_BRANCH}\" \\"
  echo "  --body \"See commit message${PLURAL} for details.\" \\"
  echo "  --base \"${BASE_BRANCH}\" \\"
  echo "  --head \"${FORK_USER}:${CURRENT_BRANCH}\" \\"
  echo "  --assignee \"@me\""
fi

if [[ "$FETCH_FAILED" == "true" ]]; then
  echo ""
  echo "WARNING: git fetch failed earlier. Branch was created from cached remote state."
  echo "Run 'git fetch' and re-run if it fetches new commits."
fi

# Exit code based on results
if [[ "$REVIEW_COUNT" -gt 0 ]]; then
  exit 2
fi
exit 0
