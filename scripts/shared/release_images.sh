#!/usr/bin/env bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'tag' "${CUTTING_EDGE}" "Additional tag(s) to use for the image (prefix 'v' will be stripped)"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to deploy to"
FLAGS_HELP="USAGE: $0 [--tag v1.2.3] [--repo quay.io/myrepo] image [image ...]"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

release_tag="${FLAGS_tag}"
repo="${FLAGS_repo}"

if [[ $# == 0 ]]; then
    echo "At least one image to release must be specified!"
    exit 1
fi

set -e

source ${SCRIPTS_DIR}/lib/debug_functions

function release_image() {
    local image=$1
    local manifest=${image##*/}

    for target_tag in $VERSION $release_tag; do
        local target_image="${image}:${target_tag#v}"
	    buildah manifest push --all ${manifest} docker://${target_image}
    done
}

echo "$QUAY_PASSWORD" | buildah login -u "$QUAY_USERNAME" --password-stdin quay.io
for image in "$@"; do
    release_image ${repo}/${image}
done

