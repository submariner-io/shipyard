#!/bin/bash

source ${SCRIPTS_DIR}/lib/version

ARCH=${ARCH:-"amd64"}
SUFFIX=""
[ "${ARCH}" != "amd64" ] && SUFFIX="_${ARCH}"

## Process command line flags ##

source /usr/share/shflags/shflags
DEFINE_string 'tag' "${VERSION}${SUFFIX}" "Tag to set for the local image"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to use for the image"
DEFINE_string 'image' '' "Image name to build" 'i'
DEFINE_string 'dockerfile' '' "Dockerfile to build from" 'f'
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

tag="${FLAGS_tag}"
repo="${FLAGS_repo}"
image="${FLAGS_image}"
dockerfile="${FLAGS_dockerfile}"

[[ -n "${image}" ]] || { echo "The image to build must be specified!"; exit 1; }
[[ -n "${dockerfile}" ]] || { echo "The dockerfile to build from must be specified!"; exit 1; }

source ${SCRIPTS_DIR}/lib/debug_functions
set -e

local_image=${repo}/${image}:${tag}
latest_image=${repo}/${image}:latest

# Always pull latest image from the repo, so that it's layers may be reused.
docker pull ${latest_image}
docker tag ${latest_image} ${local_image}

# Rebuild the image to update any changed layers and tag it back so it will be used.
docker build -t ${local_image} --cache-from ${latest_image} -f ${dockerfile} .
docker tag ${local_image} ${latest_image}

