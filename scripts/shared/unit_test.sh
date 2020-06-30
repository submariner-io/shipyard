#!/usr/bin/env bash

set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/find_functions

echo "Looking for packages to test"

packages="$(find_unit_test_dirs "$@")"

echo "Running tests in ${packages}"
[ "${ARCH}" == "amd64" ] && race=-race
go test -v ${race} -cover ${packages} -ginkgo.trace -ginkgo.reportPassed -ginkgo.reportFile junit.xml
