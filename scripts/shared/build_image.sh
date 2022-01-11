#!/bin/bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'tag' "${DEV_VERSION}" "Tag to set for the local image"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to use for the image"
DEFINE_string 'image' '' "Image name to build" 'i'
DEFINE_string 'dockerfile' '' "Dockerfile to build from" 'f'
DEFINE_string 'buildargs' '' "Build arguments to pass to 'docker build'"
DEFINE_boolean 'cache' true "Use cached layers from latest image"
DEFINE_string 'platform' '' 'Platforms to target'
DEFINE_string 'hash' '' "File to write the hash to" 'h'
DEFINE_string 'oci' '' 'File to write an OCI tarball to instead of an image in the local registry'
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

tag="${FLAGS_tag}"
repo="${FLAGS_repo}"
image="${FLAGS_image}"
dockerfile="${FLAGS_dockerfile}"
buildargs="${FLAGS_buildargs}"
platform="${FLAGS_platform}"
hashfile="${FLAGS_hash}"
ocifile="${FLAGS_oci}"
[[ "${FLAGS_cache}" = "${FLAGS_TRUE}" ]] && cache=true || cache=false

[[ -n "${image}" ]] || { echo "The image to build must be specified!"; exit 1; }
[[ -n "${dockerfile}" ]] || { echo "The dockerfile to build from must be specified!"; exit 1; }
[[ -n "${hashfile}" ]] || { echo "The file to write the hash to must be specified!"; exit 1; }
if [[ "${platform}" =~ , && -z "${ocifile}" ]]; then
    echo Multi-arch builds require OCI output, please specify --oci
    exit 1
fi

source ${SCRIPTS_DIR}/lib/debug_functions
set -e

local_image=${repo}/${image}:${tag}
cache_image=${repo}/${image}:${CUTTING_EDGE}

# When using cache pull latest image from the repo, so that its layers may be reused.
cache_flag=''
if [[ "$cache" = true ]]; then
    cache_flag="--cache-from ${cache_image}"
    if [[ -z "$(docker image ls -q ${cache_image})" ]]; then
        docker pull ${cache_image} || :
    fi
    # The shellcheck linting tool recommends piping to a while read loop, but that doesn't work for us
    # because the while loop ends up in a subshell
    # shellcheck disable=SC2013
    for parent in $(awk '/FROM/ {
                             for (i = 2; i <= NF; i++) {
                                 if ($i == "AS") next;
                                 if (!($i ~ /^--platform/ || $i ~ /scratch/))
                                     print gensub("\\${BASE_BRANCH}", ENVIRON["BASE_BRANCH"], "g", $i)
                             }
                         }' "${dockerfile}"); do
        cache_flag+=" --cache-from ${parent}"
        docker pull ${parent} || :
    done
fi

output_flag=--load
[[ -z "${ocifile}" ]] || output_flag="--output=type=oci,dest=${ocifile}"

# Default to linux/amd64 (for CI); platforms match Go OS/arch
if command -v "${GO:-go}" >/dev/null; then
    default_platform="$(${GO:-go} env GOOS)/$(${GO:-go} env GOARCH)"
else
    echo Unable to determine default container image platform, assuming linux/amd64
    default_platform=linux/amd64
fi
[[ -n "$platform" ]] || platform="$default_platform"

# Rebuild the image to update any changed layers and tag it back so it will be used.
buildargs_flag="--build-arg BUILDKIT_INLINE_CACHE=1 --build-arg BASE_BRANCH=${BASE_BRANCH}"
[[ -z "${buildargs}" ]] || buildargs_flag="${buildargs_flag} --build-arg ${buildargs}"
if docker buildx version > /dev/null 2>&1; then
    docker buildx use buildx_builder || docker buildx create --name buildx_builder --use
    docker buildx build ${output_flag} -t ${local_image} ${cache_flag} -f ${dockerfile} --iidfile "${hashfile}" --platform ${platform} ${buildargs_flag} .
else
    # Fall back to plain BuildKit
    if [[ "${platform}" != "${default_platform}" ]]; then
        echo "WARNING: buildx isn't available, cross-arch builds won't work as expected"
    fi
    DOCKER_BUILDKIT=1 docker build -t ${local_image} ${cache_flag} -f ${dockerfile} --iidfile "${hashfile}" ${buildargs_flag} .
fi

# We can only tag the image in non-OCI mode
[[ -n "${ocifile}" ]] || docker tag ${local_image} ${cache_image}
