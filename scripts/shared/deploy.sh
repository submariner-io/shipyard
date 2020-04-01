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
source ${SCRIPTS_DIR}/lib/deploy_funcs

### Main ###

declare_cidrs
declare_kubeconfig

import_image quay.io/submariner/submariner
import_image quay.io/submariner/submariner-route-agent
[[ $globalnet != "true" ]] || import_image quay.io/submariner/submariner-globalnet

load_deploytool $deploytool
deploytool_prereqs

run_parallel "{1..3}" prepare_cluster "$SUBM_NS"

with_context cluster1 setup_broker
install_subm_all_clusters

deploytool_postreqs

with_context cluster2 deploy_resource "${RESOURCES_DIR}/netshoot.yaml"
with_context cluster3 deploy_resource "${RESOURCES_DIR}/nginx-demo.yaml"

with_context cluster2 test_connection

