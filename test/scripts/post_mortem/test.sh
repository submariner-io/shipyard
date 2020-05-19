#!/usr/bin/env bash

set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

function image_name() {
    echo "norepo/${cluster}_image"
}

function deploy_deadshoot() {
    filename="/tmp/netshoot_${cluster}"
    cp ${RESOURCES_DIR}/netshoot.yaml "${filename}"

    sed -i "s#image: .*#image: $(image_name)#" "${filename}"

    kubectl apply -f "${filename}"
}

declare_kubeconfig
clusters=($(kind get clusters))
run_parallel "${clusters[*]}" deploy_deadshoot

post_mortem=$(make post-mortem)
echo "$post_mortem"

for cluster in "${clusters[@]}"; do
    img=$(image_name)
    if [[ ! $post_mortem =~ $(image_name) ]]; then
        echo "Post mortem failed, didn't find failure for ${cluster}"
        exit 1
    fi
done
