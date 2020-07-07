#!/bin/bash

source ${SCRIPTS_DIR}/lib/version

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'tag' "${DEV_VERSION}" "Tag to set for the local image"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to use for the image"
DEFINE_string 'image' '' "Image name to build" 'i'
DEFINE_string 'dockerfile' '' "Dockerfile to build from" 'f'
DEFINE_string 'buildargs' '' "Build arguments to pass to 'docker build'"
DEFINE_boolean 'cache' true "Use cached layers from latest image"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

tag="${FLAGS_tag}"
repo="${FLAGS_repo}"
image="${FLAGS_image}"
dockerfile="${FLAGS_dockerfile}"
buildargs="${FLAGS_buildargs}"
[[ "${FLAGS_cache}" = "${FLAGS_TRUE}" ]] && cache=true || cache=false

[[ -n "${image}" ]] || { echo "The image to build must be specified!"; exit 1; }
[[ -n "${dockerfile}" ]] || { echo "The dockerfile to build from must be specified!"; exit 1; }

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
buildargs_flag=''
[[ -z "${buildargs}" ]] || buildargs_flag="--build-arg ${buildargs}"
docker build -t ${local_image} ${cache_flag} -f ${dockerfile} ${buildargs_flag} .
docker tag ${local_image} ${cache_image}

