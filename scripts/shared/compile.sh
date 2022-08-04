#!/usr/bin/env bash
  
## Process command line flags ##

source "${SCRIPTS_DIR}/lib/shflags"
DEFINE_boolean 'debug' false "Build the binary with debug information included (or stripped)"
DEFINE_boolean 'upx' true "Use UPX to make the binary smaller (only when --nodebug)"
DEFINE_string 'ldflags' '' "Extra flags to send to the Go compiler"
FLAGS_HELP="USAGE: $0 [--[no]debug] [--[no]upx] [--ldflags '<flags>'] binary source"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

if [[ $# != 2 ]]; then
    echo "The binary and source must be sepcified!"
    exit 1
fi

[[ -n "${BUILD_DEBUG}" ]] || { [[ "${FLAGS_debug}" = "${FLAGS_TRUE}" ]] && BUILD_DEBUG=true || BUILD_DEBUG=false; }
[[ -n "${BUILD_UPX}" ]] || { [[ "${FLAGS_upx}" = "${FLAGS_TRUE}" ]] && BUILD_UPX=true || BUILD_UPX=false; }
[[ -n "${LDFLAGS}" ]] || LDFLAGS=${FLAGS_ldflags}
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
