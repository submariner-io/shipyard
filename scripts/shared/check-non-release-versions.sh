#!/bin/bash

tmpdir=$(mktemp -d)
trap 'rm -rf $tmpdir' EXIT

# List all submariner-io dependencies with a - in their version
# We're looking for versions pointing to commits, of the form
# vX.Y.Z-0.YYYYMMDDhhmmss-hash
failed=0
shopt -s lastpipe
GOWORK=off go list -m -mod=mod -json all |
    jq -r 'select(.Path | contains("/submariner-io/")) | select(.Main != true) | select(.Version | contains ("-")) | select(.Version | length > 14) | "\(.Path) \(.Version)"' |
    while read -r project version; do
        cd "$tmpdir" || exit 1
        git clone "https://$project"
        cd "${project##*/}" || exit 1
        hash="${version##*-}"
        branch="${GITHUB_BASE_REF:-devel}"
        if ! git merge-base --is-ancestor "$hash" "origin/$branch"; then
            printf "This project depends on %s %s\n" "$project" "$version"
            printf "but %s branch %s does not contain commit %s\n" "$project" "$branch" "$hash"
            failed=1
        fi
    done

exit $failed
