#!/usr/bin/env bash

set -em -o pipefail

source "${SCRIPTS_DIR}/lib/debug_functions"
source "${SCRIPTS_DIR}/lib/utils"

readonly GATEWAY_LABEL='submariner.io/gateway=true'

### Functions ###

function cloud_prepare() {
    [[ ${cluster_subm[$cluster]} = "true" ]] || return 0
    ! check_gateway_exists || return 0

    case "${PROVIDER}" in
    kind|ocp)
        "prepare_${PROVIDER//-/_}"
        ;;
    *)
        echo "Unknown PROVIDER ${PROVIDER@Q}."
        return 1
    esac
}

function check_gateway_exists() {
    [[ $(kubectl get nodes -l "${GATEWAY_LABEL}" --no-headers | wc -l) -gt 0 ]]
}

function prepare_kind() {
    read -r -a nodes <<< "${cluster_nodes[$cluster]}"
    local node=${cluster}-${nodes[-1]}
    kubectl label node "$node" "$GATEWAY_LABEL" --overwrite

    if [[ "$AIR_GAPPED" = true ]]; then
        local pub_ip
        pub_ip=$(kubectl get nodes "$node" -o jsonpath="{.status.addresses[0].address}")
        kubectl annotate node "$node" gateway.submariner.io/public-ip=ipv4:"$pub_ip"
    fi
}

function prepare_ocp() {
    subctl cloud prepare aws --context "${cluster}" --ocp-metadata "${OUTPUT_DIR}/ocp-${cluster}/"
    with_retries 60 sleep_on_fail 5s check_gateway_exists
}

### Main ###

load_settings
declare_kubeconfig
[[ "${PROVIDER}" == "kind" ]] || "${SCRIPTS_DIR}/get-subctl.sh"

# Run in subshell to check response, otherwise `set -e` is not honored
( run_all_clusters with_retries 3 cloud_prepare; ) &
wait $! || exit_error "Failed to prepare cloud"

