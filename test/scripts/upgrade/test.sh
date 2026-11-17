#!/usr/bin/env bash

# Offline regression tests for upgrade baseline selection.
set -e -o pipefail

repo=$(cd "$(dirname "$0")/../../.." && pwd)
scripts="$repo/scripts/shared"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/bin"
export FIXTURE="$fixture"
export PATH="$fixture/bin:$PATH"

cat > "$fixture/bin/git" <<'EOF'
#!/usr/bin/env bash
set -e
[ "$1" = "ls-remote" ] || exit 0
[ "$2" = "--tags" ] || exit 0
[[ ! -f "$FIXTURE/api-error" ]] || exit 22
cat "$FIXTURE/tags"
EOF
chmod +x "$fixture/bin/git"

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

cat > "$fixture/tags" <<'EOF'
foo refs/tags/v0.25.0
foo refs/tags/v0.24.2
foo refs/tags/v0.23.4
foo refs/tags/v0.23.3
foo refs/tags/v0.22.9
foo refs/tags/v0.22.10
foo refs/tags/v0.22.13-rc1
foo refs/tags/subctl-release-0.22
EOF

expect_baseline v0.22.10 release-0.22
expect_baseline v0.23.4 release-0.23
expect_baseline v0.25.0 devel
expect_baseline v0.25.0 release-0.25
expect_baseline v0.22.2 release-0.22 0.22.2
expect_baseline v0.22.2 custom-branch v0.22.2

expect_failure "$scripts/resolve-upgrade-baseline.sh" feature-unrecognized
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.30
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22 latest
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22 v0.22.2-rc1

# Unreleased minor falls back to the preceding minor's highest stable patch.
cat > "$fixture/tags" <<'EOF'
foo refs/tags/v0.24.2
foo refs/tags/v0.23.4
EOF
expect_baseline v0.24.2 release-0.25

# A similar minor prefix must not match; do not fall back to a newer stream.
echo 'foo refs/tags/v0.220.1' > "$fixture/tags"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22

touch "$fixture/api-error"
expect_failure "$scripts/resolve-upgrade-baseline.sh" release-0.22
# An exact override must work without network access.
expect_baseline v0.22.2 release-0.22 v0.22.2

echo "Upgrade baseline selection tests passed"
