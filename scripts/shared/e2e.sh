#!/usr/bin/env bash

set -em -o pipefail
source "${SCRIPTS_DIR}/lib/utils"

[[ "${GLOBALNET}" = "true" ]] && gn=-globalnet || gn=

# shellcheck disable=SC2206 # Split on purpose
ginkgo_args=(${TEST_ARGS})
[[ -n "${FOCUS}" ]] && ginkgo_args+=("-ginkgo.focus=${FOCUS}")
[[ -n "${SKIP}" ]] && ginkgo_args+=("-ginkgo.skip=${SKIP}")

print_env FOCUS GLOBALNET LAZY_DEPLOY SKIP SUBCTL_VERIFICATIONS TEST_ARGS TESTDIR
source "${SCRIPTS_DIR}/lib/debug_functions"

### Functions ###

function deploy_env_once() {
    if with_context "${clusters[0]}" kubectl wait --for=condition=Ready pods -l app=submariner-gateway -n "${SUBM_NS}" --timeout=3s > /dev/null 2>&1; then
        echo "Submariner already deployed, skipping deployment..."
        return
    fi

    make deploy
    declare_kubeconfig
}

function join_by { local IFS="$1"; shift; echo "$*"; }

function generate_kubecontexts() {
    join_by , "${clusters[@]}"
}

function test_with_e2e_tests {
    cd "${DAPPER_SOURCE}/${TESTDIR}"

    ${GO:-go} test -v -timeout 30m -args -test.timeout 15m \
        -submariner-namespace $SUBM_NS "${clusters[@]/#/-dp-context=}" ${gn:+"$gn"} \
        -ginkgo.v -ginkgo.randomizeAllSpecs -ginkgo.trace \
        -ginkgo.reportPassed -ginkgo.reportFile "${DAPPER_OUTPUT}/e2e-junit.xml" \
        "${ginkgo_args[@]}" 2>&1 | tee "${DAPPER_OUTPUT}/e2e-tests.log"
}

function test_with_subctl {
    subctl verify --only "${SUBCTL_VERIFICATIONS}" --kubecontexts "$(generate_kubecontexts)"
}

function count_nodes() {
    wc -w <<< "${cluster_nodes[${clusters[$1]}]}"
}

# Make sure the biggest cluster is always first, as some tests rely on having a big first cluster.
function order_clusters {
    local biggest_cluster=0
    for i in "${!clusters[@]}"; do
        if [[ $(count_nodes "$i") -gt $(count_nodes "${biggest_cluster}") ]]; then
            biggest_cluster="$i"
        fi
    done

    local orig_cluster="${clusters[0]}"
    clusters[0]="${clusters[$biggest_cluster]}"
    clusters[$biggest_cluster]="${orig_cluster}"
}

### Main ###

load_settings
order_clusters
declare_kubeconfig
[[ "${LAZY_DEPLOY}" != "true" ]] || deploy_env_once

if [ -d "${DAPPER_SOURCE}/${TESTDIR}" ]; then
    test_with_e2e_tests
else
    test_with_subctl
fi

print_clusters_message
