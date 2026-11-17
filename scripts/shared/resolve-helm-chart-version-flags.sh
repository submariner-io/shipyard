#!/usr/bin/env bash

# Print helm install version-selection flags (one per line).
# On devel: --devel (newest chart, including pre-releases).
# On release-X.Y: --version ~X.Y.0 without --devel (highest stable in that stream).
# HELM_CHART_VERSION / explicit override: --version <value>.
set -e -o pipefail

branch=${1:-${BASE_BRANCH:-${CUTTING_EDGE:-devel}}}
override_version=${2:-${HELM_CHART_VERSION:-}}

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

if [[ -n "$override_version" ]]; then
    echo "--version"
    echo "$override_version"
    exit
fi

if [[ "$branch" =~ ^release-(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    echo "--version"
    echo "~${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.0"
    exit
fi

if [[ "$branch" != devel ]]; then
    fail "Unsupported base branch '$branch'; set HELM_CHART_VERSION explicitly"
fi

echo "--devel"
