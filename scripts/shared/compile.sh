#!/usr/bin/env bash
  
## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
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

[[ "${FLAGS_debug}" = "${FLAGS_TRUE}" ]] && build_debug=true || build_debug=false
[[ "${FLAGS_upx}" = "${FLAGS_TRUE}" ]] && build_upx=true || build_upx=false
ldflags=${FLAGS_ldflags}
binary=$1
source_file=$2

set -e

source ${SCRIPTS_DIR}/lib/debug_functions

## Main ##

mkdir -p ${binary%/*}

echo "Building ${binary@Q} (ldflags: ${ldflags@Q})"
if [ "$build_debug" = "false" ]; then
    ldflags="-s -w ${ldflags}"
fi

CGO_ENABLED=0 ${GO:-go} build -buildvcs=false -trimpath -ldflags "${ldflags}" -o $binary $source_file
[[ "$build_upx" = "false" ]] || [[ "$build_debug" = "true" ]] || upx $binary

