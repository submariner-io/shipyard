#!/usr/bin/env bash

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/utils
source ${SCRIPTS_DIR}/lib/cluster_settings

### Functions ###

function post_analyze() {
    echo "======================= Post mortem $cluster ======================="
    kubectl get all -A
    for pod in $(kubectl get pods -A | tail -n +2 | grep -v Running | sed 's/  */;/g'); do
        ns=$(echo $pod | cut -f1 -d';')
        name=$(echo $pod | cut -f2 -d';')
        echo "======================= $name - $ns ============================"
        kubectl -n $ns describe pod $name
        kubectl -n $ns logs $name
        echo "===================== END $name - $ns =========================="
    done
    echo "===================== END Post mortem $cluster ====================="
}

### Main ###

declare_kubeconfig
run_sequential "${clusters[*]}" post_analyze
