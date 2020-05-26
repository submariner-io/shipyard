#!/usr/bin/env bash

## Process command line flags ##

source /usr/share/shflags/shflags
DEFINE_string 'cluster_settings' '' "Settings file to customize cluster deployments"
DEFINE_string 'focus' '.*' "Ginkgo focus for the E2E tests"
DEFINE_boolean 'lazy_deploy' true "Deploy the environment lazily (If false, don't do anything)"
FLAGS_HELP="USAGE: $0 [--cluster_settings /path/to/settings] [--focus focus] [--[no]lazy_deploy] cluster [cluster ...]"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

focus="${FLAGS_focus}"
cluster_settings="${FLAGS_cluster_settings}"
[[ "${FLAGS_lazy_deploy}" = "${FLAGS_TRUE}" ]] && lazy_deploy=true || lazy_deploy=false

if [[ $# == 0 ]]; then
    echo "At least one cluster to test on must be specified!"
    exit 1
fi

context_clusters=("$@")

set -em -o pipefail

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

# Always source the shared cluster settings, to set defaults in case something wasn't set in the provided settings
source "${SCRIPTS_DIR}/lib/cluster_settings"
[[ -z "${cluster_settings}" ]] || source ${cluster_settings}

### Functions ###

function deploy_env_once() {
    if with_context "${context_clusters[0]}" kubectl wait --for=condition=Ready pods -l app=submariner-engine -n "${SUBM_NS}" --timeout=3s > /dev/null 2>&1; then
        echo "Submariner already deployed, skipping deployment..."
        return
    fi

    make deploy
    declare_kubeconfig
}

function generate_context_flags() {
    for cluster in ${context_clusters[*]}; do
        printf " -dp-context $cluster"
    done
}

function test_with_e2e_tests {
    cd ${DAPPER_SOURCE}/test/e2e

    go test -v -timeout 30m -args -ginkgo.v -ginkgo.randomizeAllSpecs \
        -submariner-namespace $SUBM_NS $(generate_context_flags) \
        -ginkgo.reportPassed \
        -ginkgo.focus "\[${focus}\]" \
        -ginkgo.reportFile ${DAPPER_OUTPUT}/e2e-junit.xml 2>&1 | \
        tee ${DAPPER_OUTPUT}/e2e-tests.log
}

### Main ###

declare_kubeconfig
[[ "${lazy_deploy}" = "false" ]] || deploy_env_once
test_with_e2e_tests
print_clusters_message
