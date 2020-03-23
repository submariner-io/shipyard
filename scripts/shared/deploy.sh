#!/usr/bin/env bash
set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/version
source ${SCRIPTS_DIR}/lib/utils

### Functions ###

function import_images() {
    docker tag quay.io/submariner/submariner:$VERSION localhost:5000/submariner:local
    docker tag quay.io/submariner/submariner-route-agent:$VERSION localhost:5000/submariner-route-agent:local
    if [[ $globalnet = "true" ]]; then
        docker tag quay.io/submariner/submariner-globalnet:$VERSION localhost:5000/submariner-globalnet:local
    fi

    docker push localhost:5000/submariner:local
    docker push localhost:5000/submariner-route-agent:local
    if [[ $globalnet = "true" ]]; then
        docker push localhost:5000/submariner-globalnet:local
    fi
}

function get_globalip() {
    svcname=$1
    context=$2
    # It takes a while for globalIp annotation to show up on a service
    for i in {0..30}
    do
        gip=$(kubectl --context=$context get svc $svcname -o jsonpath='{.metadata.annotations.submariner\.io/globalIp}')
        if [[ -n ${gip} ]]; then
          echo $gip
          return
        fi
        sleep 1
    done
    echo "Max attempts reached, failed to get globalIp!"
    exit 1
}

function test_connection() {
    if [[ $globalnet = "true" ]]; then
        nginx_svc_ip_cluster3=$(get_globalip nginx-demo cluster3)
    else
        nginx_svc_ip_cluster3=$(kubectl --context=cluster3 get svc -l app=nginx-demo | awk 'FNR == 2 {print $3}')
    fi

    if [[ -z "$nginx_svc_ip_cluster3" ]]; then
        echo "Failed to get nginx-demo IP"
        exit 1
    fi
    netshoot_pod=$(kubectl --context=cluster2 get pods -l app=netshoot | awk 'FNR == 2 {print $1}')

    echo "Testing connectivity between clusters - $netshoot_pod cluster2 --> $nginx_svc_ip_cluster3 nginx service cluster3"

    attempt_counter=0
    max_attempts=5
    until $(kubectl --context=cluster2 exec ${netshoot_pod} -- curl --output /dev/null -m 30 --silent --head --fail ${nginx_svc_ip_cluster3}); do
        if [[ ${attempt_counter} -eq ${max_attempts} ]];then
          echo "Max attempts reached, connection test failed!"
          exit 1
        fi
        attempt_counter=$(($attempt_counter+1))
    done
    echo "Connection test was successful!"
}

function add_subm_gateway_label() {
    context=$1
    kubectl --context=$context label node $context-worker "submariner.io/gateway=true" --overwrite
}

function del_subm_gateway_label() {
    context=$1
    kubectl --context=$context label node $context-worker "submariner.io/gateway-" --overwrite
}

function prepare_cluster() {
    for app in submariner-engine submariner-routeagent submariner-globalnet; do
        if kubectl wait --for=condition=Ready pods -l app=$app -n $SUBM_NS --timeout=60s > /dev/null 2>&1; then
            echo "Removing $app pods..."
            kubectl delete pods -n $SUBM_NS -l app=$app
        fi
    done
    add_subm_gateway_label $cluster
}

function deploy_resource() {
    cluster=$1
    resource_file=$2
    resource_name=$(basename "$2" ".yaml")
    kubectl apply -f ${resource_file}
    echo "Waiting for ${resource_name} pods to be ready."
    kubectl rollout status deploy/${resource_name} --timeout=120s
    unset cluster
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

load_deploytool
import_images

# Install Helm/Operator deploy tool prerequisites
deploytool_prereqs

run_parallel "{1..3}" prepare_cluster

setup_broker cluster1
install_subm_all_clusters

deploytool_postreqs

deploy_resource "cluster2" "${RESOURCES_DIR}/netshoot.yaml"
deploy_resource "cluster3" "${RESOURCES_DIR}/nginx-demo.yaml"

test_connection

