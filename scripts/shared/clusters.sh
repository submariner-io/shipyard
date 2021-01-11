#!/usr/bin/env bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'k8s_version' '1.17.0' 'Version of K8s to use'
DEFINE_string 'olm_version' '0.14.1' 'Version of OLM to use'
DEFINE_boolean 'olm' false 'Deploy OLM'
DEFINE_boolean 'prometheus' false 'Deploy Prometheus'
DEFINE_boolean 'globalnet' false "Deploy with operlapping CIDRs (set to 'true' to enable)"
DEFINE_boolean 'registry_inmemory' true "Run local registry in memory to speed up the image loading."
DEFINE_string 'cluster_settings' '' "Settings file to customize cluster deployments"
DEFINE_string 'timeout' '5m' "Timeout flag to pass to kubectl when waiting (e.g. 30s)"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

version="${FLAGS_k8s_version}"
olm_version="${FLAGS_olm_version}"
[[ "${FLAGS_olm}" = "${FLAGS_TRUE}" ]] && olm=true || olm=false
[[ "${FLAGS_prometheus}" = "${FLAGS_TRUE}" ]] && prometheus=true || prometheus=false
[[ "${FLAGS_globalnet}" = "${FLAGS_TRUE}" ]] && globalnet=true || globalnet=false
[[ "${FLAGS_registry_inmemory}" = "${FLAGS_TRUE}" ]] && registry_inmemory=true || registry_inmemory=false
cluster_settings="${FLAGS_cluster_settings}"
timeout="${FLAGS_timeout}"
echo "Running with: k8s_version=${version}, olm_version=${olm_version}, olm=${olm}, globalnet=${globalnet}, registry_inmemory=${registry_inmemory}, cluster_settings=${cluster_settings}, timeout=${timeout}"

set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

# Always source the shared cluster settings, to set defaults in case something wasn't set in the provided settings
source "${SCRIPTS_DIR}/lib/cluster_settings"
[[ -z "${cluster_settings}" ]] || source ${cluster_settings}

cat << EOM
Cluster settings::
  broker - ${broker@Q}
  clusters - ${clusters[*]@Q}
  cni - $(typeset -p cluster_cni | cut -f 2- -d=)
  nodes per cluster - $(typeset -p cluster_nodes | cut -f 2- -d=)
  install submariner - $(typeset -p cluster_subm | cut -f 2- -d=)
EOM

### Functions ###

function render_template() {
    eval "echo \"$(cat $1)\""
}

function generate_cluster_yaml() {
    local pod_cidr="${cluster_CIDRs[${cluster}]}"
    local service_cidr="${service_CIDRs[${cluster}]}"
    local dns_domain="${cluster}.local"
    local disable_cni="false"
    [[ -z "${cluster_cni[$cluster]}" ]] || disable_cni="true"

    local nodes
    for node in ${cluster_nodes[${cluster}]}; do nodes="${nodes}"$'\n'"- role: $node"; done

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
    rm -f "$KUBECONFIG"

    if kind get clusters | grep -q "^${cluster}$"; then
        echo "KIND cluster already exists, skipping its creation..."
        kind export kubeconfig --name=${cluster}
        kind_fixup_config
        return
    fi

    echo "Creating KIND cluster..."
    if [[ "${cluster_cni[$cluster]}" == "ovn" ]]; then
        deploy_kind_ovn
        return
    fi

    generate_cluster_yaml
    local image_flag=''
    if [[ -n ${version} ]]; then
        image_flag="--image=kindest/node:v${version}"
    fi

    kind create cluster $image_flag --name=${cluster} --config=${RESOURCES_DIR}/${cluster}-config.yaml
    kind_fixup_config

    ( deploy_cluster_capabilities; ) &
    if ! wait $! ; then
        echo "Failed to deploy cluster capabilities, removing the cluster"
        kind delete cluster --name=${cluster}
        return 1
    fi
}

function deploy_cni() {
    [[ -n "${cluster_cni[$cluster]}" ]] || return 0

    eval "deploy_${cluster_cni[$cluster]}_cni"
}

function deploy_weave_cni(){
    echo "Applying weave network..."
    kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=v$version&env.IPALLOC_RANGE=${cluster_CIDRs[${cluster}]}"
    echo "Waiting for weave-net pods to be ready..."
    kubectl wait --for=condition=Ready pods -l name=weave-net -n kube-system --timeout="${timeout}"
    echo "Waiting for core-dns deployment to be ready..."
    kubectl -n kube-system rollout status deploy/coredns --timeout="${timeout}"
}

function deploy_ovn_cni(){
  echo "OVN CNI deployed."
}

function deploy_kind_ovn(){
    export K8s_VERSION="${version}"
    export NET_CIDR_IPV4="${cluster_CIDRs[${cluster}]}"
    export SVC_CIDR_IPV4="${service_CIDRs[${cluster}]}"
    export KIND_CLUSTER_NAME="${cluster}"
    export OVN_IMAGE="quay.io/vthapar/ovn-daemonset-f:latest"
    export REGISTRY_IP="${registry_ip}"

    (  cd ${OVN_DIR}/contrib; ./kind.sh; ) &
    if ! wait $! ; then
        echo "Failed to install kind with OVN"
        kind delete cluster --name=${cluster}
        return 1
    fi

    ( deploy_cluster_capabilities; ) &
    if ! wait $! ; then
        echo "Failed to deploy cluster capabilities, removing the cluster"
        kind delete cluster --name=${cluster}
        return 1
    fi
}

function run_local_registry() {
    # Run a local registry to avoid loading images manually to kind
    if registry_running; then
        echo "Local registry $KIND_REGISTRY already running."
    else
        echo "Deploying local registry $KIND_REGISTRY to serve images centrally."
        local volume_flag
        [[ $registry_inmemory != "true" ]] || volume_flag="-v /dev/shm/${KIND_REGISTRY}:/var/lib/registry"
        docker run -d $volume_flag -p 5000:5000 --restart=always --name $KIND_REGISTRY registry:2
    fi

    # This IP is consumed by kind to point the registry mirror correctly to the local registry
    registry_ip="$(docker inspect -f '{{.NetworkSettings.IPAddress}}' "$KIND_REGISTRY")"
}

function deploy_olm() {
    echo "Applying OLM CRDs..."
    kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$olm_version/crds.yaml --validate=false
    echo "Applying OLM resources..."
    kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$olm_version/olm.yaml

    echo "Waiting for olm-operator deployment to be ready..."
    kubectl rollout status deployment/olm-operator --namespace=olm --timeout="${timeout}"
    echo "Waiting for catalog-operator deployment to be ready..."
    kubectl rollout status deployment/catalog-operator --namespace=olm --timeout="${timeout}"
    echo "Waiting for packageserver deployment to be ready..."
    kubectl rollout status deployment/packageserver --namespace=olm --timeout="${timeout}"
}

function deploy_prometheus() {
    echo "Deploying Prometheus..."
    # TODO Install in a separate namespace
    kubectl create ns submariner-operator
    # Bundle from prometheus-operator, namespace changed to submariner-operator
    kubectl apply -f ${SCRIPTS_DIR}/resources/prometheus/bundle.yaml
    kubectl apply -f ${SCRIPTS_DIR}/resources/prometheus/serviceaccount.yaml
    kubectl apply -f ${SCRIPTS_DIR}/resources/prometheus/clusterrole.yaml
    kubectl apply -f ${SCRIPTS_DIR}/resources/prometheus/clusterrolebinding.yaml
    kubectl apply -f ${SCRIPTS_DIR}/resources/prometheus/prometheus.yaml
}

function deploy_cluster_capabilities() {
    deploy_cni
    [[ $olm != "true" ]] || deploy_olm
    [[ $prometheus != "true" ]] || deploy_prometheus
}

function warn_inotify() {
    if [[ "$(cat /proc/sys/fs/inotify/max_user_instances)" -lt 512 ]]; then
        echo "Please increase your inotify settings (currently $(cat /proc/sys/fs/inotify/max_user_watches) and $(cat /proc/sys/fs/inotify/max_user_instances)):"
        echo sudo sysctl fs.inotify.max_user_watches=524288
        echo sudo sysctl fs.inotify.max_user_instances=512
        echo 'See https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files'
    fi
}

### Main ###

rm -rf ${KUBECONFIGS_DIR}
mkdir -p ${KUBECONFIGS_DIR}

run_local_registry
declare_cidrs

# Run in subshell to check response, otherwise `set -e` is not honored
( run_all_clusters with_retries 3 create_kind_cluster; ) &
if ! wait $!; then
    warn_inotify
    exit 1
fi

print_clusters_message
