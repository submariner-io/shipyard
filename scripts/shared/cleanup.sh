
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

function cleanup_containers {
    local containers=$(docker ps -qf status=exited)
    if [[ -n ${containers} ]]; then
        echo "Cleaning containers..."
        docker rm -f ${containers}
    fi
}

function cleanup_images {
    local dangling_images=$(docker images -qf dangling=true)
    if [[ -n ${dangling_images} ]]; then
        echo "Cleaning images..."
        docker rmi -f ${dangling_images}
    fi
}

function cleanup_volumes {
    local dangling_volumes=$(docker volume ls -qf dangling=true)
    if [[ -n ${dangling_volumes} ]]; then
        echo "Cleaning volumes..."
        docker volume rm -f ${dangling_volumes}
    fi
}


### Main ###

run_parallel "{1..3}" delete_cluster
stop_local_registry
cleanup_containers
cleanup_images
cleanup_volumes

