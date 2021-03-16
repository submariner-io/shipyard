#!/bin/bash

## Process command line flags ##

default_platform=linux/amd64
source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'tag' "${DEV_VERSION}" "Tag to set for the local image"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to use for the image"
DEFINE_string 'image' '' "Image name to build" 'i'
DEFINE_string 'dockerfile' '' "Dockerfile to build from" 'f'
DEFINE_string 'buildargs' '' "Build arguments to pass to 'docker build'"
DEFINE_boolean 'cache' true "Use cached layers from latest image"
DEFINE_string 'platform' "${default_platform}" 'Platforms to target'
DEFINE_string 'hash' '' "File to write the hash to" 'h'
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

tag="${FLAGS_tag}"
repo="${FLAGS_repo}"
image="${FLAGS_image}"
dockerfile="${FLAGS_dockerfile}"
buildargs="${FLAGS_buildargs}"
platform="${FLAGS_platform}"
hashfile="${FLAGS_hash}"
[[ "${FLAGS_cache}" = "${FLAGS_TRUE}" ]] && cache=true || cache=false

[[ -n "${image}" ]] || { echo "The image to build must be specified!"; exit 1; }
[[ -n "${dockerfile}" ]] || { echo "The dockerfile to build from must be specified!"; exit 1; }
[[ -n "${hashfile}" ]] || { echo "The file to write the hash to must be specified!"; exit 1; }

source ${SCRIPTS_DIR}/lib/debug_functions
set -e

local_image=${repo}/${image}:${tag}
cache_image=${repo}/${image}:${CUTTING_EDGE}

# When using cache pull latest image from the repo, so that it's layers may be reused.
cache_flag=''
if [[ "$cache" = true ]]; then
    cache_flag="--cache-from ${cache_image}"
    if [[ -z "$(docker image ls -q ${cache_image})" ]]; then
        docker pull ${cache_image} || :
    fi
    for parent in $(grep FROM ${dockerfile} | cut -f2 -d' ' | grep -v scratch); do
        cache_flag+=" --cache-from ${parent}"
        docker pull ${parent} || :
    done
fi

# Rebuild the image to update any changed layers and tag it back so it will be used.
buildargs_flag="--build-arg BUILDKIT_INLINE_CACHE=1 --build-arg BASE_BRANCH=${BASE_BRANCH}"
[[ -z "${buildargs}" ]] || buildargs_flag="${buildargs_flag} --build-arg ${buildargs}"
if docker buildx version > /dev/null 2>&1; then
    docker buildx use buildx_builder || docker buildx create --name buildx_builder --use
    docker buildx build --load -t ${local_image} ${cache_flag} -f ${dockerfile} --iidfile "${hashfile}" --platform ${platform} ${buildargs_flag} .
else
    # Fall back to plain BuildKit
    if [[ "${platform}" != "${default_platform}" ]]; then
        echo "WARNING: buildx isn't available, cross-arch builds won't work as expected"
    fi
    DOCKER_BUILDKIT=1 docker build -t ${local_image} ${cache_flag} -f ${dockerfile} --iidfile "${hashfile}" ${buildargs_flag} .
fi
docker tag ${local_image} ${cache_image}
