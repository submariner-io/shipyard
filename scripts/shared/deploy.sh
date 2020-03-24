#!/usr/bin/env bash
set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/version
source ${SCRIPTS_DIR}/lib/utils

### Functions ###

function import_image() {
    local orig_image="$1:$VERSION"
    local local_image="localhost:5000/${1##*/}:local"
    docker tag ${orig_image} ${local_image}
    docker push ${local_image}
}

function get_globalip() {
    local svc_name=$1

    # It takes a while for globalIp annotation to show up on a service
    for i in {0..30}; do
        local gip=$(kubectl get svc $svc_name -o jsonpath='{.metadata.annotations.submariner\.io/globalIp}')
        [[ -n ${gip} ]] || { sleep 1; continue; }
        echo $gip
        return
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
    [[ -n "$nginx_svc_ip" ]] || { echo "Failed to get nginx-demo IP"; exit 1; }

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
    [[ -f $deploy_lib ]] || { echo "Unknown deploy method: ${deploytool}"; exit 1; }

    echo "Will deploy submariner using ${deploytool}"
    . $deploy_lib
}


### Main ###

LONGOPTS=globalnet:,deploytool:
# Only accept longopts, but must pass null shortopts or first param after "--" will be incorrectly used
SHORTOPTS=""
! PARSED=$(getopt --options=$SHORTOPTS --longoptions=$LONGOPTS --name "$0" -- "$@")
eval set -- "$PARSED"

while true; do
    case "$1" in
        --globalnet)
            globalnet="$2"
            ;;
        --deploytool)
            deploytool="$2"
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

echo "Running with: globalnet=${globalnet}, deploytool=${deploytool}"

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

