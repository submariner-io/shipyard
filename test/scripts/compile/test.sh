#!/usr/bin/env bash

set -e

function fail() {
    echo "$@"
    exit 1
}

function validate_ldflags() {
    expected=$1
    actual=$($binary)
    if [[ "$expected" != "$actual" ]]; then
        fail "Expected ${expected@Q}, but got ${actual@Q}"
    fi
}

function test_compile_arch() {
    local binary=$1
    local arch=$2
    ${SCRIPTS_DIR}/compile.sh $binary hello.go
    if ! file $binary | grep -q $arch; then
        fail "Shoul'dve compiled ${arch@Q} but got $(file $binary)."
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
if ! file $binary | grep "not stripped" > /dev/null; then
    fail "Debug information got stripped, even when requested!"
fi

BUILD_UPX=true ${SCRIPTS_DIR}/compile.sh $binary hello.go
if upx $binary > /dev/null 2>&1; then
    fail "Binary wasn't UPX'd although requested"
fi

test_compile_arch bin/linux/arm/v7/hello ARM

GOARCH=arm test_compile_arch bin/hello ARM
