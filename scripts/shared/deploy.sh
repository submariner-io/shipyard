#!/usr/bin/env bash

set -em

source "${SCRIPTS_DIR}/lib/utils"
print_env CABLE_DRIVER DEPLOYTOOL GLOBALNET IMAGE_TAG LIGHTHOUSE PARALLEL PLUGIN PRELOAD_IMAGES SETTINGS TIMEOUT
source "${SCRIPTS_DIR}/lib/debug_functions"
source "${SCRIPTS_DIR}/lib/deploy_funcs"

# Source plugin if the path is passed via plugin argument and the file exists
# shellcheck disable=SC1090
[[ -n "${PLUGIN}" ]] && [[ -f "${PLUGIN}" ]] && source "${PLUGIN}"

### Constants ###
# These are used in other scripts
# shellcheck disable=SC2034
readonly CE_IPSEC_IKEPORT=500
# shellcheck disable=SC2034
readonly CE_IPSEC_NATTPORT=4500
# shellcheck disable=SC2034
readonly SUBM_CS="submariner-catalog-source"
# shellcheck disable=SC2034
readonly SUBM_INDEX_IMG="${SUBM_IMAGE_REPO}/submariner-operator-index:${SUBM_IMAGE_TAG}"
# shellcheck disable=SC2034
readonly BROKER_NAMESPACE="submariner-k8s-broker"
# shellcheck disable=SC2034
readonly BROKER_CLIENT_SA="submariner-k8s-broker-client"
readonly MARKETPLACE_NAMESPACE="olm"
IPSEC_PSK="$(dd if=/dev/urandom count=64 bs=8 | LC_CTYPE=C tr -dc 'a-zA-Z0-9' | fold -w 64 | head -n 1)"
# shellcheck disable=SC2034
readonly IPSEC_PSK

### Common functions ###

# Create a CatalogSource
# 1st argument is the catalogsource name
# 2nd argument is the namespace
# 3rd argument is index image url
function create_catalog_source() {
  local cs=$1
  local ns=$2
  # shellcheck disable=SC2034 # this variable is used elsewhere
  local iib=$3  # Index Image Build
  echo "[INFO](${cluster}) Create the catalog source ${cs}..."

  kubectl delete catalogsource/operatorhubio-catalog -n "${MARKETPLACE_NAMESPACE}" --wait --ignore-not-found
  kubectl delete catalogsource/"${cs}" -n "${MARKETPLACE_NAMESPACE}" --wait --ignore-not-found

  # Create the CatalogSource
  render_template "${RESOURCES_DIR}"/common/catalogSource.yaml | kubectl apply -f -

  # Wait for the CatalogSource readiness
  if ! with_retries 60 kubectl get catalogsource -n "${MARKETPLACE_NAMESPACE}" "${cs}" -o jsonpath='{.status.connectionState.lastObservedState}'; then
    exit_error "[ERROR](${cluster}) CatalogSource ${cs} is not ready."
  fi

  echo "[INFO](${cluster}) Catalog source ${cs} created"
}

# Create an OperatorGroup
# 1st argument is the operatorgroup name
# 2nd argument is the target namespace
function create_operator_group() {
  local og=$1
  local ns=$2

  echo "[INFO](${cluster}) Create the OperatorGroup ${og}..."
  # Create the OperatorGroup
  render_template "${RESOURCES_DIR}"/common/operatorGroup.yaml | kubectl apply -f -
}

# Install the bundle, subscript and approve.
# 1st argument is the subscription name
# 2nd argument is the target catalogsource name
# 3nd argument is the source namespace
# 4th argument is the bundle name
function install_bundle() {
  local sub=$1
  local cs=$2
  local ns=$3
  local bundle=$4
  local installPlan

  # Delete previous CatalogSource and Subscription
  kubectl delete sub/"${sub}" -n "${ns}" --wait --ignore-not-found

  # Create the Subscription (Approval should be Manual not Automatic in order to pin the bundle version)
  echo "[INFO](${cluster}) Install the bundle ${bundle} ..."
  render_template "${RESOURCES_DIR}"/common/subscription.yaml | kubectl apply -f -

  # Manual Approve
  echo "[INFO](${cluster}) Approve the InstallPlan..."
  kubectl wait --for condition=InstallPlanPending --timeout=5m -n "${ns}" subs/"${sub}" || exit_error "[ERROR](${cluster}) InstallPlan not found."
  installPlan=$(kubectl get subscriptions.operators.coreos.com "${sub}" -n "${ns}" -o jsonpath='{.status.installPlanRef.name}')
  if [ -n "${installPlan}" ]; then
    kubectl patch installplan -n "${ns}" "${installPlan}" -p '{"spec":{"approved":true}}' --type merge
  fi

  echo "[INFO](${cluster}) Bundle ${bundle} installed"
}

### Main ###

load_settings
declare_cidrs
declare_kubeconfig

# Always import nettest image on kind, to be able to test connectivity and other things
[[ "${PROVIDER}" != 'kind' ]] || import_image "${REPO}/nettest"

# Always get subctl since we're using moving versions, and having it in the image results in a stale cached one
"${SCRIPTS_DIR}/get-subctl.sh"

load_library deploy DEPLOYTOOL
deploytool_prereqs

run_if_defined pre_deploy

with_context "$broker" setup_broker
install_subm_all_clusters

if [ "${#cluster_subm[@]}" -gt 1 ]; then
    # shellcheck disable=2206 # the array keys don't have spaces
    cls=(${!cluster_subm[@]})
    with_context "${cls[0]}" with_retries 30 verify_gw_status
    with_context "${cls[0]}" connectivity_tests "${cls[1]}"
else
    echo "Not executing connectivity tests - requires at least two clusters with submariner installed"
fi

run_if_defined post_deploy

# Check that the deployed images match those we built (if any)
image_mismatch=false
for image in package/.image.*; do
    expected="$(docker image inspect "$(cat "$image")" | jq -r '.[0].RepoDigests[0]')"
    image="${image#package/.image.}"
    for deployed in $(kubectl get pods -A -o json | jq -r '.items[].status.containerStatuses[].imageID' | grep "$image"); do
        if [ "$deployed" != "$expected" ]; then
            printf "Image %s is deployed with %s, expected %s\n" "$image" "$deployed" "$expected"
            image_mismatch=true
        else
            printf "Successfully checked image %s, deployed with %s\n" "$image" "$deployed"
        fi
    done
done
if [ "$image_mismatch" = true ]; then
    kubectl get pods -A -o json
    exit 1
fi

# Print installed versions for manual validation of CI
subctl show versions
print_clusters_message
