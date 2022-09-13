#!/usr/bin/env bash

set -e

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils
# source ${SCRIPTS_DIR}/cloud-prepare.sh

function verify_gw_topology() {
    local gateways_count
    gateways="${cluster_gateways[$cluster]:-1}"

    echo "Verify gateway nodes"
    gateways_count=$(kubectl get nodes -l=submariner.io/gateway=true \
        --no-headers=true -o custom-columns=NAME:.metadata.name | wc -l)

    if [[ "$gateways" -ne "$gateways_count" ]]; then
        echo "Expect $gateways gateways nodes but detected $gateways_count"
        return 1
    else
        echo "Found expected number of gateways - $gateways"
    fi
}

function remove_gw_labels() {
    local gw_nodes

    echo "Reset gateway nodes (unlabel)"
    readarray -t gw_nodes < <(kubectl get nodes -l=submariner.io/gateway=true \
        --no-headers=true -o custom-columns=NAME:.metadata.name)
    
    for node in "${gw_nodes[@]}"; do
        kubectl label --overwrite nodes "$node" submariner.io/gateway-
    done
}

echo "Prepare cluster"
make clusters SETTINGS=test/scripts/validation/scenario_config1

for scenario in scenario_config1 scenario_config2 scenario_config3; do
    export SETTINGS="test/scripts/validation/$scenario"
    echo "Set gateway nodes according to $scenario scenario"
    make cloud-prepare

    declare_kubeconfig
    load_settings
    clusters=($(kind get clusters))
    run_parallel "${clusters[*]}" verify_gw_topology
    run_parallel "${clusters[*]}" remove_gw_labels
done

echo "Perform cleanup"
make cleanup SETTINGS=test/scripts/validation/scenario_config1
