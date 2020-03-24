#!/usr/bin/env bash
set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

### Functions ###

function render_template() {
    eval "echo \"$(cat $1)\""
}

function generate_cluster_yaml() {
    local pod_cidr="${cluster_CIDRs[${cluster}]}"
    local service_cidr="${service_CIDRs[${cluster}]}"
    local dns_domain="${cluster}.local"
    local disable_cni="true"
    if [[ "${cluster}" = "cluster1" ]]; then
        disable_cni="false"
    fi

    render_template ${RESOURCES_DIR}/kind-cluster-config.yaml > ${RESOURCES_DIR}/${cluster}-config.yaml
}

function kind_fixup_config() {
    local master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${cluster}-control-plane | head -n 1)
    sed -i -- "s/server: .*/server: https:\/\/$master_ip:6443/g" $KUBECONFIG
    sed -i -- "s/user: kind-.*/user: ${cluster}/g" $KUBECONFIG
    sed -i -- "s/name: kind-.*/name: ${cluster}/g" $KUBECONFIG
    sed -i -- "s/cluster: kind-.*/cluster: ${cluster}/g" $KUBECONFIG
    sed -i -- "s/current-context: .*/current-context: ${cluster}/g" $KUBECONFIG
    chmod a+r $KUBECONFIG
}

function create_kind_cluster() {
    export KUBECONFIG=${KUBECONFIGS_DIR}/kind-config-${cluster}
    if [[ $(kind get clusters | grep "^${cluster}$" | wc -l) -gt 0  ]]; then
        echo "KIND cluster already exists, skipping its creation..."
        kind export kubeconfig --name=${cluster}
        kind_fixup_config
        return
    fi

    echo "Creating KIND cluster..."
    generate_cluster_yaml
    local image_flag=''
    if [[ -n ${version} ]]; then
        image_flag="--image=kindest/node:v${version}"
    fi

    kind create cluster $image_flag --name=${cluster} --config=${RESOURCES_DIR}/${cluster}-config.yaml
    kind_fixup_config
}

function deploy_weave_cni(){
    if kubectl wait --for=condition=Ready pods -l name=weave-net -n kube-system --timeout=60s > /dev/null 2>&1; then
        echo "Weave already deployed."
        return
    fi

    echo "Applying weave network..."
    kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=v$version&env.IPALLOC_RANGE=${cluster_CIDRs[${cluster}]}"
    echo "Waiting for weave-net pods to be ready..."
    kubectl wait --for=condition=Ready pods -l name=weave-net -n kube-system --timeout=300s
    echo "Waiting for core-dns deployment to be ready..."
    kubectl -n kube-system rollout status deploy/coredns --timeout=300s
}

function run_local_registry() {
    # Run a local registry to avoid loading images manually to kind
    if registry_running; then
        echo "Local registry $KIND_REGISTRY already running."
    else
        echo "Deploying local registry $KIND_REGISTRY to serve images centrally."
        docker run -d -p 5000:5000 --restart=always --name $KIND_REGISTRY registry:2
    fi

    # This IP is consumed by kind to point the registry mirror correctly to the local registry
    registry_ip="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' "$KIND_REGISTRY")"
}


### Main ###

LONGOPTS=k8s_version:,globalnet:
# Only accept longopts, but must pass null shortopts or first param after "--" will be incorrectly used
SHORTOPTS=""
! PARSED=$(getopt --options=$SHORTOPTS --longoptions=$LONGOPTS --name "$0" -- "$@")
eval set -- "$PARSED"

while true; do
    case "$1" in
        --k8s_version)
            version="$2"
            ;;
        --globalnet)
            globalnet="$2"
            ;;
        --)
            break
            ;;
        *)
            echo "Ignoring unknown option: $1 $2"
            ;;
    esac
    shift 2
done

echo "Running with: k8s_version=${version}, globalnet=${globalnet}"

rm -rf ${KUBECONFIGS_DIR}
mkdir -p ${KUBECONFIGS_DIR}

run_local_registry
declare_cidrs
run_parallel "{1..3}" create_kind_cluster
declare_kubeconfig
run_parallel "2 3" deploy_weave_cni

