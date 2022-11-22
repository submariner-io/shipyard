#!/usr/bin/env bash

source "${SCRIPTS_DIR}/lib/debug_functions"
source "${SCRIPTS_DIR}/lib/utils"

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
        if [ "$(kubectl get pods -n "$namespace" "$pod" -o jsonpath='{.status.containerStatuses[*].ready}')" != true ]; then
            print_section "*** $pod (terminated) ***"
            kubectl -n "$namespace" logs -p "$pod"
        else
            print_section "*** $pod ***"
            kubectl -n "$namespace" logs "$pod"
        fi
    done
}

function post_analyze() {
    print_section "* Kubernetes client and server versions in $cluster *"
    kubectl version || true

    print_section "* Overview of all resources in $cluster *"
    for resource in $(kubectl api-resources --verbs=list -o name); do
        print_section "** Resource: $resource"
        kubectl get --all-namespaces --show-kind -o wide --ignore-not-found "$resource"
    done

    print_section "* Details of pods with statuses other than Running in $cluster *"
    for pod in $(kubectl get pods -A | tail -n +2 | grep -v Running | sed 's/  */;/g'); do
        ns=$(echo "$pod" | cut -f1 -d';')
        name=$(echo "$pod" | cut -f2 -d';')
        print_section "** NS: $ns; Pod: $name **"
        kubectl -n "$ns" describe pod "$name"
        kubectl -n "$ns" logs "$name"
    done

    print_section "* Kube-controller-manager pod logs for $cluster *"
    print_pods_logs "kube-system" "component=kube-controller-manager"

    print_section "* Submariner-operator pod logs for $cluster *"
    print_pods_logs "submariner-operator"

    print_section "* Output of 'subctl show all' in $cluster *"
    subctl show all --context "$cluster"

    print_section "* Output of 'subctl diagnose all' in $cluster *"
    subctl diagnose all --context "$cluster"

    return 0
}

### Main ###

declare_kubeconfig
"${SCRIPTS_DIR}/get-subctl.sh"
for cluster in $(kind get clusters); do
    post_analyze
done
