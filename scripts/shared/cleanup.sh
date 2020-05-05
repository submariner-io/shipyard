
#!/usr/bin/env bash
set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

### Functions ###

function delete_cluster() {
    if kind get clusters | grep -q ${cluster}; then
        kind delete cluster --name=${cluster};
    fi
}

function stop_local_registry {
    if registry_running; then
        echo "Stopping local KIND registry..."
        docker stop $KIND_REGISTRY
    fi
}


### Main ###

run_all_clusters delete_cluster
[[ -z "${DAPPER_OUTPUT}" ]] || rm -rf ${DAPPER_OUTPUT}/*
stop_local_registry
docker system prune --volumes -f

