#!/usr/bin/env bash

set -e

source "${SCRIPTS_DIR}/lib/debug_functions"

function _build_find_exclude() {
    local find_exclude
    excluded_dirs+=" vendor .git .trash-cache bin"

    for dir in $excluded_dirs; do
        find_exclude+=" -path ./$dir -prune -o"
    done

    echo "${find_exclude}"
}

function _find_pkg_dirs() {
    # shellcheck disable=SC2046
    find . $(_build_find_exclude) -path "$1" -printf "%h\0" | sort -z -u
}

function find_modules() {
    # shellcheck disable=SC2046
    find . $(_build_find_exclude) -name go.mod -printf "%h\0" | sort -z -u
}

function find_unit_test_dirs() {
    local excluded_dirs="${*}"
    _find_pkg_dirs "./*/*_test.go"
}

echo "Looking for packages to test"

readarray -d '' modules < <(find_modules)

result=0

for module in "${modules[@]}"; do
    printf "Looking for tests in module %s\n" "${module}"

    excluded_modules=""
    for exc_module in "${modules[@]}"; do
        if [ "$exc_module" != "$module" ] && [ "$exc_module" != "." ]; then
            excluded_modules+=" ${exc_module:2}"
        fi
    done

    # Run in subshell to return to base directory in any case the subshell exits
    (
        cd "$module"
        readarray -d '' packages < <(find_unit_test_dirs "$excluded_modules" "$@")
        [ "${#packages[@]}" -gt 0 ] || exit 0

        echo "Running tests in ${packages[*]}"
        [ "${ARCH}" == "amd64" ] && race=-race
        # It's important that the `go test` command's exit status is reported from this () block.
        # Can't be one command (with -cover). Need detailed -coverprofile for Sonar and summary to console.
        if go mod graph | grep 'submariner.*ginkgo/v2' >/dev/null; then
            ${GO:-go} test -v ${race} -coverprofile unit.coverprofile "${packages[@]}" --ginkgo.v --ginkgo.trace --ginkgo.junit-report junit.xml "$@" && \
            go tool cover -func unit.coverprofile
        else
            ${GO:-go} test -v ${race} -coverprofile unit.coverprofile "${packages[@]}" -ginkgo.v -ginkgo.trace -ginkgo.reportPassed -ginkgo.reportFile junit.xml "$@" && \
            go tool cover -func unit.coverprofile
        fi
    ) || result=1
done

exit $result
