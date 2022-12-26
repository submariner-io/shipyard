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
    local gw_count="${cluster_gateways[$cluster]:-1}"

    readarray -t nodes < <(kubectl get nodes -o yaml | yq '.items[].metadata.name' | sort -r)

    for node in "${nodes[@]:0:$gw_count}"; do
        kubectl label node "$node" "$GATEWAY_LABEL" --overwrite

        [[ "$AIR_GAPPED" = true ]] || continue
        local pub_ip
        pub_ip=$(kubectl get nodes "$node" -o jsonpath="{.status.addresses[0].address}")
        kubectl annotate node "$node" gateway.submariner.io/public-ip=ipv4:"$pub_ip"
    done
}

function prepare_ocp() {
    source "${SCRIPTS_DIR}/lib/ocp_utils"
    local platform
    platform=$(determine_ocp_platform "$OCP_TEMPLATE_DIR")

    # In case of OpenStack, `cloud prepare` addresses it as `rhos`.
    [[ "$platform" != "openstack" ]] || platform=rhos

    subctl cloud prepare "$platform" --context "${cluster}" --ocp-metadata "${OUTPUT_DIR}/ocp-${cluster}/"
    with_retries 60 sleep_on_fail 5s check_gateway_exists
}

### Main ###

load_settings
declare_kubeconfig
[[ "${PROVIDER}" == "kind" ]] || "${SCRIPTS_DIR}/get-subctl.sh"

# Run in subshell to check response, otherwise `set -e` is not honored
( run_all_clusters with_retries 3 cloud_prepare; ) &
wait $! || exit_error "Failed to prepare cloud"

