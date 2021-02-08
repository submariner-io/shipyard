#!/usr/bin/env bash

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils

### Functions ###

function print_section() {
    echo "===================================================================="
    echo "::endgroup::"
    echo "::group::$*"
    echo "======================= $* ======================="
}

function print_pods_logs() {
    local namespace=$1
    local selector=$2

    print_section "** Pods logs for NS $namespace using selector '$selector' **"
    for pod in $(kubectl get pods --selector="$selector" -n "$namespace" -o jsonpath='{.items[*].metadata.name}'); do
        print_section "*** $pod ***"
        kubectl -n $namespace logs $pod
    done
}

function post_analyze() {
    print_section "* Overview of all resources in $cluster *"
    kubectl api-resources --verbs=list -o name | xargs -n 1 kubectl get --show-kind -o wide --ignore-not-found

    print_section "* Pods not running in $cluster *"
    for pod in $(kubectl get pods -A | tail -n +2 | grep -v Running | sed 's/  */;/g'); do
        ns=$(echo $pod | cut -f1 -d';')
        name=$(echo $pod | cut -f2 -d';')
        print_section "** NS: $ns; Pod: $name **"
        kubectl -n $ns describe pod $name
        kubectl -n $ns logs $name
    done

    # TODO (revisit): The following is added to debug intermittent globalnet failures.
    print_section "* Globalnet related logs in $cluster *"
    print_pods_logs "kube-system" "k8s-app=kube-proxy"
    print_pods_logs "submariner-operator"

    print_section "* Dump 'subctl show all' *"
    subctl show all
    return 0
}

### Main ###

declare_kubeconfig
for cluster in $(kind get clusters); do
    post_analyze
done
