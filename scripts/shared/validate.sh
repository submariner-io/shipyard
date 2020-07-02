#!/bin/bash
set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/find_functions

PACKAGES="$(find_go_pkg_dirs) *.go"

# TODO: Use goimports via golangci-lint, enable it via per-repo config files
if [[ $(goimports -l ${PACKAGES} | wc -l) -gt 0 ]]; then
    echo "Incorrect formatting"
    echo "These are the files with formatting errors:"
    goimports -l ${PACKAGES}
    echo "These are the formatting errors:"
    goimports -d ${PACKAGES}
    exit 1
fi

# Show which golangci-lint linters are enabled/disabled
golangci-lint linters

golangci-lint run --timeout 5m $@
