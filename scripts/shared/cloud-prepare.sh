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
    kind|aws-ocp)
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
    kubectl label node "${cluster}-${nodes[-1]}" "${GATEWAY_LABEL}" --overwrite
}

function prepare_aws_ocp() {
    subctl cloud prepare aws --kubecontext "${cluster}" --ocp-metadata "${OUTPUT_DIR}/aws-ocp-${cluster}/"
    with_retries 60 sleep_on_fail 5s check_gateway_exists
}

### Main ###

load_settings
declare_kubeconfig
[[ "${PROVIDER}" == "kind" ]] || "${SCRIPTS_DIR}/get-subctl.sh"

# Run in subshell to check response, otherwise `set -e` is not honored
( run_all_clusters cloud_prepare; ) &
wait $! || exit_error "Failed to prepare cloud"

