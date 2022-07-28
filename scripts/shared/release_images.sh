#!/usr/bin/env bash

## Process command line flags ##

source "${SCRIPTS_DIR}/lib/shflags"
DEFINE_string 'tag' "${CUTTING_EDGE}" "Additional tag(s) to use for the image (prefix 'v' will be stripped)"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

[[ -n "${TAG}" ]] || TAG="${FLAGS_tag}"

if [[ $# == 0 ]]; then
    echo "At least one image to release must be specified!"
    exit 1
fi

set -e

source "${SCRIPTS_DIR}/lib/utils"
print_env REPO TAG
source "${SCRIPTS_DIR}/lib/debug_functions"

function release_image() {
    for target_tag in $VERSION $TAG; do
        local target_image="${image}:${target_tag#v}"
        if [[ -z "${OCIDIR}" ]]; then
            # Single-arch
            skopeo copy "docker-daemon:${REPO}/${image}:${DEV_VERSION}" "docker://${REPO}/${target_image}"
        else
            skopeo copy --all "oci-archive:${OCIDIR}/${image}.tar" "docker://${REPO}/${target_image}"
        fi
    done
}

echo "$QUAY_PASSWORD" | skopeo login quay.io -u "$QUAY_USERNAME" --password-stdin

for image; do
    release_image
done

