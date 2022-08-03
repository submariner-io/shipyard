#!/usr/bin/env bash

set -e
source "${SCRIPTS_DIR}/lib/utils"

[[ $# -gt 0 ]] || exit_error "At least one image to release must be specified!"

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

