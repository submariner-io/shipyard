#!/usr/bin/env bash

## Process command line flags ##

source "${SCRIPTS_DIR}/lib/shflags"
DEFINE_string 'plugin' '' "Path to the plugin that has pre_cleanup and post_cleanup hook"

FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

set -em

source "${SCRIPTS_DIR}/lib/debug_functions"
source "${SCRIPTS_DIR}/lib/utils"

# Source plugin if the path is passed via plugin argument and the file exists
# shellcheck disable=SC1090
[[ -n "${FLAGS_plugin}" ]] && [[ -f "${FLAGS_plugin}" ]] && source "${FLAGS_plugin}"

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
[[ -z "${DAPPER_OUTPUT}" ]] || rm -rf "${DAPPER_OUTPUT}"/*

stop_local_registry
docker system prune --volumes -f

run_if_defined post_cleanup
