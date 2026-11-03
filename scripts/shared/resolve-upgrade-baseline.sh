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

headers=(-H 'Accept: application/vnd.github+json')
token=${GH_TOKEN:-${GITHUB_TOKEN:-}}
[[ -z "$token" ]] || headers+=(-H "Authorization: Bearer $token")

tags=
page=1
while :; do
    releases=$(curl --fail --silent --show-error --location --retry 3 "${headers[@]}" \
        "https://api.github.com/repos/submariner-io/releases/releases?per_page=100&page=$page")
    jq -e 'type == "array" and all(.[];
        (.tag_name | type == "string") and
        (.draft | type == "boolean") and (.prerelease | type == "boolean"))' \
        <<< "$releases" > /dev/null || fail "Invalid releases response on page $page"
    tags+="$(jq -r '.[] | select(.draft == false and .prerelease == false) | .tag_name' <<< "$releases")"$'\n'
    [[ $(jq 'length' <<< "$releases") -eq 100 ]] || break
    page=$((page + 1))
done

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
