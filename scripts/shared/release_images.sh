#!/usr/bin/env bash

set -e
source "${SCRIPTS_DIR}/lib/utils"

[[ $# -gt 0 ]] || exit_error "At least one image to release must be specified!"

print_env REPO TAG
source "${SCRIPTS_DIR}/lib/debug_functions"

# Temp dir for capturing pushed manifest digests so we sign the exact artifact.
SIGN_TMPDIR="$(mktemp -d)"
readonly SIGN_TMPDIR

function cleanup() {
    cosign logout quay.io 2>/dev/null || true
    skopeo logout quay.io 2>/dev/null || true
    rm -rf "${SIGN_TMPDIR}"
}
trap cleanup EXIT

# Sign a pushed image (by digest) with cosign keyless (Sigstore OIDC).
# --recursive also signs the child manifests of a multi-arch index (the default
# release path), not just the index itself; it is a no-op for single-arch images.
# Non-fatal: warns and returns 0 if cosign is missing or signing fails, so a
# signing problem never breaks the release push.
function sign_image() {
    local ref="$1"
    if ! command -v cosign >/dev/null 2>&1; then
        >&2 printf 'WARNING: cosign not found on PATH; skipping signing of %s\n' "${ref}"
        return 0
    fi
    cosign sign --yes --recursive "${ref}" \
        || >&2 printf 'WARNING: failed to sign %s\n' "${ref}"
}

function release_image() {
    local digestfile="${SIGN_TMPDIR}/digest"
    # Reset so the guard below reflects THIS image's copy, not a prior image's leftover.
    rm -f "${digestfile}"
    for target_tag in $VERSION $TAG; do
        local target_image="${image}:${target_tag#v}"
        if [[ -z "${OCIDIR}" ]]; then
            # Single-arch
            skopeo copy --digestfile "${digestfile}" "docker-daemon:${REPO}/${image}:${DEV_VERSION}" "docker://${REPO}/${target_image}"
        else
            skopeo copy --all --digestfile "${digestfile}" "oci-archive:${OCIDIR}/${image}.tar" "docker://${REPO}/${target_image}"
        fi
    done
    # VERSION and TAG push identical content, so one signature by digest covers
    # both tags; sign once after the pushes succeed.
    if [[ -f "${digestfile}" ]]; then
        sign_image "${REPO}/${image}@$(cat "${digestfile}")"
    fi
}

echo "$QUAY_PASSWORD" | skopeo login quay.io -u "$QUAY_USERNAME" --password-stdin

# cosign uses keyless signing (OIDC) but still needs registry auth to push the
# signature blob to quay.io. Non-fatal.
if command -v cosign >/dev/null 2>&1; then
    echo "$QUAY_PASSWORD" | cosign login quay.io -u "$QUAY_USERNAME" --password-stdin \
        || >&2 printf 'WARNING: cosign login to quay.io failed; signing may be skipped\n'
fi

for image; do
    release_image
done

