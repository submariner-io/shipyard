#!/usr/bin/env bash

# Offline regression tests for helm chart version flag selection.
set -e -o pipefail

repo=$(cd "$(dirname "$0")/../../.." && pwd)
export SCRIPTS_DIR="$repo/scripts/shared"
script="$SCRIPTS_DIR/resolve-helm-chart-version-flags.sh"
log=$(mktemp)
trap 'rm -f "$log"' EXIT

expect_flags() {
    local branch=$1
    shift
    local expected=("$@")
    local actual output
    if ! output=$("$script" "$branch"); then
        echo "Expected success for $branch, resolver failed" >&2
        exit 1
    fi
    mapfile -t actual <<< "${output}"
    if [[ "${actual[*]}" != "${expected[*]}" ]]; then
        echo "Expected (${expected[*]}) for $branch, got (${actual[*]})" >&2
        exit 1
    fi
}

expect_override() {
    local branch=$1 version=$2
    shift 2
    local expected=("$@")
    local actual output
    if ! output=$("$script" "$branch" "$version"); then
        echo "Expected success for $branch override $version, resolver failed" >&2
        exit 1
    fi
    mapfile -t actual <<< "${output}"
    if [[ "${actual[*]}" != "${expected[*]}" ]]; then
        echo "Expected (${expected[*]}) for $branch override $version, got (${actual[*]})" >&2
        exit 1
    fi
}

expect_failure() {
    if "$@" >"$log" 2>&1; then
        echo "Expected failure: $*" >&2
        exit 1
    fi
}

# Clear inherited env so default-selection tests are deterministic.
unset HELM_CHART_VERSION BASE_BRANCH CUTTING_EDGE

expect_flags devel --devel
expect_flags release-0.23 --version '~0.23.0'
expect_flags release-0.22 --version '~0.22.0'
expect_override devel 0.23.4 --version 0.23.4
expect_override release-0.23 0.23.2 --version 0.23.2
expect_override custom-branch 0.24.1 --version 0.24.1

# Env override (no argv version) must win over branch defaults.
export HELM_CHART_VERSION=0.23.1
expect_flags devel --version 0.23.1
expect_flags release-0.23 --version 0.23.1
unset HELM_CHART_VERSION

expect_failure "$script" feature-unrecognized
expect_failure "$script" release-0.23.5

# Call the production helper so a regression to process substitution / nameref
# shadowing is caught.
# shellcheck source=scripts/shared/lib/deploy_helm
source "${SCRIPTS_DIR}/lib/deploy_helm"

version_flags=()
export BASE_BRANCH=feature-unrecognized
if helm_chart_version_flags version_flags >"$log" 2>&1; then
    echo "Expected helm_chart_version_flags to fail for BASE_BRANCH=feature-unrecognized" >&2
    exit 1
fi
if ((${#version_flags[@]})); then
    echo "Expected no flags on resolver failure, got (${version_flags[*]})" >&2
    exit 1
fi
if [[ -s "$log" ]] && grep -qE -- '--version|--devel' "$log"; then
    echo "Resolver failure must not produce helm version flags" >&2
    cat "$log" >&2
    exit 1
fi

# Success path: nameref must populate the caller's array (including when the
# caller uses a name that could collide with a careless local).
version_flags=()
export BASE_BRANCH=release-0.23
unset HELM_CHART_VERSION
helm_chart_version_flags version_flags >"$log" 2>&1
if [[ "${version_flags[*]}" != "--version ~0.23.0" ]]; then
    echo "Expected (--version ~0.23.0) via helper, got (${version_flags[*]})" >&2
    cat "$log" >&2
    exit 1
fi

flags=()
export BASE_BRANCH=devel
helm_chart_version_flags flags >"$log" 2>&1
if [[ "${flags[*]}" != "--devel" ]]; then
    echo "Expected (--devel) via helper nameref name 'flags', got (${flags[*]})" >&2
    cat "$log" >&2
    exit 1
fi

echo "helm chart version flag selection tests passed"
