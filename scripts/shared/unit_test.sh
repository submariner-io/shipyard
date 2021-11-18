#!/usr/bin/env bash

set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/find_functions

echo "Looking for packages to test"

modules=($(find_modules))

result=0

for module in "${modules[@]}"; do
    printf "Looking for tests in module %s\n" ${module}

    excluded_modules=""
    for exc_module in "${modules[@]}"; do
	if [ "$exc_module" != "$module" -a "$exc_module" != "." ]; then
	    excluded_modules+=" ${exc_module:2}"
	fi
    done

    packages="$(cd $module; find_unit_test_dirs "$excluded_modules" "$@" | tr '\n' ' ')"

    if [ -n "${packages}" ]; then
	echo "Running tests in ${packages}"
	[ "${ARCH}" == "amd64" ] && race="-race -vet=off"
	(cd $module && ${GO:-go} test -v ${race} -cover ${packages} -ginkgo.v -ginkgo.trace -ginkgo.reportPassed -ginkgo.reportFile junit.xml) || result=1
    fi
done

exit $result
