#!/bin/bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'tag' "${DEV_VERSION}" "Tag to set for the local image"
DEFINE_string 'repo' 'quay.io/submariner' "Quay.io repo to use for the image"
DEFINE_string 'image' '' "Image name to build" 'i'
DEFINE_string 'dockerfile' '' "Dockerfile to build from" 'f'
DEFINE_string 'buildargs' '' "Build arguments to pass to 'docker build'"
DEFINE_string 'platform' '' 'Platforms to target'
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

[[ -n "${image}" ]] || { echo "The image to build must be specified!"; exit 1; }
[[ -n "${dockerfile}" ]] || { echo "The dockerfile to build from must be specified!"; exit 1; }
[[ -n "${hashfile}" ]] || { echo "The file to write the hash to must be specified!"; exit 1; }

source ${SCRIPTS_DIR}/lib/debug_functions
set -e

local_image=${repo}/${image}:${tag}
cache_image=${repo}/${image}:${CUTTING_EDGE}

# We always use manifest builds
buildah manifest rm "${image}" || :
buildah manifest create "${image}" || :

buildargs_flag="--build-arg BASE_BRANCH=${BASE_BRANCH}"
[[ -z "${buildargs}" ]] || buildargs_flag="${buildargs_flag} --build-arg ${buildargs}"

# Default to linux/amd64 (for CI); Buildah platforms match Go OS/arch
if command -v "${GO:-go}" >/dev/null; then
    default_platform="$(${GO:-go} env GOOS)/$(${GO:-go} env GOARCH)"
else
    echo Unable to determine default container image platform, assuming linux/amd64
    default_platform=linux/amd64
fi
[[ -n "$platform" ]] || platform="$default_platform"

OIFS="$IFS"; IFS=,; platforms=($platform); IFS="$OIFS"
for p in "${platforms[@]}"; do
    # We need to manually specify TARGETPLATFORM
    # See https://github.com/containers/buildah/issues/1368 (closed but not actually fixed)
    buildah bud --tag "${local_image}" \
                --manifest "${image}" \
                --file "${dockerfile}" \
                --iidfile "${hashfile}.${p/\//-}" \
                --platform "${p}" \
                ${buildargs_flag} \
                --build-arg TARGETPLATFORM=${p} \
                .
    
    # Make images matching the local platform available to Docker, and link their hashfile
    if [ "$p" = "$default_platform" ]; then
        ln -sf "${hashfile}.${p/\//-}" "${hashfile}"
        buildah push ${local_image} docker-daemon:${local_image} || :
        buildah push ${local_image} docker-daemon:${cache_image} || :
    fi
done

buildah tag ${local_image} ${cache_image}
