#!/bin/bash

set -e

### Variables ###

[[ -n "${USE_CACHE}" ]] || USE_CACHE='true'
[[ $# == 1 ]] || { echo "Exactly one image to build must be specified!"; exit 1; }
[[ -n "${DOCKERFILE}" ]] || { echo "The DOCKERFILE to build from must be specified!"; exit 1; }
[[ -n "${HASHFILE}" ]] || { echo "The HASHFILE to write the hash to must be specified!"; exit 1; }
if [[ "${PLATFORM}" =~ , && -z "${OCIFILE}" ]]; then
    echo Multi-arch builds require OCI output, please set OCIFILE
    exit 1
fi

### Main ###

source "${SCRIPTS_DIR}/lib/debug_functions"
local_image="${REPO}/${1}:${DEV_VERSION}"
cache_image="${REPO}/${1}:${CUTTING_EDGE}"

# When using cache pull latest image from the repo, so that its layers may be reused.
declare -a cache_flags
if [[ "${USE_CACHE}" = true ]]; then
    cache_flags+=(--cache-from "${cache_image}")
    if [[ -z "$(docker image ls -q "${cache_image}")" ]]; then
        docker pull "${cache_image}" || :
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
                         }' "${DOCKERFILE}"); do
        cache_flags+=(--cache-from "${parent}")
        docker pull "${parent}" || :
    done
fi

output_flag=--load
[[ -z "${OCIFILE}" ]] || output_flag="--output=type=oci,dest=${OCIFILE}"

# Default to linux/amd64 (for CI); platforms match Go OS/arch
if command -v "${GO:-go}" >/dev/null; then
    default_platform="$(${GO:-go} env GOOS)/$(${GO:-go} env GOARCH)"
else
    echo Unable to determine default container image platform, assuming linux/amd64
    default_platform=linux/amd64
fi
[[ -n "$PLATFORM" ]] || PLATFORM="$default_platform"

# Rebuild the image to update any changed layers and tag it back so it will be used.
buildargs_flags=(--build-arg BUILDKIT_INLINE_CACHE=1 --build-arg "BASE_BRANCH=${BASE_BRANCH}")
if [[ "${PLATFORM}" != "${default_platform}" ]] && docker buildx version > /dev/null 2>&1; then
    docker buildx use buildx_builder || docker buildx create --name buildx_builder --use
    docker buildx build "${output_flag}" -t "${local_image}" "${cache_flags[@]}" -f "${DOCKERFILE}" --iidfile "${HASHFILE}" --platform "${PLATFORM}" "${buildargs_flags[@]}" .
else
    # Fall back to plain BuildKit
    if [[ "${PLATFORM}" != "${default_platform}" ]]; then
        echo "WARNING: buildx isn't available, cross-arch builds won't work as expected"
    fi
    DOCKER_BUILDKIT=1 docker build -t "${local_image}" "${cache_flags[@]}" -f "${DOCKERFILE}" --iidfile "${HASHFILE}" "${buildargs_flags[@]}" .
fi

# We can only tag the image in non-OCI mode
[[ -n "${OCIFILE}" ]] || docker tag "${local_image}" "${cache_image}"
