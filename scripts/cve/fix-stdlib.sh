#!/bin/bash
# Fix stdlib CVEs by updating the go directive.
# Usage: fix-stdlib.sh STATE_FILE GO_VERSION CVE_ID [CVE_ID...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

STATE_FILE="${1:?Usage: fix-stdlib.sh STATE_FILE GO_VERSION CVE_ID [CVE_ID...]}"
GO_VERSION="${2:?Missing GO_VERSION}"
shift 2
CVE_IDS=("$@")
[[ ${#CVE_IDS[@]} -gt 0 ]] || { echo "ERROR: At least one CVE_ID required" >&2; exit 1; }

load_state "$STATE_FILE"

trap 'git reset --quiet HEAD -- . 2>/dev/null || true; git checkout -- . 2>/dev/null || true' ERR

echo "--- Fixing stdlib: go $GO_VERSION for ${CVE_IDS[*]} ---"

OLD_GO=$(grep '^go ' go.mod | awk '{print $2}')

# Check for Go minor version upgrade (breaking on stable branches)
OLD_MINOR=$(echo "$OLD_GO" | cut -d. -f1-2)
NEW_MINOR=$(echo "$GO_VERSION" | cut -d. -f1-2)
if [[ "$OLD_MINOR" != "$NEW_MINOR" ]]; then
  echo "NEEDS_REVIEW: stdlib — would upgrade Go $OLD_GO -> $GO_VERSION (minor version change)"
  exit 2
fi

# Check host Go version is sufficient
HOST_GO=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+\.[0-9]+')
if [[ -n "$HOST_GO" ]] && [[ "${GOTOOLCHAIN:-}" != *auto* ]] && \
   [[ "$(printf '%s\n' "$HOST_GO" "$GO_VERSION" | sort -V | head -1)" == "$HOST_GO" ]] && \
   [[ "$HOST_GO" != "$GO_VERSION" ]]; then
  echo "NEEDS_REVIEW: stdlib — host Go $HOST_GO is older than required $GO_VERSION"
  echo "Update Go: go install golang.org/dl/go${GO_VERSION}@latest && go${GO_VERSION} download"
  echo "Or set GOTOOLCHAIN=auto to allow automatic toolchain downloads."
  exit 2
fi

# Update go directive in all go.mod files
echo "Updating go directive and tidying modules..."
while IFS= read -r GOMOD; do
  MODDIR=$(dirname "$GOMOD")
  go -C "$MODDIR" mod edit -go="${GO_VERSION}"
  go -C "$MODDIR" mod tidy
done < <(find_gomods)

clean_gomod

# Check if Shipyard build image has sufficient Go version
if [[ "$SHIPYARD_GO_VERSION" != "unknown" ]]; then
  SHIPYARD_GO=$(echo "$SHIPYARD_GO_VERSION" | grep -oP '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
  if [[ -n "$SHIPYARD_GO" ]]; then
    OLDEST=$(printf '%s\n' "$SHIPYARD_GO" "$GO_VERSION" | sort -V | head -1)
    if [[ "$OLDEST" == "$SHIPYARD_GO" ]] && [[ "$SHIPYARD_GO" != "$GO_VERSION" ]]; then
      echo "NOTE: Shipyard build image has Go $SHIPYARD_GO but go.mod now requires $GO_VERSION"
      echo "CI may fail until Shipyard is updated with a newer Go version."
    fi
  fi
fi

# Stage all changed go.mod/go.sum files
git diff --name-only | grep -E 'go\.(mod|sum)$' | xargs -r git add || true

FIXES=$(printf '%s, ' "${CVE_IDS[@]}")
FIXES="${FIXES%, }"
git commit -s -m "$(cat <<EOF
Bump Go to $GO_VERSION for stdlib CVEs

Updates Go requirement from $OLD_GO to $GO_VERSION to address
stdlib vulnerabilities.

Fixes: $FIXES
EOF
)"

echo "FIXED: stdlib go $GO_VERSION for ${CVE_IDS[*]}"
