#!/usr/bin/env bash

function print_indent() {
    printf "%-24s" "$1"
    printf "$2"
}

title=$(print_indent "Target" "Description")
echo "$title"
echo "$title" | tr '[:alnum:]' '-'

make_targets=($(make -pRrq : 2>/dev/null |\
    grep -oP '^(?!Makefile.*)[-[:alnum:]]*(?=:)' | sort -u))

for target in ${make_targets[*]}; do
    print_indent "${target}"
    (grep -hoP "(?<=\[${target}\] ).*" Makefile* ${SHIPYARD_DIR}/Makefile* || printf '\n') | head -1
done

