#!/bin/bash
set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/find_functions

PACKAGES="$(find_go_pkg_dirs) *.go"

# Show which golangci-lint linters are enabled/disabled
golangci-lint linters

golangci-lint run --timeout 5m $@

gitlint --commits origin/master..HEAD
