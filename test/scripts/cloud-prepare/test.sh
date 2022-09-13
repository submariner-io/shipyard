#!/usr/bin/env bash

set -e -o pipefail

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

function verify_gw_topology() {
    local expected_gateways="${1}"
    local gateways_count

    echo "Verifying ${expected_gateways} gateway nodes"
    gateways_count=$(kubectl get nodes -l=submariner.io/gateway=true \
        -o yaml | yq '.items | length' -)

    if [[ "$expected_gateways" -ne "$gateways_count" ]]; then
        echo "Expected ${expected_gateways} gateways nodes but detected ${gateways_count}"
        return 1
    fi

    echo "Found expected number of gateways - ${expected_gateways}"
}

function remove_gw_labels() {
    local gw_nodes

    echo "Reset gateway nodes (unlabel)"
    readarray -t gw_nodes < <(kubectl get nodes -l=submariner.io/gateway=true \
        -o yaml | yq '.items[].metadata.name' -)

    for node in "${gw_nodes[@]}"; do
        kubectl label --overwrite nodes "$node" submariner.io/gateway-
    done
}

function test_gateways() {
    local scenario="$1"
    local expected_gateways="$2"

    echo "Set gateway nodes according to ${scenario} scenario"
    export SETTINGS="$(dirname $0)/${scenario}"
    make cloud-prepare

    load_settings
    run_all_clusters verify_gw_topology "$expected_gateways"
    run_all_clusters remove_gw_labels
}

echo "Prepare cluster"
make clusters SETTINGS="$(dirname $0)/no_gw_defined.yml"
declare_kubeconfig

test_gateways no_gw_defined.yml 1
test_gateways two_gw_defined.yml 2
echo "Gateways scenarios have been verified"
