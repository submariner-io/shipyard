#!/bin/bash

set -e
source "${SCRIPTS_DIR}/lib/utils"

[[ $# == 3 ]] || exit_error 'You must specify exactly 3 arguments: The image name, the Dockerfile and a hash file to write to'
[[ "${PLATFORM}" =~ , && -z "${OCIFILE}" ]] && exit_error 'Multi-arch builds require OCI output, please set OCIFILE'

print_env OCIFILE PLATFORM REPO
source "${SCRIPTS_DIR}/lib/debug_functions"

### Arguments ###

image="$1"
dockerfile="$2"
hashfile="$3"

### Main ###

local_image="${REPO}/${image}:${DEV_VERSION}"
cache_image="${REPO}/${image}:${CUTTING_EDGE}"

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
                         }' "${dockerfile}"); do
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
buildargs_flags=(--build-arg BUILDKIT_INLINE_CACHE=1 --build-arg "BASE_BRANCH=${BASE_BRANCH}" --build-arg "VERSION=${VERSION}")
if [[ "${PLATFORM}" != "${default_platform}" ]] && docker buildx version > /dev/null 2>&1; then
    docker buildx use buildx_builder || docker buildx create --name buildx_builder --use
    docker buildx build "${output_flag}" -t "${local_image}" "${cache_flags[@]}" -f "${dockerfile}" --iidfile "${hashfile}" --platform "${PLATFORM}" "${buildargs_flags[@]}" .
else
    # Fall back to plain BuildKit
    if [[ "${PLATFORM}" != "${default_platform}" ]]; then
        echo "WARNING: buildx isn't available, cross-arch builds won't work as expected"
    fi
    DOCKER_BUILDKIT=1 docker build -t "${local_image}" "${cache_flags[@]}" -f "${dockerfile}" --iidfile "${hashfile}" "${buildargs_flags[@]}" .
fi

# We can only tag the image in non-OCI mode
[[ -n "${OCIFILE}" ]] || docker tag "${local_image}" "${cache_image}"
