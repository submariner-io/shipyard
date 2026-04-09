#!/bin/bash
# Fix a single CVE package: update, verify, commit.
# Usage: fix-package.sh STATE_FILE PACKAGE VERSION CVE_ID [CVE_ID...]
# Exit 0: fixed. Exit 2: needs review (breaking change). Exit 3: CVE persists.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

STATE_FILE="${1:?Usage: fix-package.sh STATE_FILE PACKAGE VERSION CVE_ID [CVE_ID...]}"
PACKAGE="${2:?Missing PACKAGE}"
VERSION="${3:?Missing VERSION}"
shift 3
CVE_IDS=("$@")
[[ ${#CVE_IDS[@]} -gt 0 ]] || { echo "ERROR: At least one CVE_ID required" >&2; exit 1; }

load_state "$STATE_FILE"

trap 'git reset --quiet HEAD -- . 2>/dev/null || true; git checkout -- . 2>/dev/null || true' ERR

echo "--- Fixing: $PACKAGE -> v$VERSION for ${CVE_IDS[*]} ---"

# Check for replace directives across all go.mod files (warn, don't block)
while IFS= read -r GOMOD; do
  if grep -q "replace.*${PACKAGE}" "$GOMOD" 2>/dev/null; then
    echo "WARNING: Replace directive found in $GOMOD for $PACKAGE"
    grep "replace.*${PACKAGE}" "$GOMOD" 2>/dev/null || true
  fi
done < <(find_gomods)

# Snapshot go directives before update (for breaking-change detection)
GO_BEFORE=""
while IFS= read -r GOMOD; do
  GO_VER=$(grep '^go ' "$GOMOD" 2>/dev/null | awk '{print $2}')
  [[ -n "$GO_VER" ]] && GO_BEFORE+="$GOMOD:$GO_VER "
done < <(find_gomods)
# shellcheck disable=SC2046 # word splitting is intentional (multiple file args)
K8S_BEFORE=$(grep -h 'k8s.io/client-go' $(find_gomods) 2>/dev/null | grep -oP 'v0\.\K[0-9]+' | sort -un | tr '\n' ' ')

# Update in all go.mod files that contain this package
while IFS= read -r GOMOD; do
  MODDIR=$(dirname "$GOMOD")
  if grep -qF "$PACKAGE" "$GOMOD" 2>/dev/null; then
    echo "Updating $PACKAGE in $GOMOD..."
    go -C "$MODDIR" get "${PACKAGE}@v${VERSION}" && go -C "$MODDIR" mod tidy
  fi
done < <(find_gomods)

clean_gomod

# Check for breaking changes (Go or K8s minor version upgrade in any go.mod)
BREAKING=""
while IFS= read -r GOMOD; do
  GO_AFTER=$(grep '^go ' "$GOMOD" 2>/dev/null | awk '{print $2}')
  [[ -z "$GO_AFTER" ]] && continue
  # Find the before version for this go.mod
  for PAIR in $GO_BEFORE; do
    if [[ "${PAIR%%:*}" == "$GOMOD" ]]; then
      GO_WAS="${PAIR#*:}"
      if [[ "$(echo "$GO_WAS" | cut -d. -f1-2)" != "$(echo "$GO_AFTER" | cut -d. -f1-2)" ]]; then
        BREAKING="${BREAKING:+$BREAKING; }$GOMOD: Go $GO_WAS -> $GO_AFTER"
      fi
      break
    fi
  done
done < <(find_gomods)

# shellcheck disable=SC2046 # word splitting is intentional (multiple file args)
K8S_AFTER=$(grep -h 'k8s.io/client-go' $(find_gomods) 2>/dev/null | grep -oP 'v0\.\K[0-9]+' | sort -un | tr '\n' ' ')
if [[ -n "$K8S_BEFORE" ]] && [[ -n "$K8S_AFTER" ]] && [[ "$K8S_BEFORE" != "$K8S_AFTER" ]]; then
  BREAKING="${BREAKING:+$BREAKING; }K8s minor versions changed"
fi

if [[ -n "$BREAKING" ]]; then
  echo "NEEDS_REVIEW: $PACKAGE — would upgrade $BREAKING"
  git checkout -- . || echo "ERROR: Could not rollback changes" >&2
  exit 2
fi

# Verify fix: check that go.mod has the new version
STILL_VULNERABLE=false
while IFS= read -r GOMOD; do
  if grep -q "${PACKAGE}.*v${VERSION}" "$GOMOD" 2>/dev/null; then
    : # Updated to new version, good
  elif grep -qF "$PACKAGE" "$GOMOD" 2>/dev/null; then
    echo "WARNING: $GOMOD still has old version of $PACKAGE"
    STILL_VULNERABLE=true
  fi
done < <(find_gomods)

if [[ "$STILL_VULNERABLE" == "true" ]]; then
  echo "NEEDS_REVIEW: $PACKAGE — CVE persists after update to v$VERSION"
  git checkout -- . || echo "ERROR: Could not rollback changes" >&2
  exit 3
fi

# Stage all changed go.mod/go.sum files (some repos have extra modules like coredns/)
git diff --name-only | grep -E 'go\.(mod|sum)$' | xargs -r git add || true

# Handle generated files
if [[ -n "$GENERATED_FILE" ]] && [[ -n "$DIFF_IGNORE_ARGS" ]]; then
  # shellcheck disable=SC2086 # DIFF_IGNORE_ARGS needs word splitting (-I'pattern')
  if git diff $DIFF_IGNORE_ARGS "$GENERATED_FILE" 2>/dev/null | grep -q .; then
    git add "$GENERATED_FILE"
  else
    git checkout "$GENERATED_FILE" 2>/dev/null || true
  fi
fi

# Determine if tools-only change (all staged go files under tools/)
TOOLS_ONLY=""
if ! git diff --staged --name-only | grep -qE '^go\.(mod|sum)$' && \
     git diff --staged --name-only | grep -qE '^tools/'; then
  TOOLS_ONLY=" in /tools"
fi

# Format commit message
ABBREV=$(abbreviate_package "$PACKAGE")
if [[ ${#CVE_IDS[@]} -eq 1 ]]; then
  SUBJECT="Bump ${ABBREV} for ${CVE_IDS[0]}${TOOLS_ONLY}"
  BODY="Full package: $PACKAGE"
else
  SUBJECT="Bump ${ABBREV} for CVEs${TOOLS_ONLY}"
  FIXES=$(printf '%s, ' "${CVE_IDS[@]}")
  BODY="Full package: $PACKAGE
Fixes: ${FIXES%, }"
fi

git commit -s -m "$(printf '%s\n\n%s' "$SUBJECT" "$BODY")"

echo "FIXED: $PACKAGE v$VERSION for ${CVE_IDS[*]}"
