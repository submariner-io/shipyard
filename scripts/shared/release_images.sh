#!/usr/bin/env bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'tag' "${CUTTING_EDGE}" "Additional tag(s) to use for the image (prefix 'v' will be stripped)"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to deploy to"
DEFINE_string 'oci' '' 'Directory containing OCI images (for multi-arch pushes)'
FLAGS_HELP="USAGE: $0 [--tag v1.2.3] [--repo quay.io/myrepo] [--oci package] image [image ...]"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

release_tag="${FLAGS_tag}"
repo="${FLAGS_repo}"
oci="${FLAGS_oci}"

if [[ $# == 0 ]]; then
    echo "At least one image to release must be specified!"
    exit 1
fi

set -e

source ${SCRIPTS_DIR}/lib/debug_functions

function release_image() {
    local image=$1

    for target_tag in $VERSION $release_tag; do
        local target_image="${image}:${target_tag#v}"
        if [[ -z "${oci}" ]]; then
            # Single-arch
            skopeo copy docker-daemon:${repo}/${image}:${DEV_VERSION} docker://${repo}/${target_image}
        else
            skopeo copy --all oci-archive:${oci}/${image}.tar docker://${repo}/${target_image}
        fi
    done
}

echo "$QUAY_PASSWORD" | skopeo login quay.io -u "$QUAY_USERNAME" --password-stdin

for image; do
    release_image ${image}
done

