#!/bin/bash
# Tests RHEL 9/10 repository access across architectures
# Shows: x86/arm on GA (dist), s390x on EUS 9.4, ppc64le blocked everywhere (RHEL 9 & 10)

CERT=$(ls /etc/pki/entitlement/*.pem 2>/dev/null | grep -v key | head -1)
KEY="${CERT%.pem}-key.pem"

[ -f "$CERT" ] && [ -f "$KEY" ] || { echo "Error: Entitlement certificate or key not found"; exit 1; }

test_repo() {
    local arch=$1 path=$2 rhel_ver=$3 version=$4

    if [ -n "$version" ]; then
        local base="https://cdn.redhat.com/content/$path/rhel${rhel_ver}/$version/$arch/baseos/os"
        local label="$path/$version"
    else
        local base="https://cdn.redhat.com/content/$path/rhel${rhel_ver}/${rhel_ver}/$arch/baseos/os"
        local label="$path"
    fi

    local tmp=$(mktemp)
    local http=$(curl -sk -w %{http_code} -o "$tmp" --cert "$CERT" --key "$KEY" "$base/repodata/repomd.xml")

    if [ "$http" = "200" ]; then
        local primary=$(grep -m1 primary.xml.gz "$tmp" | sed 's/.*href="//;s/".*//')
        if [ -n "$primary" ]; then
            local count=$(curl -sk --cert "$CERT" --key "$KEY" "$base/$primary" | gunzip 2>/dev/null | grep -oP 'packages="\K[0-9]+' || echo 0)
            echo "$arch ($label): HTTP $http, $count packages"
        else
            echo "$arch ($label): HTTP $http, no metadata"
        fi
    else
        echo "$arch ($label): HTTP $http"
    fi
    rm -f "$tmp"
}

echo "=== RHEL 9 ==="

# Working configurations
test_repo x86_64 dist 9
test_repo aarch64 dist 9
test_repo s390x eus 9 9.4

# ppc64le failures
echo ""
test_repo ppc64le dist 9
test_repo ppc64le beta 9
test_repo ppc64le eus 9 9.4

echo ""
echo "=== RHEL 10 ==="

# ppc64le also blocked in RHEL 10
test_repo ppc64le beta 10
