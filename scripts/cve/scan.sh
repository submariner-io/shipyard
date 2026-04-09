#!/bin/bash
# Scan for CVEs using grype.
# Usage: scan.sh STATE_FILE [--fresh] [--no-update] [--json]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

STATE_FILE="${1:?Usage: scan.sh STATE_FILE [--fresh] [--json]}"
shift
load_state "$STATE_FILE"

# Collect flags
GRYPE_FLAGS=()
SKIP_BUILD=false
for arg in "$@"; do
  case "$arg" in
    --fresh|--no-update|--json) GRYPE_FLAGS+=("$arg") ;;
    --skip-build) SKIP_BUILD=true ;;
    *) echo "Unknown flag: $arg" >&2; exit 1 ;;
  esac
done

# Build if needed (e.g., submariner repo with UPX compression for stdlib CVE detection)
if [[ "$NEEDS_BUILD_FOR_SCAN" == "true" ]] && [[ "$SKIP_BUILD" != "true" ]]; then
  if ! make BUILD_UPX=false build >&2; then
    echo "WARNING: Build failed. Scanning source only (may miss stdlib CVEs in binaries)." >&2
    echo "VPN can cause transient Docker DNS failures. Retry, or: sudo systemctl restart docker" >&2
  fi
fi

run_grype "${GRYPE_FLAGS[@]}"
