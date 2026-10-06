#!/usr/bin/env bash

# Check image tags and rollouts, independently of commit-derived binary versions.
set -e -o pipefail

expected=${1:?Expected component image tag}
expected=${expected#v}
context_args=()
[[ -z "${2:-}" ]] || context_args=(--context "$2")
timeout_seconds=${VERSION_CHECK_TIMEOUT:-300}
interval_seconds=${VERSION_CHECK_INTERVAL:-5}
[[ "$timeout_seconds" =~ ^(0|[1-9][0-9]*)$ && "$interval_seconds" =~ ^(0|[1-9][0-9]*)$ ]] || {
    echo 'Version check timeout and interval must be non-negative integer seconds' >&2
    exit 1
}
deadline=$((SECONDS + timeout_seconds))
waiting=false

installed=$(kubectl "${context_args[@]}" get submariners,servicediscoveries --namespace submariner-operator --output json)
required=$(jq -er '
    [.items[] | select(.kind == "Submariner" or .kind == "ServiceDiscovery")] as $components |
    if ($components | length) == 0 then error("No installed Submariner or ServiceDiscovery resources found")
    else ["deployment/submariner-operator"] + [$components[] |
        if .kind == "Submariner" then
            "daemonset/submariner-gateway", "daemonset/submariner-routeagent",
            (if (.spec.globalCIDR // "") != "" then "daemonset/submariner-globalnet" else empty end)
        else "deployment/submariner-lighthouse-agent", "deployment/submariner-lighthouse-coredns" end] | unique end' \
    <<< "$installed")

check_images() {
    jq -er --arg expected "$expected" --argjson required "$required" '
        [.items[] | select(.metadata.name == "submariner-operator" or
            .metadata.name == "submariner-gateway" or .metadata.name == "submariner-routeagent" or
            .metadata.name == "submariner-networkplugin-syncer" or
            .metadata.name == "submariner-globalnet" or .metadata.name == "submariner-lighthouse-agent" or
            .metadata.name == "submariner-lighthouse-coredns") |
            {resource: ((.kind | ascii_downcase) + "/" + .metadata.name), image: .spec.template.spec.containers[0].image}] as $deployed |
        ($required - ($deployed | map(.resource))) as $missing |
        if ($missing | length) != 0 then error("Missing Submariner workloads: \($missing | join(", "))")
        else $deployed[] | if (.image | endswith(":" + $expected)) then
            [.resource, .image] | @tsv
        else error("Expected \(.resource) image tag \($expected), got \(.image)") end end'
}

while :; do
    resources=$(kubectl "${context_args[@]}" get deployments,daemonsets --namespace submariner-operator --output json)
    if result=$(check_images <<< "$resources" 2>&1); then
        break
    fi
    if [[ "$SECONDS" -ge "$deadline" ]]; then
        echo "$result" >&2
        exit 1
    fi
    if [[ "$waiting" == false ]]; then
        echo "Waiting for Submariner image tags to become $expected" >&2
        waiting=true
    fi
    sleep "$interval_seconds"
done

while IFS=$'\t' read -r resource image; do
    remaining=$((deadline - SECONDS))
    [[ "$remaining" -gt 0 ]] || remaining=1
    kubectl "${context_args[@]}" rollout status "$resource" --namespace submariner-operator --timeout="${remaining}s"
    echo "Verified $resource: $image"
done <<< "$result"

# A controller may have changed a template while rollout status followed its latest revision.
resources=$(kubectl "${context_args[@]}" get deployments,daemonsets --namespace submariner-operator --output json)
check_images <<< "$resources" > /dev/null
