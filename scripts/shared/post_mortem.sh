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

function post_analyze() {
    print_section "** Pods not running in $cluster **"
    kubectl get all --all-namespaces
    for pod in $(kubectl get pods -A | tail -n +2 | grep -v Running | sed 's/  */;/g'); do
        ns=$(echo $pod | cut -f1 -d';')
        name=$(echo $pod | cut -f2 -d';')
        print_section "NS: $ns; Pod: $name"
        kubectl -n $ns describe pod $name
        kubectl -n $ns logs $name
    done

    # TODO (revisit): The following is added to debug intermittent globalnet failures.
    print_section "** Globalnet related logs in $cluster **"
    namespace="kube-system"
    for pod in $(kubectl get pods --selector=k8s-app=kube-proxy -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"

    namespace="submariner-operator"
    for pod in $(kubectl get pods --selector=name=submariner-operator -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"

    namespace="submariner-operator"
    for pod in $(kubectl get pods --selector=app=submariner-globalnet -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"

    for pod in $(kubectl get pods --selector=app=submariner-engine -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"

    for pod in $(kubectl get pods --selector=app=submariner-routeagent -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"

    for pod in $(kubectl get pods --selector=app=submariner-lighthouse-agent -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"

    for pod in $(kubectl get pods --selector=app=submariner-lighthouse-coredns -n $namespace -o jsonpath='{.items[*].metadata.name}'); do
        echo "+++++++++++++++++++++: Logs for Pod $pod in namespace $namespace :++++++++++++++++++++++"
        kubectl -n $namespace logs $pod
    done

    echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"
    subctl show all
    return 0
}

### Main ###

declare_kubeconfig
for cluster in $(kind get clusters); do
    post_analyze
done
