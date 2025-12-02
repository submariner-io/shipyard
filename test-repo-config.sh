#!/bin/bash
# Comprehensive test of RHEL 9 multi-arch repository configuration

set -euo pipefail

echo "======================================================================"
echo "Testing RHEL 9 Multi-Architecture Repository Configuration"
echo "======================================================================"
echo ""

REPO_FILE=".rpm-lockfiles/nettest/submariner-rhel-9.repo"
CERT=$(ls /etc/pki/entitlement/*.pem 2>/dev/null | grep -v key | head -1)
KEY="${CERT%.pem}-key.pem"

[ -f "$CERT" ] && [ -f "$KEY" ] || { echo "ERROR: Entitlement certificates not found"; exit 1; }
[ -f "$REPO_FILE" ] || { echo "ERROR: Repository file not found: $REPO_FILE"; exit 1; }

echo "1. Verifying repository configuration..."
echo ""

# Check x86_64 (should be enabled)
if grep -A5 "^\[rhel-9-for-x86_64-baseos-rpms\]" "$REPO_FILE" | grep -q "enabled = 1"; then
    echo "✅ x86_64: enabled (GA dist repos)"
else
    echo "❌ x86_64: NOT enabled or misconfigured"
    exit 1
fi

# Check aarch64 (should be enabled)
if grep -A5 "^\[rhel-9-for-aarch64-baseos-rpms\]" "$REPO_FILE" | grep -q "enabled = 1"; then
    echo "✅ aarch64: enabled (GA dist repos)"
else
    echo "❌ aarch64: NOT enabled or misconfigured"
    exit 1
fi

# Check ppc64le (uses separate CentOS Stream repo file)
CENTOS_REPO_FILE=".rpm-lockfiles/nettest/centos-stream-9-ppc64le.repo"
if [ -f "$CENTOS_REPO_FILE" ] && grep -q "^\[centos-stream-9-baseos-ppc64le\]" "$CENTOS_REPO_FILE"; then
    echo "✅ ppc64le: Uses CentOS Stream repos (separate config)"
else
    echo "❌ ppc64le: CentOS Stream repo file not found or misconfigured"
    exit 1
fi

# Check s390x (should be enabled)
if grep -A5 "^\[rhel-9-for-s390x-baseos-rpms\]" "$REPO_FILE" | grep -q "enabled = 1"; then
    echo "✅ s390x: enabled (EUS 9.4 repos)"
else
    echo "❌ s390x: NOT enabled or misconfigured"
    exit 1
fi

echo ""
echo "2. Testing repository accessibility..."
echo ""

test_repo() {
    local arch=$1
    local url=$2
    local expected_status=$3

    local http=$(curl -sk -w %{http_code} -o /dev/null --cert "$CERT" --key "$KEY" "$url/repodata/repomd.xml")

    if [ "$http" = "$expected_status" ]; then
        echo "✅ $arch: HTTP $http (as expected)"
        return 0
    else
        echo "❌ $arch: HTTP $http (expected $expected_status)"
        return 1
    fi
}

# Test x86_64 (GA dist)
test_repo "x86_64" "https://cdn.redhat.com/content/dist/rhel9/9/x86_64/baseos/os" "200"

# Test aarch64 (GA dist)
test_repo "aarch64" "https://cdn.redhat.com/content/dist/rhel9/9/aarch64/baseos/os" "200"

# Test s390x (EUS 9.4)
test_repo "s390x" "https://cdn.redhat.com/content/eus/rhel9/9.4/s390x/baseos/os" "200"

# Test ppc64le RHEL repos (returns 200 but has 0 packages - unusable)
echo "⚠️  ppc64le RHEL repos: Testing (expect HTTP 200 but repo is empty)"
test_repo "ppc64le (RHEL beta)" "https://cdn.redhat.com/content/beta/rhel9/9/ppc64le/baseos/os" "200" || true

# Test ppc64le CentOS Stream (should work - 200)
test_repo "ppc64le (CentOS Stream)" "https://mirror.stream.centos.org/9-stream/BaseOS/ppc64le/os" "200"

echo ""
echo "3. Verifying rpms.in.yaml architecture list..."
echo ""

RPMS_IN=".rpm-lockfiles/nettest/rpms.in.yaml"
if grep -q "ppc64le" "$RPMS_IN"; then
    echo "✓ ppc64le is listed in rpms.in.yaml (uses CentOS Stream repos)"
else
    echo "✗ ERROR: ppc64le should be in rpms.in.yaml"
    exit 1
fi

echo ""
echo "======================================================================"
echo "✅ Configuration Test PASSED"
echo "======================================================================"
echo ""
echo "Summary:"
echo "  • x86_64:  Enabled, using RHEL 9 GA (dist) repos"
echo "  • aarch64: Enabled, using RHEL 9 GA (dist) repos"
echo "  • s390x:   Enabled, using RHEL 9 EUS 9.4 repos"
echo "  • ppc64le: Enabled, using CentOS Stream 9 repos (RHEL incompatible)"
echo ""
echo "The configuration supports all 4 architectures with appropriate sources."
echo ""
