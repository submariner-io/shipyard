#!/usr/bin/env bash
set -em

source ${SCRIPTS_DIR}/lib/debug_functions

### Constants ###

readonly RESOURCES_DIR=${SCRIPTS_DIR}/resources
readonly OUTPUT_DIR=${DAPPER_OUTPUT}
readonly KUBECONFIGS_DIR=${DAPPER_OUTPUT}/kubeconfigs
readonly KIND_REGISTRY=kind-registry

### Functions ###

# Mask kubectl to use cluster context if the variable is set and context isn't specified,
# otherwise use the config context as always.
function kubectl() {
    context_flag=""
    if [[ -n "${cluster}" && ! "${@}" =~ "context" ]]; then
        context_flag="--context=${cluster}"
    fi
    command kubectl ${context_flag} "$@"
}

# Run cluster commands in parallel.
# 1st argument is the numbers of the clusters to run for, supports "1 2 3" or "{1..3}" for range
# 2nd argument is the command to execute, which will have the $cluster variable set.
function run_parallel() {
    clusters=$(eval echo "$1")
    cmnd=$2
    declare -A pids
    for i in ${clusters}; do
        cluster="cluster${i}"
        ( $cmnd | sed "s/^/[${cluster}] /" ) &
        unset cluster
        pids["${i}"]=$!
    done

    for i in ${!pids[@]}; do
        wait ${pids[$i]}
    done
}

function render_template() {
    eval "echo \"$(cat $1)\""
}

function generate_cluster_yaml() {
    pod_cidr="${cluster_CIDRs[$1]}"
    service_cidr="${service_CIDRs[$1]}"
    dns_domain="$1.local"
    disable_cni="true"
    if [[ "$1" = "cluster1" ]]; then
        disable_cni="false"
    fi

    render_template ${RESOURCES_DIR}/kind-cluster-config.yaml > ${RESOURCES_DIR}/$1-config.yaml
}

function kind_fixup_config() {
    master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${cluster}-control-plane | head -n 1)
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
        kind_fixup_config ${cluster}
        return
    fi

    echo "Creating KIND cluster..."
    generate_cluster_yaml "${cluster}"
    image_flag=''
    if [[ -n ${version} ]]; then
        image_flag="--image=kindest/node:v${version}"
    fi

    kind create cluster $image_flag --name=${cluster} --config=${RESOURCES_DIR}/${cluster}-config.yaml
    kind_fixup_config ${cluster}
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

function registry_running() {
    docker ps --filter name="^/?$KIND_REGISTRY$" | grep $KIND_REGISTRY
    return $?
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

if [[ $globalnet = "true" ]]; then
    # When globalnet is set to true, we want to deploy clusters with overlapping CIDRs
    declare -A cluster_CIDRs=( ["cluster1"]="10.244.0.0/16" ["cluster2"]="10.244.0.0/16" ["cluster3"]="10.244.0.0/16" )
    declare -A service_CIDRs=( ["cluster1"]="100.94.0.0/16" ["cluster2"]="100.94.0.0/16" ["cluster3"]="100.94.0.0/16" )
    declare -A global_CIDRs=( ["cluster1"]="169.254.1.0/24" ["cluster2"]="169.254.2.0/24" ["cluster3"]="169.254.3.0/24" )
else
    declare -A cluster_CIDRs=( ["cluster1"]="10.244.0.0/16" ["cluster2"]="10.245.0.0/16" ["cluster3"]="10.246.0.0/16" )
    declare -A service_CIDRs=( ["cluster1"]="100.94.0.0/16" ["cluster2"]="100.95.0.0/16" ["cluster3"]="100.96.0.0/16" )
fi

rm -rf ${KUBECONFIGS_DIR}
mkdir -p ${KUBECONFIGS_DIR}

# Run a local registry to avoid loading images manually to kind
if registry_running; then
    echo Local registry $KIND_REGISTRY already running.
else
    echo Deploying local registry $KIND_REGISTRY to serve images centrally.
    docker run -d -p 5000:5000 --restart=always --name $KIND_REGISTRY registry:2
fi

# This IP is consumed by kind to point the registry mirror correctly to the local registry
registry_ip="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' "$KIND_REGISTRY")"

run_parallel "{1..3}" create_kind_cluster
export KUBECONFIG=$(echo ${KUBECONFIGS_DIR}/kind-config-cluster{1..3} | sed 's/ /:/g')
run_parallel "2 3" deploy_weave_cni

