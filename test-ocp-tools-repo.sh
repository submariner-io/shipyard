#!/bin/bash
# Test OpenShift Developer Tools repository across architectures

CERT=$(ls /etc/pki/entitlement/*.pem 2>/dev/null | grep -v key | head -1)
KEY="${CERT%.pem}-key.pem"

[ -f "$CERT" ] && [ -f "$KEY" ] || { echo "Error: Entitlement certificate or key not found"; exit 1; }

echo "Testing OpenShift Developer Tools 4.19 for RHEL 9..."
echo ""

test_arch() {
    local arch=$1
    local base="https://cdn.redhat.com/content/dist/layered/rhel9/${arch}/ocp-tools/4.19/os"

    echo "Testing $arch:"
    tmp=$(mktemp)
    http=$(curl -sk -w %{http_code} -o "$tmp" --cert "$CERT" --key "$KEY" "$base/repodata/repomd.xml")

    if [ "$http" = "200" ]; then
        primary=$(grep -m1 primary.xml.gz "$tmp" | sed 's/.*href="//;s/".*//')
        if [ -n "$primary" ]; then
            count=$(curl -sk --cert "$CERT" --key "$KEY" "$base/$primary" | gunzip 2>/dev/null | grep -oP 'packages="\K[0-9]+' || echo 0)
            echo "  Result: HTTP $http, $count packages"

            # If packages exist, check for our required packages
            if [ "$count" -gt 0 ]; then
                echo "  Checking for required packages..."
                curl -sk --cert "$CERT" --key "$KEY" "$base/$primary" | gunzip > /tmp/primary-${arch}.xml 2>/dev/null

                for pkg in bind-utils curl iperf3 iproute iputils nmap-ncat tcpdump; do
                    if grep -q "<name>$pkg</name>" /tmp/primary-${arch}.xml 2>/dev/null; then
                        echo "    ✓ $pkg"
                    else
                        echo "    ✗ $pkg NOT found"
                    fi
                done
                rm -f /tmp/primary-${arch}.xml
            fi
        else
            echo "  Result: HTTP $http, no metadata"
        fi
    else
        echo "  Result: HTTP $http"
    fi
    rm -f "$tmp"
    echo ""
}

# Test all architectures that matter
for arch in x86_64 aarch64 ppc64le s390x; do
    test_arch $arch
done
