#!/usr/bin/env bash

set -e

# In case we're pretending to be `subctl`
if [[ "${0##*/}" = subctl ]] && [[ -L "$0" ]]; then
    run_subctl=true

    # Delete ourselves to ensure we don't run into issues with the new subctl
    rm -f "$0"
fi

# Default to devel if we don't know what base branch were on
curl -Ls --retry 3 https://get.submariner.io | VERSION="${SUBCTL_VERSION:-${BASE_BRANCH:-devel}}" bash

# If we're pretending to be subctl, run subctl with any given arguments
[[ -z "${run_subctl}" ]] || subctl "$@"
