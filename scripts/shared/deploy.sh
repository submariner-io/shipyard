#!/usr/bin/env bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'cluster_settings' '' "Settings file to customize cluster deployments"
DEFINE_string 'deploytool' 'operator' 'Tool to use for deploying (operator/helm)'
DEFINE_string 'globalnet' 'false' "Deploy with operlapping CIDRs (set to 'true' to enable)"
DEFINE_string 'cable_driver' '' "Cable driver implementation"

FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

globalnet="${FLAGS_globalnet}"
deploytool="${FLAGS_deploytool}"
cluster_settings="${FLAGS_cluster_settings}"
cable_driver="${FLAGS_cable_driver}"

echo "Running with: globalnet=${globalnet}, deploytool=${deploytool}, cluster_settings=${cluster_settings}, cable_driver=${cable_driver}"

set -em

source ${SCRIPTS_DIR}/lib/debug_functions
source ${SCRIPTS_DIR}/lib/version
source ${SCRIPTS_DIR}/lib/utils
source ${SCRIPTS_DIR}/lib/deploy_funcs

# Always source the shared cluster settings, to set defaults in case something wasn't set in the provided settings
source "${SCRIPTS_DIR}/lib/cluster_settings"
[[ -z "${cluster_settings}" ]] || source ${cluster_settings}

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

with_context cluster2 connectivity_tests

