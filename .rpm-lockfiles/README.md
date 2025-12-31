# RPM Lockfiles for Konflux Hermetic Builds

This directory contains RPM lockfiles and tooling for Konflux hermetic container builds.

**Directory Structure:**

- Scripts and docs (this README) live on `devel`
- Component configs (`<component>/rpms.in.yaml`, `.repo` files) live on release branches

## Prerequisites

Red Hat entitlement certificates are required to run the lockfile scripts.

### Activation Key Setup

Go to [Red Hat Console](https://console.redhat.com) → RHEL → Inventory → System Configuration →
Activation Keys. These are RHEL activation keys; BaseOS and AppStream are auto-enabled for
supported arches.

### Register Your System

Red Hat VPN may be required.

```bash
# If switching keys, unregister and clean first:
sudo subscription-manager unregister
sudo subscription-manager clean

sudo subscription-manager register --org="YOUR_ORG_ID" --activationkey="YOUR_KEY_NAME"
```

### Verify Access

```bash
.rpm-lockfiles/check-repo-access.sh
```

## Current Status

| Component | x86_64 | aarch64 | ppc64le | s390x |
|-----------|--------|---------|---------|-------|
| nettest   | OK     | OK      | 403     | EUS   |

**Legend:**

- OK = working with standard RHEL 9 repos
- EUS = working with Extended Update Support repos (see s390x EUS Solution)
- 403 = repos inaccessible with current subscription (see Blocking Issues)

## s390x EUS Solution

Standard RHEL 9 repos return 403 for s390x with self-serve subscriptions. However,
**EUS (Extended Update Support) repos are accessible** and contain all required packages.

The solution (implemented on release branches for nettest):

1. Add `skip_if_unavailable = 1` to standard repo entries (allows graceful fallback)
2. Add s390x-specific EUS repo entries pointing to `content/eus/rhel9/9.4/s390x/`
3. Add s390x to `rpms.in.yaml` arches and regenerate lockfile
4. Add `linux/s390x` to Tekton pipeline build-platforms

See release branch `.rpm-lockfiles/nettest/` for working implementation.

## Blocking Issues

### ppc64le (all components)

All RHEL 9 repos for ppc64le return 403 with self-serve activation keys:
- Standard repos: 403
- EUS repos: 403
- TUS/E4S/AUS repos: 403

May require OpenShift Platform Plus or enterprise subscription with ppc64le entitlements.

## Component Details

### nettest

| Package | Available In |
|---------|--------------|
| iperf3, tcpdump | RHEL 9 AppStream |
| bind-utils, curl, iproute, iputils, nmap-ncat | UBI (public) |

iperf3 and tcpdump are **not in UBI** - only RHEL 9 AppStream. s390x can use EUS repos; ppc64le requires a different subscription.

## Verification Scripts

### Quick Access Check

```bash
.rpm-lockfiles/check-repo-access.sh
```

Example output (actual results depend on your subscription):

```text
Component  Packages                        Repository        x86_64  aarch64 ppc64le s390x
---------  ------------------------------  ----------------  ------  ------- ------- -----
nettest    iperf3,tcpdump                  RHEL 9 AppStream  OK      OK      403     403
nettest    bind-utils,curl,iproute,...     UBI (public)      OK      OK      OK      OK
```

**Note:** nettest s390x shows 403 because the script tests standard repos. EUS repos
are accessible and can be configured in `.repo` files - see s390x EUS Solution.

### Detailed Package Verification

```bash
.rpm-lockfiles/verify-packages.sh [branch]
```

Example output (actual results depend on your subscription and branch configuration):

```text
nettest (repos: rhel-9-for-appstream-rpms rhel-9-for-baseos-rpms rhel-9-for-s390x-appstream-eus-rpms ...)
  x86_64   OK: bind-utils@rhel-appstream curl@rhel-baseos iperf3@rhel-appstream ...
  aarch64  OK: bind-utils@rhel-appstream curl@rhel-baseos iperf3@rhel-appstream ...
  ppc64le  NO REPO ACCESS (subscription lacks ppc64le)
  s390x    OK: bind-utils@rhel-appstream-eus curl@rhel-baseos-eus iperf3@rhel-appstream-eus ...
```

### Update Lockfiles

```bash
.rpm-lockfiles/update-lockfile.sh <branch> [component]
```

Generates `rpms.lock.yaml` from component configs on the specified branch.

**Additional prerequisite:** `podman login registry.redhat.io`
