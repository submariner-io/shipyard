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
        readarray -d '' e2e_test_files < <(git grep -z -w -l RunE2ETests -- '*_test.go' || true)
        for file in "${e2e_test_files[@]}"; do
            dir="$(dirname -- "$file")"
            exclude_args+=(-path "./${dir}" -prune -o)
        done

        readarray -d '' packages < <(_find "${exclude_args[@]}" -path "./*/*_test.go")
        [ "${#packages[@]}" -gt 0 ] || exit 0

        echo "Running tests in ${packages[*]}"
        [ "${ARCH}" == "amd64" ] && race=--race
        # It's important that the test command's exit status is reported from this () block.
        # Can't be one command (with -cover). Need detailed -coverprofile for Sonar and summary to console.
        # shellcheck disable=SC2086 # Split `$TEST_ARGS` on purpose
        "${GO:-go}" run github.com/onsi/ginkgo/v2/ginkgo -v -p --trace ${race} \
            --coverprofile=unit.coverprofile --junit-report=junit.xml --fail-fast "${packages[@]}" $TEST_ARGS && \
        "${GO:-go}" tool cover -func unit.coverprofile
    ) || result=1
done

exit $result
