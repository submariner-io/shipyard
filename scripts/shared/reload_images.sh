#!/usr/bin/env bash

set -e

source "${SCRIPTS_DIR}/lib/utils"
print_env RESTART
source "${SCRIPTS_DIR}/lib/debug_functions"
source "${SCRIPTS_DIR}/lib/deploy_funcs"

function find_resources() {
    local resource_type=$1
    kubectl -n "$(find_submariner_namespace)" get "${resource_type}" -o jsonpath="{range .items[*]}{.metadata.name}{'\n'}"
}

load_settings
declare_kubeconfig

case "${RESTART}" in
    none)
        ;;
    all)
        for resource in $(find_resources deployments); do
            run_subm_clusters reload_pods deployment "${resource}"
        done

        for resource in $(find_resources daemonsets); do
            run_subm_clusters reload_pods daemonset "${resource}"
        done
        ;;
    *)
        run_subm_clusters reload_pods deployment "submariner-${restart}" || \
            run_subm_clusters reload_pods daemonset "submariner-${restart}" || :
        ;;
esac

