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
${SCRIPTS_DIR}/compile.sh $binary hello.go --noupx
validate_ldflags "hello nobody"

${SCRIPTS_DIR}/compile.sh --ldflags "-X main.MYVAR=somebody" $binary hello.go --noupx
validate_ldflags "hello somebody"

${SCRIPTS_DIR}/compile.sh $binary hello.go --debug
if ! file $binary | grep "not stripped" > /dev/null; then
    echo "Debug information got stripped, even when requested!"
    exit 1
fi

${SCRIPTS_DIR}/compile.sh $binary hello.go --upx
if upx $binary > /dev/null 2>&1; then
    echo "Binary wasn't UPX'd although requested"
    exit 1
fi
