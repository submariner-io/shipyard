#!/usr/bin/env bash

set -e -o pipefail

branch=${1:-${BASE_BRANCH:-devel}}
override_version=${2:-${UPGRADE_BASELINE_VERSION:-}}

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

if [[ -n "$override_version" ]]; then
    [[ "$override_version" =~ ^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] ||
        fail "Upgrade baseline must be an exact stable version, got '$override_version'"
    tag="v${override_version#v}"
    echo "Upgrade baseline for $branch: $tag (explicit override)" >&2
    echo "$tag"
    exit
fi

if [[ "$branch" =~ ^release-(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    major=${BASH_REMATCH[1]}
    minor=${BASH_REMATCH[2]}
elif [[ "$branch" != devel ]]; then
    fail "Unsupported base branch '$branch'; set UPGRADE_BASELINE_VERSION explicitly"
fi

tags=$(git ls-remote --tags https://github.com/submariner-io/releases.git | awk -F/ '{print $NF}')

# Compare version components numerically, independently of publication order.
versions=$(jq -Rn '[inputs | select(test("^v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$")) |
    {tag: ., version: (ltrimstr("v") | split(".") | map(tonumber))}]' <<< "$tags")

if [[ "$branch" == devel ]]; then
    tag=$(jq -r 'max_by(.version).tag // empty' <<< "$versions")
else
    tag=$(jq -r --argjson major "$major" --argjson minor "$minor" \
        'map(select(.version[0] == $major and .version[1] == $minor)) | max_by(.version).tag // empty' <<< "$versions")
    if [[ -z "$tag" && "$minor" -gt 0 ]]; then
        # Before the first stable release, exercise an upgrade from the previous minor.
        tag=$(jq -r --argjson major "$major" --argjson minor "$((minor - 1))" \
            'map(select(.version[0] == $major and .version[1] == $minor)) | max_by(.version).tag // empty' <<< "$versions")
        [[ -z "$tag" ]] || echo "No stable release for $branch; using the preceding minor" >&2
    fi
fi

[[ -n "$tag" ]] || fail "No stable upgrade baseline for '$branch'; set UPGRADE_BASELINE_VERSION explicitly"
echo "Upgrade baseline for $branch: $tag" >&2
echo "$tag"
