#!/usr/bin/env bash

set -em

source "${SCRIPTS_DIR}/lib/utils"
print_env PLUGIN
source "${SCRIPTS_DIR}/lib/debug_functions"

# Source plugin if the path is passed via plugin argument and the file exists
# shellcheck disable=SC1090
[[ -n "${PLUGIN}" ]] && [[ -f "${PLUGIN}" ]] && source "${PLUGIN}"

### Functions ###

function delete_cluster() {
    kind delete cluster --name="${cluster}"
}

function stop_local_registry {
    if registry_running; then
        echo "Stopping local KIND registry..."
        docker stop "$KIND_REGISTRY"
    fi
}


### Main ###

readarray -t clusters < <(kind get clusters)

run_if_defined pre_cleanup

# run_parallel expects cluster names as a single argument
run_parallel "${clusters[*]}" delete_cluster
[[ -z "${DAPPER_OUTPUT}" ]] || rm -rf "${DAPPER_OUTPUT:?}"/*

stop_local_registry
docker system prune --volumes -f

run_if_defined post_cleanup
