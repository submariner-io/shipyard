#!/usr/bin/env bash

set -e

source "${SCRIPTS_DIR}/lib/debug_functions"

function _find() {
    declare -a excludes
    for entry in .git $(git ls-files -o -i --exclude-from=.gitignore --directory); do
        test -f "$entry" || excludes+=(-path "./${entry/%\/}" -prune -o)
    done

    find . "${excludes[@]}" "$@" -printf "%h\0" | sort -z -u
}

result=0
echo "Looking for packages to test"
readarray -d '' modules < <(_find -name go.mod)

for module in "${modules[@]}"; do
    exclude_args=()
    echo "Looking for tests in module ${module}"

    # Exclude any sub-modules
    for exc_module in "${modules[@]}"; do
        if [ "$exc_module" != "$module" ] && [ "$exc_module" != "." ]; then
            exclude_args+=(-path "$exc_module" -prune -o)
        fi
    done

    # Run in subshell to return to base directory even if the tests fail
    (
        cd "$module"

        # Exclude any directories containing e2e tests
        for dir in $(git grep -w -l e2e | grep _test.go | sed 's#\(.*/.*\)/.*$#\1#' | sort -u); do
            exclude_args+=(-path "./${dir}" -prune -o)
        done

        readarray -d '' packages < <(_find "${exclude_args[@]}" -path "./*/*_test.go")
        [ "${#packages[@]}" -gt 0 ] || exit 0

        echo "Running tests in ${packages[*]}"
        [ "${ARCH}" == "amd64" ] && race=-race
        # It's important that the `go test` command's exit status is reported from this () block.
        # Can't be one command (with -cover). Need detailed -coverprofile for Sonar and summary to console.
        # shellcheck disable=SC2086 # Split `$TEST_ARGS` on purpose
        "${GO:-go}" test -v ${race} -coverprofile unit.coverprofile "${packages[@]}" \
            --ginkgo.v --ginkgo.trace --ginkgo.junit-report junit.xml $TEST_ARGS && \
        go tool cover -func unit.coverprofile
    ) || result=1
done

exit $result
