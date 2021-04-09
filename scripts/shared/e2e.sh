#!/usr/bin/env bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'cluster_settings' '' "Settings file to customize cluster deployments"
DEFINE_string 'focus' '' "Ginkgo focus for the E2E tests"
DEFINE_string 'skip' '' "Ginkgo skip for the E2E tests"
DEFINE_string 'testdir' 'test/e2e' "Directory under to be used for E2E testing"
DEFINE_boolean 'lazy_deploy' true "Deploy the environment lazily (If false, don't do anything)"
DEFINE_boolean 'globalnet' false "Indicates if the globalnet feature is enabled"
FLAGS_HELP="USAGE: $0 [--cluster_settings /path/to/settings] [--focus focus] [--skip skip] [--[no]lazy_deploy] [--testdir test/e2e] cluster [cluster ...]"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

[[ "${FLAGS_globalnet}" = "${FLAGS_TRUE}" ]] && globalnet=-globalnet || globalnet=
ginkgo_args=()
[[ -n "${FLAGS_focus}" ]] && ginkgo_args+=("-ginkgo.focus=${FLAGS_focus}")
[[ -n "${FLAGS_skip}" ]] && ginkgo_args+=("-ginkgo.skip=${FLAGS_skip}")
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
    if with_context "${context_clusters[0]}" kubectl wait --for=condition=Ready pods -l app=submariner-gateway -n "${SUBM_NS}" --timeout=3s > /dev/null 2>&1; then
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

function generate_kubeconfigs() {
    for cluster in "${context_clusters[@]}"; do
	grep -l "current-context: ${cluster}" output/kubeconfigs/*
    done
}

function test_with_e2e_tests {
    cd ${DAPPER_SOURCE}/${FLAGS_testdir}

    go test -v -timeout 30m -args -ginkgo.v -ginkgo.randomizeAllSpecs -ginkgo.trace\
        -submariner-namespace $SUBM_NS $(generate_context_flags) ${globalnet} \
        -ginkgo.reportPassed -test.timeout 15m \
        "${ginkgo_args[@]}" \
        -ginkgo.reportFile ${DAPPER_OUTPUT}/e2e-junit.xml 2>&1 | \
        tee ${DAPPER_OUTPUT}/e2e-tests.log
}

function test_with_subctl {
    subctl verify --only connectivity $(generate_kubeconfigs)
}

### Main ###

declare_kubeconfig
[[ "${lazy_deploy}" = "false" ]] || deploy_env_once

if [ -d ${DAPPER_SOURCE}/${FLAGS_testdir} ]; then
    test_with_e2e_tests
else
    test_with_subctl
fi

print_clusters_message
