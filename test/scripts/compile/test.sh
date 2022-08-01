#!/usr/bin/env bash

set -e

function validate_ldflags() {
    expected=$1
    actual=$($binary)
    if [[ "$expected" != "$actual" ]]; then
        echo "Expected ${expected@Q}, but got ${actual@Q}"
        exit 1
    fi
}

cd $(dirname $0)
binary=bin/test/hello
BUILD_UPX=false ${SCRIPTS_DIR}/compile.sh $binary hello.go
validate_ldflags "hello nobody"

BUILD_UPX=false LDFLAGS="-X main.MYVAR=somebody" ${SCRIPTS_DIR}/compile.sh $binary hello.go
validate_ldflags "hello somebody"

BUILD_DEBUG=true ${SCRIPTS_DIR}/compile.sh $binary hello.go
if ! file $binary | grep "not stripped" > /dev/null; then
    echo "Debug information got stripped, even when requested!"
    exit 1
fi

BUILD_UPX=true ${SCRIPTS_DIR}/compile.sh $binary hello.go
if upx $binary > /dev/null 2>&1; then
    echo "Binary wasn't UPX'd although requested"
    exit 1
fi
