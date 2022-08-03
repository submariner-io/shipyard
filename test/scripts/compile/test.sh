#!/usr/bin/env bash

set -e
source "${SCRIPTS_DIR}"/lib/utils

function validate_ldflags() {
    expected=$1
    actual=$($binary)
    [[ "$expected" == "$actual" ]] || exit_error "Expected ${expected@Q}, but got ${actual@Q}"
}

function test_compile_arch() {
    local binary=$1
    local arch=$2
    ${SCRIPTS_DIR}/compile.sh $binary hello.go
    if ! file $binary | grep -q $arch; then
        exit_error "Should have compiled ${arch@Q} but got $(file $binary)."
    fi
}

cd $(dirname $0)
binary=bin/linux/amd64/hello
export BUILD_UPX=false
${SCRIPTS_DIR}/compile.sh $binary hello.go
validate_ldflags "hello nobody"

LDFLAGS="-X main.MYVAR=somebody" ${SCRIPTS_DIR}/compile.sh $binary hello.go
validate_ldflags "hello somebody"

BUILD_DEBUG=true ${SCRIPTS_DIR}/compile.sh $binary hello.go
file $binary | grep "not stripped" > /dev/null || exit_error "Debug information got stripped, even when requested!"

BUILD_UPX=true ${SCRIPTS_DIR}/compile.sh $binary hello.go
upx $binary > /dev/null 2>&1 && exit_error "Binary wasn't UPX'd although requested"

test_compile_arch bin/linux/arm/v7/hello ARM

GOARCH=arm test_compile_arch bin/hello ARM
