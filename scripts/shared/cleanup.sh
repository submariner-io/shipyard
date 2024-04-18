#!/usr/bin/env bash

set -em

source "${SCRIPTS_DIR}/lib/utils"
print_env PLUGIN
source "${SCRIPTS_DIR}/lib/debug_functions"

# Source plugin if the path is passed via plugin argument and the file exists
# shellcheck disable=SC1090
[[ -n "${PLUGIN}" ]] && [[ -f "${PLUGIN}" ]] && source "${PLUGIN}"

### Main ###

load_library cleanup PROVIDER
run_if_defined pre_cleanup
provider_initialize

run_all_clusters provider_delete_cluster
run_if_defined provider_delete_load_balancer

# Remove any files inside the output directory, but not any directories as a provider might be using them.
\rm -f "${OUTPUT_DIR:?}"/* 2> /dev/null || true

provider_finalize
run_if_defined post_cleanup
