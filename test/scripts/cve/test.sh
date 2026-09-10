#!/usr/bin/env bash
# Run CVE fix script unit tests inside dapper.
# Uses source tree (not installed scripts) since we're testing changes.
set -e
cd "$(dirname "$0")/../../.."
./skills/cve-fix/scripts/test-lib.sh
