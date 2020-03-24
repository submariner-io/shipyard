
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
    registry_running || return 0

    echo "Stopping local KIND registry..."
    docker stop $KIND_REGISTRY
}


### Main ###

run_parallel "{1..3}" delete_cluster
stop_local_registry
docker system prune --volumes -f

