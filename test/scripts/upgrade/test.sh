#!/usr/bin/env bash

# Offline regression tests: no release downloads or cluster deployments.
set -e -o pipefail

repo=$(cd "$(dirname "$0")/../../.." && pwd)
scripts="$repo/scripts/shared"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/bin"
export FIXTURE="$fixture"
export VERSION_CHECK_TIMEOUT=0
export SUBCTL_IMAGE_VERSION=
export PATH="$fixture/bin:$PATH"

cat > "$fixture/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -e
[[ ! -f "$FIXTURE/api-error" ]] || exit 22
page=${!#}
page=${page##*page=}
cat "$FIXTURE/page-$page.json"
EOF
cat > "$fixture/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
[[ ! -f "$FIXTURE/kube-error" ]] || exit 1
if [[ " $* " == *' rollout status '* ]]; then
    [[ ! -f "$FIXTURE/rollout-error" ]] || exit 1
    [[ ! -f "$FIXTURE/change-after-rollout" ]] || touch "$FIXTURE/wrong-final-image"
    exit 0
fi
printf '%s\n' "$*" > "$FIXTURE/kubectl-args"
if [[ " $* " == *' get submariners,servicediscoveries '* ]]; then
    cat "$FIXTURE/installed.json"
elif [[ -f "$FIXTURE/pending-image" || -f "$FIXTURE/wrong-final-image" ]]; then
    rm -f "$FIXTURE/pending-image"
    jq '.items[0].spec.template.spec.containers[0].image |= sub(":0.22.2$"; ":0.24.2")' "$FIXTURE/components.json"
else
    cat "$FIXTURE/components.json"
fi
EOF
chmod +x "$fixture/bin/"*

expect_baseline() {
    local expected=$1 branch=$2 actual
    actual=$("$scripts/resolve-upgrade-baseline.sh" "$branch" "${3:-}")
    [[ "$actual" == "$expected" ]] || { echo "Expected $expected for $branch, got $actual" >&2; exit 1; }
}

expect_failure() {
    if "$@" > "$fixture/error.log" 2>&1; then
        echo "Expected failure: $*" >&2
        exit 1
    fi
}

cat > "$fixture/page-1.json" <<'EOF'
[
  {"tag_name":"v0.24.2","draft":false,"prerelease":false},
  {"tag_name":"v0.22.9","draft":false,"prerelease":false},
  {"tag_name":"v0.22.10","draft":false,"prerelease":false},
  {"tag_name":"v0.22.11","draft":true,"prerelease":false},
  {"tag_name":"v0.22.12","draft":false,"prerelease":true},
  {"tag_name":"v0.22.13-rc1","draft":false,"prerelease":false},
  {"tag_name":"v0.220.1","draft":true,"prerelease":false},
  {"tag_name":"subctl-release-0.22","draft":false,"prerelease":false}
]
EOF
expect_baseline v0.22.10 release-0.22
expect_baseline v0.24.2 devel
expect_baseline v0.24.2 release-0.25
expect_baseline v0.22.2 release-0.22 0.22.2
expect_baseline v0.22.2 custom-branch v0.22.2
expect_failure "$scripts/resolve-upgrade-baseline.sh" feature-unrecognized
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.30
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22 latest
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22 v0.22.2-rc1

# A similar minor prefix must not match; do not fall back to a newer stream.
echo '[{"tag_name":"v0.220.1","draft":false,"prerelease":false}]' > "$fixture/page-1.json"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22

# Follow full pages; a maintenance baseline may be on a later page.
jq -n '[range(100) | {tag_name:"v0.24.2",draft:false,prerelease:false}]' > "$fixture/page-1.json"
echo '[{"tag_name":"v0.22.2","draft":false,"prerelease":false}]' > "$fixture/page-2.json"
expect_baseline v0.22.2 release-0.22

touch "$fixture/api-error"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22
# An exact override must work without network access.
expect_baseline v0.22.2 release-0.22 v0.22.2
rm "$fixture/api-error"
echo '{"message":"API error"}' > "$fixture/page-1.json"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22
echo 'invalid JSON' > "$fixture/page-1.json"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22
echo '[{"tag_name":"v0.22.2","draft":false}]' > "$fixture/page-1.json"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22
echo '[]' > "$fixture/page-1.json"
expect_failure "$scripts/resolve-upgrade-baseline.sh" devel

# Exercise the actual baseline recipe: a target-phase image setting must not leak.
echo '[{"tag_name":"v0.22.2","draft":false,"prerelease":false}]' > "$fixture/page-1.json"
cat > "$fixture/bin/recursive-make" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$FIXTURE/baseline-make-args"
EOF
chmod +x "$fixture/bin/recursive-make"
make --no-print-directory --file "$repo/Makefile.inc" SHIPYARD_DIR="$repo" SCRIPTS_DIR="$scripts" \
    BASE_BRANCH=release-0.22 SUBCTL_IMAGE_VERSION=release-0.22 MAKE="$fixture/bin/recursive-make" deploy-latest
grep -qx 'SUBCTL_VERSION=v0.22.2' "$fixture/baseline-make-args"
grep -qx 'RELEASED_SUBCTL=true' "$fixture/baseline-make-args"
grep -qx 'SUBCTL_IMAGE_VERSION=' "$fixture/baseline-make-args"

# Assert image tags rather than embedded binary versions, and ignore test helpers.
echo '{"items":[{"kind":"Submariner","spec":{}}]}' > "$fixture/installed.json"
cat > "$fixture/components.json" <<'EOF'
{"items":[
  {"kind":"Deployment","metadata":{"name":"submariner-operator"},"spec":{"template":{"spec":{"containers":[{"image":"quay.io/submariner/submariner-operator:0.22.2"}]}}}},
  {"kind":"DaemonSet","metadata":{"name":"submariner-routeagent"},"spec":{"template":{"spec":{"containers":[{"image":"quay.io/submariner/submariner-route-agent:0.22.2"}]}}}},
  {"kind":"DaemonSet","metadata":{"name":"submariner-gateway"},"spec":{"template":{"spec":{"containers":[{"image":"quay.io/submariner/submariner-gateway:0.22.2"}]}}}},
  {"kind":"Deployment","metadata":{"name":"netshoot"},"spec":{"template":{"spec":{"containers":[{"image":"netshoot:latest"}]}}}}
]}
EOF
cp "$fixture/components.json" "$fixture/healthy-components.json"
"$scripts/check-deployed-version.sh" v0.22.2 cluster2
[[ $(cat "$fixture/kubectl-args") == '--context cluster2 get deployments,daemonsets --namespace submariner-operator --output json' ]]
touch "$fixture/pending-image"
VERSION_CHECK_TIMEOUT=2 VERSION_CHECK_INTERVAL=0 "$scripts/check-deployed-version.sh" 0.22.2
[[ ! -f "$fixture/pending-image" ]]
touch "$fixture/change-after-rollout"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
rm "$fixture/change-after-rollout" "$fixture/wrong-final-image"

# A partial list during reconciliation must not pass, even if all visible tags match.
jq 'del(.items[1])' "$fixture/healthy-components.json" > "$fixture/components.json"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
cp "$fixture/healthy-components.json" "$fixture/components.json"
echo '{"items":[{"kind":"Submariner","spec":{"globalCIDR":"242.0.0.0/24"}}]}' > "$fixture/installed.json"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
echo '{"items":[{"kind":"ServiceDiscovery","spec":{}}]}' > "$fixture/installed.json"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
# Service discovery alone legitimately has no gateway or route-agent.
jq '.items |= map(select(.metadata.name == "submariner-operator")) |
    .items += ["submariner-lighthouse-agent", "submariner-lighthouse-coredns"] | .items |= map(
        if type == "string" then {kind:"Deployment",metadata:{name:.},
            spec:{template:{spec:{containers:[{image:"quay.io/submariner/lighthouse:0.22.2"}]}}}} else . end)' \
    "$fixture/healthy-components.json" > "$fixture/components.json"
"$scripts/check-deployed-version.sh" 0.22.2
echo '{"items":[]}' > "$fixture/installed.json"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
echo '{"items":[{"kind":"Submariner","spec":{}}]}' > "$fixture/installed.json"
cp "$fixture/healthy-components.json" "$fixture/components.json"
touch "$fixture/rollout-error"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
rm "$fixture/rollout-error"
expect_failure "$scripts/check-deployed-version.sh" 0.24.2
jq '.items[1].spec.template.spec.containers[0].image = "quay.io/submariner/submariner-route-agent:0.24.2"' \
    "$fixture/components.json" > "$fixture/updated.json"
mv "$fixture/updated.json" "$fixture/components.json"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
echo '{"items":[]}' > "$fixture/components.json"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2
touch "$fixture/kube-error"
expect_failure "$scripts/check-deployed-version.sh" 0.22.2

# Exercise the real installer wrapper with a stub installer and an existing PR binary.
cat > "$fixture/bin/curl" <<'EOF'
#!/usr/bin/env bash
cat <<'INSTALLER'
set -e
mkdir -p "$DESTDIR"
VERSION=${STUB_SUBCTL_VERSION:-$VERSION}
cat > "$DESTDIR/subctl" <<BIN
#!/usr/bin/env bash
echo 'subctl version: $VERSION'
BIN
chmod +x "$DESTDIR/subctl"
INSTALLER
EOF
chmod +x "$fixture/bin/curl"
mkdir -p "$fixture/local-bin"
printf '#!/usr/bin/env bash\necho "subctl version: PR-build"\n' > "$fixture/local-bin/subctl"
chmod +x "$fixture/local-bin/subctl"
SUBCTL_INSTALL_DIR="$fixture/released/bin" SUBCTL_VERSION=v0.22.2 SCRIPTS_DIR="$scripts" \
    "$scripts/get-subctl.sh"
actual=$(PATH="$fixture/released/bin:$fixture/local-bin:$PATH" subctl version)
[[ "$actual" == 'subctl version: v0.22.2' ]]
[[ $("$fixture/local-bin/subctl" version) == 'subctl version: PR-build' ]]

# Run the actual deployment entry point with cluster operations stubbed out.
# This catches regressions in phase-specific PATH changes, not just installation.
mkdir -p "$fixture/scripts/lib"
ln -s "$scripts/get-subctl.sh" "$scripts/check-deployed-version.sh" "$fixture/scripts/"
touch "$fixture/scripts/lib/debug_functions" "$fixture/scripts/lib/deploy_funcs"
cat > "$fixture/scripts/lib/utils" <<'EOF'
OUTPUT_DIR=$DAPPER_OUTPUT
print_env() { :; }
exit_error() { echo "$*" >&2; exit 1; }
load_settings() {
    clusters=(cluster1 cluster2)
    declare -gA cluster_subm=([cluster1]=true [cluster2]=true)
}
declare_cidrs() { :; }
declare_kubeconfig() { :; }
load_library() { :; }
run_all_clusters() { :; }
run_if_defined() { :; }
with_context() { "${@:2}"; }
setup_broker() { :; }
install_subm_all_clusters() { :; }
verify_gw_status() { :; }
connectivity_tests() { :; }
print_clusters_message() { :; }
with_retries() { "${@:2}"; }
deploytool_prereqs() {
    subctl version > "$FIXTURE/active-version"
    command -v subctl > "$FIXTURE/active-path"
}
EOF
rm "$fixture/kube-error"
cp "$fixture/healthy-components.json" "$fixture/components.json"
PATH="$fixture/local-bin:$PATH" RELEASED_SUBCTL=true SUBCTL_VERSION=v0.22.2 SUBCTL_IMAGE_VERSION=release-0.22 \
    PROVIDER=fixture DAPPER_OUTPUT="$fixture/output" SCRIPTS_DIR="$fixture/scripts" \
    "$scripts/deploy.sh"
[[ $(cat "$fixture/active-version") == 'subctl version: v0.22.2' ]]
[[ $(cat "$fixture/active-path") == "$fixture/output/released-subctl/bin/subctl" ]]
[[ $(cat "$fixture/kubectl-args") == '--context cluster2 get deployments,daemonsets --namespace submariner-operator --output json' ]]

# Reject a successful installer that produced the wrong version.
expect_failure env PATH="$fixture/local-bin:$PATH" RELEASED_SUBCTL=true SUBCTL_VERSION=v0.22.2 \
    STUB_SUBCTL_VERSION=v0.24.2 PROVIDER=fixture DAPPER_OUTPUT="$fixture/output" SCRIPTS_DIR="$fixture/scripts" \
    "$scripts/deploy.sh"
grep -q 'Expected baseline subctl v0.22.2, got v0.24.2' "$fixture/error.log"

# An ordinary deployment still selects the previously built local subctl.
PATH="$fixture/local-bin:$PATH" RELEASED_SUBCTL=false SUBCTL_VERSION=devel \
    DESTDIR="$fixture/installed" PROVIDER=fixture DAPPER_OUTPUT="$fixture/output" SCRIPTS_DIR="$fixture/scripts" \
    "$scripts/deploy.sh"
[[ $(cat "$fixture/active-version") == 'subctl version: PR-build' ]]

# An explicit target image version also triggers deployment image/rollout checks.
PATH="$fixture/local-bin:$PATH" RELEASED_SUBCTL=false SUBCTL_IMAGE_VERSION=0.22.2 \
    SUBCTL_VERSION=devel DESTDIR="$fixture/installed" PROVIDER=fixture DAPPER_OUTPUT="$fixture/output" \
    SCRIPTS_DIR="$fixture/scripts" "$scripts/deploy.sh"
[[ $(cat "$fixture/active-version") == 'subctl version: PR-build' ]]
expect_failure env PATH="$fixture/local-bin:$PATH" RELEASED_SUBCTL=false SUBCTL_IMAGE_VERSION=0.24.2 \
    SUBCTL_VERSION=devel DESTDIR="$fixture/installed" PROVIDER=fixture DAPPER_OUTPUT="$fixture/output" \
    SCRIPTS_DIR="$fixture/scripts" "$scripts/deploy.sh"

# Run the real operator helpers, checking both broker and join image flags.
cat > "$fixture/bin/operator-subctl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FIXTURE/operator-calls"
EOF
chmod +x "$fixture/bin/operator-subctl"
for image_version in '' release-0.22; do
    rm -f "$fixture/operator-calls"
    SUBCTL="$fixture/bin/operator-subctl" SUBCTL_IMAGE_VERSION="$image_version" \
        OUTPUT_DIR="$fixture/output" SUBM_IMAGE_TAG=subctl cluster=cluster1 \
        bash -c '
            source "$1"
            declare -A cluster_subm=([cluster1]=true) global_CIDRs=([cluster1]="")
            setup_broker
            subctl_install_subm
        ' _ "$scripts/lib/deploy_operator"
    [[ $(wc -l < "$fixture/operator-calls") -eq 2 ]]
    if [[ -n "$image_version" ]]; then
        [[ $(grep -c -- '--version release-0.22' "$fixture/operator-calls") -eq 2 ]]
    else
        if grep -q -- '--version' "$fixture/operator-calls"; then
            echo 'Ordinary deployment unexpectedly forced an image version' >&2
            exit 1
        fi
    fi
done

echo 'Upgrade baseline regression tests passed'
