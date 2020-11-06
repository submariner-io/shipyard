#!/usr/bin/env bash

## Process command line flags ##

source ${SCRIPTS_DIR}/lib/shflags
DEFINE_string 'cluster_settings' '' "Settings file to customize cluster deployments"
DEFINE_string 'deploytool' 'operator' 'Tool to use for deploying (operator/helm)'
DEFINE_string 'deploytool_broker_args' '' 'Any extra arguments to pass to the deploytool when deploying the broker'
DEFINE_string 'deploytool_submariner_args' '' 'Any extra arguments to pass to the deploytool when deploying submariner'
DEFINE_boolean 'globalnet' false "Deploy with operlapping CIDRs (set to 'true' to enable)"
DEFINE_string 'timeout' '5m' "Timeout flag to pass to kubectl when waiting (e.g. 30s)"
DEFINE_string 'image_tag' 'local' "Tag to use for the images"
DEFINE_string 'cable_driver' 'libreswan' "Tunneling method for connections between clusters (libreswan, wireguard requires kernel module on host)"

FLAGS "$@" || exit $?
eval set -- "${FLAGS_ARGV}"

[[ "${FLAGS_globalnet}" = "${FLAGS_TRUE}" ]] && globalnet=true || globalnet=false
deploytool="${FLAGS_deploytool}"
deploytool_broker_args="${FLAGS_deploytool_broker_args}"
deploytool_submariner_args="${FLAGS_deploytool_submariner_args}"
cluster_settings="${FLAGS_cluster_settings}"
timeout="${FLAGS_timeout}"
image_tag="${FLAGS_image_tag}"
cable_driver="${FLAGS_cable_driver}"

echo "Running with: globalnet=${globalnet@Q}, deploytool=${deploytool@Q}, deploytool_broker_args=${deploytool_broker_args@Q}, deploytool_submariner_args=${deploytool_submariner_args@Q}, cluster_settings=${cluster_settings@Q}, timeout=${timeout}, image_tag=${image_tag}, cable_driver=${cable_driver}"

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

# Always get subctl since we're using moving versions, and having it in the image results in a stale cached one
bash -c "curl -Ls https://get.submariner.io | VERSION=${CUTTING_EDGE} DESTDIR=/go/bin bash"

# nettest is always referred to using :local
import_image quay.io/submariner/nettest
import_image quay.io/submariner/submariner-operator ${image_tag}
import_image quay.io/submariner/submariner ${image_tag}
import_image quay.io/submariner/submariner-route-agent ${image_tag}
[[ $globalnet != "true" ]] || import_image quay.io/submariner/submariner-globalnet ${image_tag}
[[ "${cluster_cni[$cluster]}" != "ovn" ]] || import_image quay.io/submariner/submariner-networkplugin-syncer ${image_tag}

load_deploytool $deploytool
deploytool_prereqs

run_subm_clusters prepare_cluster

with_context $broker setup_broker
install_subm_all_clusters

if [ "${#cluster_subm[@]}" -gt 1 ]; then
    cls=(${!cluster_subm[@]})
    with_context "${cls[0]}" with_retries 30 verify_gw_status
    with_context "${cls[0]}" connectivity_tests "${cls[1]}"
else
    echo "Not executing connectivity tests - requires at least two clusters with submariner installed"
fi

