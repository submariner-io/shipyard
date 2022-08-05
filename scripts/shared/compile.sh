#!/usr/bin/env bash
  
set -e

[[ -n "${BUILD_UPX}" ]] || BUILD_UPX=true

## Process command line arguments ##

if [[ $# != 2 ]]; then
    echo "The binary and source must be sepcified!"
    exit 1
fi

binary=$1
source_file=$2

set -e

source "${SCRIPTS_DIR}/lib/utils"
print_env BUILD_DEBUG BUILD_UPX LDFLAGS
source "${SCRIPTS_DIR}/lib/debug_functions"

### Functions ###

# Determine GOARCH based on the last component of the target directory, if any
function determine_goarch() {
    GOARCH="$(dirname "${binary}")"
    [[ "${GOARCH}" != '.' ]] || { unset GOARCH && return 0; }

    # Convert from Docker arch to Go arch
    GOARCH="${GOARCH/arm\/v7/arm}"
    export GOARCH="${GOARCH##*/}"
}

## Main ##

[[ -n "${GOARCH}" ]] || determine_goarch
mkdir -p "${binary%/*}"

echo "Building ${binary@Q} (LDFLAGS: ${LDFLAGS@Q})"
[[ "$BUILD_DEBUG" == "true" ]] || LDFLAGS="-s -w ${LDFLAGS}"

CGO_ENABLED=0 ${GO:-go} build -trimpath -ldflags "${LDFLAGS}" -o "$binary" "$source_file"
[[ "$BUILD_UPX" != "true" ]] || [[ "$BUILD_DEBUG" == "true" ]] || upx "$binary"
