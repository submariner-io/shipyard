#!/usr/bin/env bash

## Process command line flags ##

source /usr/share/shflags/shflags
DEFINE_string 'tag' '' "Additional tag to use for the image (prefix 'v' will be stripped)"
DEFINE_string 'repo' '' "Quay.io repo to deploy to"
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
source ${SCRIPTS_DIR}/lib/version

function release_image() {
    local image=$1
    local images=("${image}:${release_tag#v}" "${image}:${TRAVIS_COMMIT:0:7}")

    for target_image in "${images[@]}"; do
        docker tag ${image}:${VERSION} ${target_image}
        docker push ${target_image}
    done
}

echo "$QUAY_PASSWORD" | docker login quay.io -u "$QUAY_USERNAME" --password-stdin
for image in "$@"; do
    release_image ${repo}/${image}
done

