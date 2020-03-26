#!/usr/bin/env bash

## Process command line flags ##

source /usr/share/shflags/shflags
DEFINE_string 'deploytool' 'operator' 'Tool to use for deploying (operator/helm)'
DEFINE_string 'globalnet' 'false' "Deploy with operlapping CIDRs (set to 'true' to enable)"
FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

globalnet="${FLAGS_globalnet}"
deploytool="${FLAGS_deploytool}"
echo "Running with: globalnet=${globalnet}, deploytool=${deploytool}"

set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/version
source ${SCRIPTS_DIR}/lib/utils

### Functions ###

function import_image() {
    local orig_image="$1:$VERSION"
    local local_image="localhost:5000/${1##*/}:local"
    if ! docker tag "${orig_image}" "${local_image}"; then
        # The project doesn't build this image, pull it
        docker pull "${1}:latest"
        docker tag "${1}:latest" "${orig_image}"
        docker tag "${orig_image}" "${local_image}"
    fi
    docker push ${local_image}
}

function get_globalip() {
    local svc_name=$1

    # It takes a while for globalIp annotation to show up on a service
    for i in {0..30}; do
        local gip=$(kubectl get svc $svc_name -o jsonpath='{.metadata.annotations.submariner\.io/globalIp}')
        if [[ -n ${gip} ]]; then
            echo $gip
            return
        fi
        sleep 1
    done

    echo "Max attempts reached, failed to get globalIp!"
    exit 1
}

function get_svc_ip() {
    local svc_name=$1

    if [[ $globalnet = "true" ]]; then
        get_globalip ${svc_name}
    else
        kubectl --context=$cluster get svc -l app=${svc_name} | awk 'FNR == 2 {print $3}'
    fi
}

function test_connection() {
    local nginx_svc_ip=$(with_context cluster3 get_svc_ip nginx-demo)
    if [[ -z "$nginx_svc_ip" ]]; then
        echo "Failed to get nginx-demo IP"
        exit 1
    fi

    local netshoot_pod=$(kubectl get pods -l app=netshoot | awk 'FNR == 2 {print $1}')

    echo "Testing connectivity between clusters - $netshoot_pod cluster2 --> $nginx_svc_ip_cluster3 nginx service cluster3"

    for i in {1..5}; do
        if kubectl exec ${netshoot_pod} -- curl --output /dev/null -m 30 --silent --head --fail ${nginx_svc_ip}; then
            echo "Connection test was successful!"
            return
        fi
    done

    echo "Max attempts reached, connection test failed!"
    exit 1
}

function add_subm_gateway_label() {
    kubectl label node $cluster-worker "submariner.io/gateway=true" --overwrite
}

function del_subm_gateway_label() {
    kubectl label node $cluster-worker "submariner.io/gateway-" --overwrite
}

function prepare_cluster() {
    for app in submariner-engine submariner-routeagent submariner-globalnet; do
        if kubectl wait --for=condition=Ready pods -l app=$app -n $SUBM_NS --timeout=60s > /dev/null 2>&1; then
            echo "Removing $app pods..."
            kubectl delete pods -n $SUBM_NS -l app=$app
        fi
    done
    add_subm_gateway_label
}

function deploy_resource() {
    local resource_file=$1
    local resource_name=$(basename "$resource_file" ".yaml")
    kubectl apply -f ${resource_file}
    echo "Waiting for ${resource_name} pods to be ready."
    kubectl rollout status deploy/${resource_name} --timeout=120s
}

function load_deploytool() {
    local deploy_lib=${SCRIPTS_DIR}/lib/deploy_${deploytool}
    if [[ ! -f $deploy_lib ]]; then
        echo "Unknown deploy method: ${deploytool}"
        exit 1
    fi

    echo "Will deploy submariner using ${deploytool}"
    . $deploy_lib
}


### Main ###

declare_cidrs
declare_kubeconfig

import_image quay.io/submariner/submariner
import_image quay.io/submariner/submariner-route-agent
[[ $globalnet != "true" ]] || import_image quay.io/submariner/submariner-globalnet

load_deploytool
deploytool_prereqs

run_parallel "{1..3}" prepare_cluster

with_context cluster1 setup_broker
install_subm_all_clusters

deploytool_postreqs

with_context cluster2 deploy_resource "${RESOURCES_DIR}/netshoot.yaml"
with_context cluster3 deploy_resource "${RESOURCES_DIR}/nginx-demo.yaml"

with_context cluster2 test_connection

