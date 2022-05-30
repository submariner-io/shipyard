#!/usr/bin/env bash

## Kubernetes version mapping, as supported by kind ##
# See the release notes of the kind version in use
DEFAULT_K8S_VERSION=1.23
declare -A kind_k8s_versions
kind_k8s_versions[1.17]=1.17.17@sha256:e477ee64df5731aa4ef4deabbafc34e8d9a686b49178f726563598344a3898d5
kind_k8s_versions[1.18]=1.18.20@sha256:e3dca5e16116d11363e31639640042a9b1bd2c90f85717a7fc66be34089a8169
kind_k8s_versions[1.19]=1.19.16@sha256:81f552397c1e6c1f293f967ecb1344d8857613fb978f963c30e907c32f598467
kind_k8s_versions[1.20]=1.20.15@sha256:393bb9096c6c4d723bb17bceb0896407d7db581532d11ea2839c80b28e5d8deb
kind_k8s_versions[1.21]=1.21.10@sha256:84709f09756ba4f863769bdcabe5edafc2ada72d3c8c44d6515fc581b66b029c
kind_k8s_versions[1.22]=1.22.7@sha256:1dfd72d193bf7da64765fd2f2898f78663b9ba366c2aa74be1fd7498a1873166
kind_k8s_versions[1.23]=1.23.4@sha256:0e34f0d0fd448aa2f2819cfd74e99fe5793a6e4938b328f657c8e3f81ee0dfb9

## Process command line flags ##

source "${SCRIPTS_DIR}/lib/shflags"
DEFINE_string 'k8s_version' "${DEFAULT_K8S_VERSION}" 'Version of K8s to use'
DEFINE_string 'olm_version' 'v0.18.3' 'Version of OLM to use'
DEFINE_boolean 'olm' false 'Deploy OLM'
DEFINE_boolean 'prometheus' false 'Deploy Prometheus'
DEFINE_boolean 'globalnet' false "Deploy with operlapping CIDRs (set to 'true' to enable)"
DEFINE_boolean 'registry_inmemory' true "Run local registry in memory to speed up the image loading."
DEFINE_string 'settings' '' "Settings YAML file to customize cluster deployments"
DEFINE_string 'timeout' '5m' "Timeout flag to pass to kubectl when waiting (e.g. 30s)"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

k8s_version="${FLAGS_k8s_version}"
olm_version="${FLAGS_olm_version}"
[[ -z "${k8s_version}" ]] && k8s_version="${DEFAULT_K8S_VERSION}"
[[ -n "${kind_k8s_versions[$k8s_version]}" ]] && k8s_version="${kind_k8s_versions[$k8s_version]}"
[[ "${FLAGS_olm}" = "${FLAGS_TRUE}" ]] && olm=true || olm=false
[[ "${FLAGS_prometheus}" = "${FLAGS_TRUE}" ]] && prometheus=true || prometheus=false
[[ "${FLAGS_globalnet}" = "${FLAGS_TRUE}" ]] && globalnet=true || globalnet=false
[[ "${FLAGS_registry_inmemory}" = "${FLAGS_TRUE}" ]] && registry_inmemory=true || registry_inmemory=false
settings="${FLAGS_settings}"
timeout="${FLAGS_timeout}"

echo "Running with: k8s_version=${k8s_version}, olm_version=${olm_version}, olm=${olm}, globalnet=${globalnet}, prometheus=${prometheus}, registry_inmemory=${registry_inmemory}, settings=${settings}, timeout=${timeout}"

set -em

source "${SCRIPTS_DIR}/lib/debug_functions"
source "${SCRIPTS_DIR}/lib/utils"

### Functions ###

function generate_cluster_yaml() {
    # These are used by render_template
    local pod_cidr service_cidr dns_domain disable_cni
    # shellcheck disable=SC2034
    pod_cidr="${cluster_CIDRs[${cluster}]}"
    # shellcheck disable=SC2034
    service_cidr="${service_CIDRs[${cluster}]}"
    # shellcheck disable=SC2034
    dns_domain="${cluster}.local"
    disable_cni="false"
    # shellcheck disable=SC2034
    [[ -z "${cluster_cni[$cluster]}" ]] || disable_cni="true"

    local nodes
    for node in ${cluster_nodes[${cluster}]}; do nodes="${nodes}"$'\n'"- role: $node"; done

    render_template "${RESOURCES_DIR}/kind-cluster-config.yaml" > "${RESOURCES_DIR}/${cluster}-config.yaml"
}

function kind_fixup_config() {
    local master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${cluster}-control-plane" | head -n 1)
    sed -i -- "s/server: .*/server: https:\/\/$master_ip:6443/g" "$KUBECONFIG"
    sed -i -- "s/user: kind-.*/user: ${cluster}/g" "$KUBECONFIG"
    sed -i -- "s/name: kind-.*/name: ${cluster}/g" "$KUBECONFIG"
    sed -i -- "s/cluster: kind-.*/cluster: ${cluster}/g" "$KUBECONFIG"
    sed -i -- "s/current-context: .*/current-context: ${cluster}/g" "$KUBECONFIG"
    chmod a+r "$KUBECONFIG"
}

# In development environments where clusters are brought up and down
# multiple times, several Docker images are repeatedly pulled and deleted,
# leading to the Docker error:
#   "toomanyrequests: You have reached your pull rate limit"
# Preload the KIND image. Also tag it so the `docker system prune` during
# cleanup won't remove it.
function download_kind() {
    if [[ -z "${k8s_version}" ]]; then
        echo "k8s_version not set."
        return
    fi

    # Example: kindest/node:v1.20.7@sha256:cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9
    kind_image="kindest/node:v${k8s_version}"
    # Example: kindest/node:v1.20.7
    kind_image_tag="kindest/node:v$(echo ${k8s_version}|awk -F"@" '{print $1}')"
    # Example: kindest/node:@sha256:cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9
    kind_image_sha="kindest/node@$(echo ${k8s_version}|awk -F"@" '{print $2}')"

    # Check if image is already present, and if not, download it.
    echo "Processing Image: $kind_image_tag ($kind_image)"
    if [[ -n $(docker images -q "$kind_image_tag") ]] ; then
        echo "Image $kind_image_tag already downloaded."
        return
    fi

    echo "Image $kind_image_tag not found, downloading..."
    if ! docker pull "$kind_image"; then
        echo "**** 'docker pull $kind_image' failed. Manually run. ****"
        return
    fi

    image_id=$(docker images -q "$kind_image_sha")
    if ! docker tag "$image_id" "$kind_image_tag"; then
        echo "'docker tag ${image_id} ${kind_image_tag}' failed."
    fi
}

function create_kind_cluster() {
    export KUBECONFIG=${KUBECONFIGS_DIR}/kind-config-${cluster}
    rm -f "$KUBECONFIG"

    if kind get clusters | grep -q "^${cluster}$"; then
        echo "KIND cluster already exists, skipping its creation..."
        kind export kubeconfig --name="${cluster}"
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
    if [[ -n ${k8s_version} ]]; then
        image_flag="--image=kindest/node:v${k8s_version}"
    fi

    kind version
    cat "${RESOURCES_DIR}/${cluster}-config.yaml"
    kind create cluster ${image_flag:+"$image_flag"} --name="${cluster}" --config="${RESOURCES_DIR}/${cluster}-config.yaml"
    kind_fixup_config

    ( deploy_cluster_capabilities; ) &
    if ! wait $! ; then
        echo "Failed to deploy cluster capabilities, removing the cluster"
        kubectl cluster-info dump 1>&2
        kind delete cluster --name="${cluster}"
        return 1
    fi
}

function deploy_cni() {
    [[ -n "${cluster_cni[$cluster]}" ]] || return 0

    eval "deploy_${cluster_cni[$cluster]}_cni"
}

function deploy_weave_cni(){
    echo "Applying weave network..."

    WEAVE_YAML=$(curl -sL "https://cloud.weave.works/k8s/net?k8s-version=v$k8s_version&env.IPALLOC_RANGE=${cluster_CIDRs[${cluster}]}" | sed 's!ghcr.io/weaveworks/launcher!weaveworks!')

    # Search the YAML for images that need to be downloaded
    readarray -t IMAGE_LIST < <(echo "${WEAVE_YAML}" | yq e '.items[].spec.template.spec.containers[].image, .items[].spec.template.spec.initContainers[].image' -)
    echo "IMAGE_LIST=${IMAGE_LIST[@]}"
    for image in "${IMAGE_LIST[@]}"
    do
        IMAGE_FAILURE=false

        # Check if image is already present, and if not, download it.
        echo "Processing Image: $image"
        if [ -z "`docker images -q "$image"`" ] ; then
            echo "Image $image not found, downloading..."
            if ! docker pull "$image"; then
                echo "**** 'docker pull $image' failed. Manually run. ****"
                IMAGE_FAILURE=true
            fi
        else
            echo "Image $image already downloaded."
        fi

        if [ "${IMAGE_FAILURE}" == false ] ; then
            LCL_REG_IMAGE_NAME="${image/weaveworks/localhost:5000}"
            # Copy image to local registry if not there
            if [ -z "$(docker images -q "${LCL_REG_IMAGE_NAME}")" ] ; then
                echo "Image ${LCL_REG_IMAGE_NAME} not found, tagging and pushing ..."
                if ! docker tag "$image" "${LCL_REG_IMAGE_NAME}"; then
                    echo "'docker tag $image ${LCL_REG_IMAGE_NAME}' failed."
                    IMAGE_FAILURE=true
                else
                    if ! docker push "${LCL_REG_IMAGE_NAME}"; then
                        echo "'docker push ${LCL_REG_IMAGE_NAME}' failed."
                        IMAGE_FAILURE=true
                    fi
                fi
            else
                echo "Image ${LCL_REG_IMAGE_NAME} already present."
            fi
        fi

        if [ "${IMAGE_FAILURE}" == false ] ; then
            # Update the YAML by replacing upstream image name with the local registry image name
            WEAVE_YAML=$(echo "${WEAVE_YAML}" | \
                    image=${image} \
                    LCL_REG_IMAGE_NAME=${LCL_REG_IMAGE_NAME} \
                    yq e 'with(.items[] | select(.kind == "DaemonSet")| .spec.template.spec.containers[].image | select(. == strenv(image));
                            . = strenv(LCL_REG_IMAGE_NAME) | . style="single") |
                          with(.items[] | select(.kind == "DaemonSet")| .spec.template.spec.initContainers[].image | select(. == strenv(image));
                            . = strenv(LCL_REG_IMAGE_NAME) | . style="single")
                    ' - )
        fi
    done

    echo "${WEAVE_YAML}" | kubectl apply -f -
    echo "Waiting for weave-net pods to be ready..."
    kubectl wait --for=condition=Ready pods -l name=weave-net -n kube-system --timeout="${timeout}"
    echo "Waiting for core-dns deployment to be ready..."
    kubectl -n kube-system rollout status deploy/coredns --timeout="${timeout}"
}

function deploy_ovn_cni(){
    echo "OVN CNI deployed."
}

function deploy_kind_ovn(){
    local OVN_SRC_IMAGE="ghcr.io/ovn-org/ovn-kubernetes/ovn-kube-f:master"
    export K8s_VERSION="${k8s_version}"
    export NET_CIDR_IPV4="${cluster_CIDRs[${cluster}]}"
    export SVC_CIDR_IPV4="${service_CIDRs[${cluster}]}"
    export KIND_CLUSTER_NAME="${cluster}"

    export OVN_IMAGE="localhost:5000/ovn-daemonset-f:latest"
    docker pull "${OVN_SRC_IMAGE}"
    docker tag "${OVN_SRC_IMAGE}" "${OVN_IMAGE}"
    docker push "${OVN_IMAGE}"

    ( ./ovn-kubernetes/contrib/kind.sh -ov "$OVN_IMAGE" -cn "${KIND_CLUSTER_NAME}" -ric -lr -dd "${KIND_CLUSTER_NAME}.local"; ) &
    if ! wait $! ; then
        echo "Failed to install kind with OVN"
        kind delete cluster --name="${cluster}"
        return 1
    fi

    ( deploy_cluster_capabilities; ) &
    if ! wait $! ; then
        echo "Failed to deploy cluster capabilities, removing the cluster"
        kind delete cluster --name="${cluster}"
        return 1
    fi
}

function run_local_registry() {
    # Run a local registry to avoid loading images manually to kind
    if registry_running; then
        echo "Local registry $KIND_REGISTRY already running."
    else
        echo "Deploying local registry $KIND_REGISTRY to serve images centrally."
        declare -a volume_flags
        if [[ $registry_inmemory = true ]]; then
            volume_dir="/var/lib/registry"
            volume_flags+=(-v "/dev/shm/${KIND_REGISTRY}:${volume_dir}")
            selinuxenabled && volume_flag="${volume_flag}:z" 2>/dev/null
        fi
        docker run -d "${volume_flags[@]}" -p 127.0.0.1:5000:5000 --restart=always --name $KIND_REGISTRY registry:2
        docker network connect kind $KIND_REGISTRY || true

        # If the registry is stored in memory and the local volume mount directory is empty,
        # then try to push any images with "localhost:5000". The volume mount directory is
        # probably empty due to a host reboot.
        if [[ $registry_inmemory = true ]] && [[ ! "$(docker exec -e tmp_dir=${volume_dir} -it $KIND_REGISTRY /bin/sh -c 'ls -A ${tmp_dir} 2>/dev/null')" ]]; then
            echo "Push images to local registry: $KIND_REGISTRY"
            readarray -t local_image_list < <(docker images | awk -F' ' '/localhost:5000/ {print $1":"$2}')
            for image in "${local_image_list[@]}"
            do
                if ! docker push "${image}"; then
                    echo "'docker push ${image}' failed."
                fi
            done
        fi
    fi
}

function deploy_olm() {
    echo "Applying OLM CRDs..."
    kubectl apply -f "https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${olm_version}/crds.yaml" --validate=false
    kubectl wait --for=condition=Established -f "https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${olm_version}/crds.yaml"
    echo "Applying OLM resources..."
    kubectl apply -f "https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${olm_version}/olm.yaml"

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
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/bundle.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/serviceaccount.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/clusterrole.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/clusterrolebinding.yaml"
    kubectl apply -f "${SCRIPTS_DIR}/resources/prometheus/prometheus.yaml"
}

function deploy_cluster_capabilities() {
    deploy_cni
    [[ $olm != "true" ]] || deploy_olm
    [[ $prometheus != "true" ]] || deploy_prometheus
}

function warn_inotify() {
    if [[ "$(cat /proc/sys/fs/inotify/max_user_instances)" -lt 512 ]]; then
        echo "Your inotify settings are lower than our recommendation."
        echo "This may cause failures in large deployments, but we don't know if it caused this failure."
        echo "You may need to increase your inotify settings (currently $(cat /proc/sys/fs/inotify/max_user_watches) and $(cat /proc/sys/fs/inotify/max_user_instances)):"
        echo sudo sysctl fs.inotify.max_user_watches=524288
        echo sudo sysctl fs.inotify.max_user_instances=512
        echo 'See https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files'
    fi
}

# If any of the clusters use OVN-K as the CNI then clone the
# ovn-kubernetes repo from master in order to access the required
# kind scripts, and manifest generation templates.
function download_ovnk() {
    if [[ ${cluster_cni[*]} != *"ovn"* ]]; then
        return
    fi

    echo "Cloning ovn-kubernetes from source"
    git clone https://github.com/ovn-org/ovn-kubernetes.git \
	|| { git -C ovn-kubernetes fetch && git -C ovn-kubernetes reset --hard origin/master; }
}

### Main ###

rm -rf "${KUBECONFIGS_DIR}"
mkdir -p "${KUBECONFIGS_DIR}"

download_kind
load_settings
download_ovnk
run_local_registry
declare_cidrs

# Run in subshell to check response, otherwise `set -e` is not honored
( run_all_clusters with_retries 3 create_kind_cluster; ) &
if ! wait $!; then
    echo "Failed to create kind clusters."
    warn_inotify
    exit 1
fi

print_clusters_message
