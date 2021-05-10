#!/usr/bin/env bash

function print_indent() {
    printf "%-24s%s\n" "$1" "$2"
}

print_indent Target Description | tee >(tr '[:alnum:]' '-')

make_targets=($(make -pRrq : 2>/dev/null |\
    grep -oP '^(?!Makefile.*)[-[:alnum:]]*(?=:)' | sort -u))

for target in ${make_targets[*]}; do
    print_indent "${target}" \
    "$(grep -hoP -m1 "(?<=\[${target}\] ).*" Makefile* ${SHIPYARD_DIR}/Makefile*)"
done
