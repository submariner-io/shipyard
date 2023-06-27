#!/usr/bin/env bash

set -em -o pipefail

source "${SCRIPTS_DIR}/lib/utils"
print_env AIR_GAPPED CABLE_DRIVER GLOBALNET K8S_VERSION OLM OLM_VERSION PARALLEL PROMETHEUS PROVIDER SETTINGS TIMEOUT
source "${SCRIPTS_DIR}/lib/debug_functions"

### Functions ###

function deploy_olm() {
    echo "Applying OLM CRDs..."
    kubectl apply -f "https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/crds.yaml" --validate=false
    kubectl wait --for=condition=Established -f "https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/crds.yaml"
    echo "Applying OLM resources..."
    kubectl apply -f "https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/olm.yaml"

    echo "Waiting for olm-operator deployment to be ready..."
    kubectl rollout status deployment/olm-operator --namespace=olm --timeout="${TIMEOUT}"
    echo "Waiting for catalog-operator deployment to be ready..."
    kubectl rollout status deployment/catalog-operator --namespace=olm --timeout="${TIMEOUT}"
    echo "Waiting for packageserver deployment to be ready..."
    kubectl rollout status deployment/packageserver --namespace=olm --timeout="${TIMEOUT}"
}

function deploy_prometheus() {
    echo "Deploying Prometheus..."
    # TODO Install in a separate namespace
    kubectl create ns submariner-operator
    # Bundle from prometheus-operator, namespace changed to submariner-operator
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/bundle.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/serviceaccount.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/clusterrole.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/clusterrolebinding.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/prometheus.yaml"
}

function deploy_cluster_capabilities() {
    [[ "${OLM}" != "true" ]] || deploy_olm
    [[ "${PROMETHEUS}" != "true" ]] || deploy_prometheus
}

### Main ###

mkdir -p "${KUBECONFIGS_DIR}"

load_settings
declare_cidrs

load_library clusters PROVIDER
provider_prepare

# Run in subshell to check response, otherwise `set -e` is not honored
( run_all_clusters with_retries 3 provider_create_cluster; ) &
if ! wait $!; then
    run_if_defined provider_failed
    exit_error "Failed to create clusters using ${PROVIDER@Q}."
fi

declare_kubeconfig
run_if_defined provider_succeeded
run_all_clusters deploy_cluster_capabilities
print_clusters_message
