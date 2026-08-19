#!/usr/bin/env bash

set -e -o pipefail
source "${SCRIPTS_DIR}/lib/utils"

# In case we're pretending to be `subctl`
if [[ "${0##*/}" = subctl ]] && [[ -L "$0" ]]; then
    run_subctl=true

    # Delete ourselves to ensure we don't run into issues with the new subctl
    rm -f "$0"
fi

# Default to devel if we don't know what base branch we're on.
VERSION="${SUBCTL_VERSION:-${BASE_BRANCH:-devel}}"

# `devel` and `release-0.X` are subctl-repo release tags whose tarball URL is
# derivable directly, so download the archive and verify it against the
# published checksums before use. Everything else (`latest`, `rc`, a concrete
# `vX.Y.Z`) is an alias resolved by get.submariner.io, so it stays on the
# existing install path.
case "${VERSION}" in
devel|release-0.*)
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"
    case "${arch}" in
        x86_64) arch=amd64;;
        aarch64) arch=arm64;;
        armv7l|armv6l) arch=arm;;
    esac
    archive="subctl-${VERSION}-${os}-${arch}.tar.xz"
    baseurl="https://github.com/submariner-io/subctl/releases/download/subctl-${VERSION}"
    mkdir -p ~/.local/bin
    with_retries 3 curl -Lsf -o "/tmp/${archive}" "${baseurl}/${archive}"
    with_retries 3 curl -Lsf -o /tmp/subctl-checksums.txt "${baseurl}/subctl-checksums.txt"
    (cd /tmp && grep -F -- "  ${archive}" subctl-checksums.txt | sha256sum -c -)
    tar -xJf "/tmp/${archive}" -C ~/.local/bin/ --strip-components=1
    ;;
*)
    with_retries 3 curl -Lsf https://get.submariner.io | VERSION="${VERSION}" bash
    ;;
esac

# If we're pretending to be subctl, run subctl with any given arguments
[[ -z "${run_subctl}" ]] || subctl "$@"
