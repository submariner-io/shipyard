# RPM Lockfiles for Konflux Hermetic Builds

This directory contains RPM lockfiles and tooling for Konflux hermetic container builds.

**Directory Structure:**

- Scripts and docs (this README) live on the `devel` branch
- Component configs (`<component>/rpms.in.yaml`, `.repo` files) live on release branches

## Prerequisites

### 1. Red Hat Customer Portal Login ID

This is a separate account from your normal Red Hat SSO/Kerberos credentials.
The login ID must follow a specific naming convention. See
[Slack thread](https://redhat-internal.slack.com/archives/CKPULPXL3/p1767769916219789)
for details on how to request one.

### 2. Create Activation Key

Log in with your Customer Portal credentials (not SSO) and create an activation key:

https://console.redhat.com/insights/connector/activation-keys/

**Note:** The activation key name is used as a secret. Keep the random default
and add something like `yourname-yourproject-randomstring`.

### 3. Add Repos to Activation Key

RHEL 10 BaseOS and AppStream are auto-enabled - these are all that nettest requires.

**Note:** UBI repos are public and don't require activation keys, but nettest
needs `iperf3` and `tcpdump` which are only available in RHEL AppStream.

### 4. Register Your System

Red Hat VPN may be required.

```bash
sudo subscription-manager unregister
sudo subscription-manager clean
sudo subscription-manager register --org='<ORG_ID>' --activationkey='<KEY_NAME>'
```

Find your org ID on the [activation key page](https://console.redhat.com/insights/connector/activation-keys/).

### 5. Registry Login

```bash
podman login registry.redhat.io
```

This uses your Red Hat account (not the new Customer Portal account).

## Verification

### 6. Verify Repository Access

```bash
.rpm-lockfiles/check-repo-access.sh
```

Expected output (all OK):

```text
Shipyard RPM Dependency Status
===============================

Component  Packages                        Repository        x86_64  aarch64 ppc64le s390x
---------  ------------------------------  ----------------  ------  ------- ------- -----
nettest    iperf3,tcpdump                  RHEL 10 AppStream OK      OK      OK      OK
nettest    bind-utils,curl,iproute,...     UBI (public)      OK      OK      OK      OK

Legend: OK=accessible  403=subscription lacks this arch
```

### 7. Verify Packages (Optional)

```bash
.rpm-lockfiles/verify-packages.sh <branch>
```

Runs dnf inside a container to verify each package is available for each architecture.
Branch is required since component configs live on release branches, not devel.

Expected output:

```text
nettest (repos: rhel-10-for-appstream-rpms rhel-10-for-baseos-rpms)
  x86_64   OK: bind-utils@rhel-appstream curl@rhel-baseos iperf3@rhel-appstream iproute@rhel-baseos iputils@rhel-baseos nmap-ncat@rhel-appstream tcpdump@rhel-appstream
  aarch64  OK: bind-utils@rhel-appstream curl@rhel-baseos iperf3@rhel-appstream iproute@rhel-baseos iputils@rhel-baseos nmap-ncat@rhel-appstream tcpdump@rhel-appstream
  ppc64le  OK: bind-utils@rhel-appstream curl@rhel-baseos iperf3@rhel-appstream iproute@rhel-baseos iputils@rhel-baseos nmap-ncat@rhel-appstream tcpdump@rhel-appstream
  s390x    OK: bind-utils@rhel-appstream curl@rhel-baseos iperf3@rhel-appstream iproute@rhel-baseos iputils@rhel-baseos nmap-ncat@rhel-appstream tcpdump@rhel-appstream
```

## Updating Lockfiles

### 8. Generate Lockfiles

```bash
.rpm-lockfiles/update-lockfile.sh <branch> [component]
```

Generates `rpms.lock.yaml` from component configs on the specified branch.

Example:
```bash
.rpm-lockfiles/update-lockfile.sh release-0.19 nettest
```

### 9. Update Tekton Pipeline Architectures

Ensure the `build-platforms` in `.tekton/<component>-*-push.yaml` and
`.tekton/<component>-*-pull-request.yaml` match the arches in `rpms.in.yaml`.

Example from a push pipeline:
```yaml
  - name: build-platforms
    value:
    - linux/x86_64
    - linux/arm64
    - linux/ppc64le
    - linux/s390x
```

**Note:** Tekton uses `arm64` while lockfiles use `aarch64` - these refer to the same architecture.

### 10. Upload Activation Key to Konflux

Once local validation is complete, create a secret in your Konflux tenant namespace
so builds can access subscription content.

Login to the Konflux cluster:
```bash
oc login --web https://api.kflux-prd-rh02.0fk9.p1.openshiftapps.com:6443/
```

Check current state (record existing values before making changes):
```bash
oc get secret activation-key -n submariner-tenant -o yaml 2>/dev/null || echo "Secret does not exist yet"
```

Create or update the activation key secret:
```bash
oc create secret generic activation-key -n submariner-tenant \
  --from-literal=org='<ORG_ID>' \
  --from-literal=activationkey='<KEY_NAME>' \
  --dry-run=client -o yaml | oc apply -f -
```

Verify both fields match what you set:
```bash
oc get secret activation-key -n submariner-tenant -o jsonpath='{.data.org}' | base64 -d && echo
oc get secret activation-key -n submariner-tenant -o jsonpath='{.data.activationkey}' | base64 -d && echo
```

Confirm the output matches `<ORG_ID>` and `<KEY_NAME>` from the create command above.

Using the default name `activation-key` applies it to all builds in the namespace.

See [Konflux activation key docs](https://konflux-ci.dev/docs/building/activation-keys-subscription/).

---

## Reference

### Component Details

#### nettest

| Package | Source |
|---------|--------|
| iperf3, tcpdump | RHEL 10 AppStream |
| bind-utils, curl, iproute, iputils, nmap-ncat | RHEL 10 BaseOS |

**Note:** `iperf3` and `tcpdump` are NOT available in UBI repos - only in RHEL AppStream.
This is why nettest requires RHEL subscription entitlements.
