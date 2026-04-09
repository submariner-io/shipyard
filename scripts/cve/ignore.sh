#!/bin/bash
# Add CVE(s) to the .grype.yaml ignore list and commit.
# Usage: ignore.sh STATE_FILE PACKAGE SEVERITY REASON CVE_ID [CVE_ID...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

STATE_FILE="${1:?Usage: ignore.sh STATE_FILE PACKAGE SEVERITY REASON CVE_ID [CVE_ID...]}"
PACKAGE="${2:?Missing PACKAGE}"
SEVERITY="${3:?Missing SEVERITY}"
REASON="${4:?Missing REASON}"
shift 4
CVE_IDS=("$@")
[[ ${#CVE_IDS[@]} -gt 0 ]] || { echo "ERROR: At least one CVE_ID required" >&2; exit 1; }

load_state "$STATE_FILE"

echo "--- Ignoring: ${CVE_IDS[*]} ($PACKAGE) [$SEVERITY] ---"

for CVE_ID in "${CVE_IDS[@]}"; do
  insert_grype_ignore .grype.yaml "$CVE_ID" "$PACKAGE" "$REASON"
done

git add .grype.yaml

ABBREV=$(abbreviate_package "$PACKAGE")

git commit -s -m "$(cat <<EOF
Ignore $ABBREV CVEs incompatible with $BRANCH

$REASON
EOF
)"

echo "IGNORED: ${CVE_IDS[*]} ($PACKAGE) — $REASON"
