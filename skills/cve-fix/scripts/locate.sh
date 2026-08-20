#!/bin/bash
# Find a Go package in go.mod files, check for replace directives and dependencies.
# Usage: locate.sh STATE_FILE PACKAGE
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

STATE_FILE="${1:?Usage: locate.sh STATE_FILE PACKAGE}"
PACKAGE="${2:?Usage: locate.sh STATE_FILE PACKAGE}"
load_state "$STATE_FILE"

echo "=== Locating: $PACKAGE ==="

# Find in all go.mod files
FOUND_IN=""
while IFS= read -r GOMOD; do
  if grep -qF "$PACKAGE" "$GOMOD" 2>/dev/null; then
    FOUND_IN="${FOUND_IN:+$FOUND_IN }$GOMOD"
  fi
done < <(find_gomods)

if [[ -z "$FOUND_IN" ]]; then
  echo "ERROR: $PACKAGE not found in any go.mod" >&2
  exit 1
fi
echo "Found in: $FOUND_IN"

# Check for replace directives across all go.mod files
REPLACE_HITS=""
while IFS= read -r GOMOD; do
  HITS=$(grep "replace.*${PACKAGE}" "$GOMOD" 2>/dev/null || true)
  [[ -n "$HITS" ]] && REPLACE_HITS+="$GOMOD: $HITS"$'\n'
done < <(find_gomods)
if [[ -n "$REPLACE_HITS" ]]; then
  echo ""
  echo "REPLACE DIRECTIVES:"
  printf '%s' "$REPLACE_HITS"
fi

# Show dependency graph excerpt for each module containing the package
echo ""
echo "Dependency graph (may download module metadata)..."
for GOMOD in $FOUND_IN; do
  MODDIR=$(dirname "$GOMOD")
  [[ "$MODDIR" != "." ]] && echo "($MODDIR)"
  timeout 30 go -C "$MODDIR" mod graph 2>/dev/null | grep -F "$PACKAGE" | head -10 || true
done
