#!/usr/bin/env bash

set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/find_functions

echo "Looking for packages to test"

# This can’t be done as simply with parameter substitution
# shellcheck disable=SC2001
packages="$(find_go_pkg_dirs "$@" | sed -e 's![^ ]*!./&/...!g')"

echo "Running tests in ${packages}"
[ "${ARCH}" == "amd64" ] && race=-race
go test -v ${race} -cover ${packages} -ginkgo.trace -ginkgo.reportPassed -ginkgo.reportFile junit.xml
