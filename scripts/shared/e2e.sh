#!/usr/bin/env bash

set -em -o pipefail

[[ "${GLOBALNET}" = "true" ]] && gn=-globalnet || gn=
ginkgo_args=()
[[ -n "${FOCUS}" ]] && ginkgo_args+=("-ginkgo.focus=${FOCUS}")
[[ -n "${SKIP}" ]] && ginkgo_args+=("-ginkgo.skip=${SKIP}")

if [[ $# == 0 ]]; then
    echo "At least one cluster to test on must be specified!"
    exit 1
fi

context_clusters=("$@")

source "${SCRIPTS_DIR}/lib/utils"
print_env FOCUS GLOBALNET LAZY_DEPLOY SKIP TESTDIR
source "${SCRIPTS_DIR}/lib/debug_functions"

### Functions ###

function deploy_env_once() {
    if with_context "${context_clusters[0]}" kubectl wait --for=condition=Ready pods -l app=submariner-gateway -n "${SUBM_NS}" --timeout=3s > /dev/null 2>&1; then
        echo "Submariner already deployed, skipping deployment..."
        return
    fi

    make deploy
    declare_kubeconfig
}

function join_by { local IFS="$1"; shift; echo "$*"; }

function generate_kubecontexts() {
    join_by , "${context_clusters[@]}"
}

function test_with_e2e_tests {
    cd "${DAPPER_SOURCE}/${TESTDIR}"

    ${GO:-go} test -v -timeout 30m -args -ginkgo.v -ginkgo.randomizeAllSpecs -ginkgo.trace\
        -submariner-namespace $SUBM_NS "${context_clusters[@]/#/-dp-context=}" ${gn:+"$gn"} \
        -ginkgo.reportPassed -test.timeout 15m \
        "${ginkgo_args[@]}" \
        -ginkgo.reportFile "${DAPPER_OUTPUT}/e2e-junit.xml" 2>&1 | \
        tee "${DAPPER_OUTPUT}/e2e-tests.log"
}

function test_with_subctl {
    subctl verify --only connectivity --kubecontexts "$(generate_kubecontexts)"
}

### Main ###

declare_kubeconfig
[[ "${LAZY_DEPLOY}" != "true" ]] || deploy_env_once

if [ -d "${DAPPER_SOURCE}/${TESTDIR}" ]; then
    test_with_e2e_tests
else
    test_with_subctl
fi

print_clusters_message
