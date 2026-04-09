#!/bin/bash
# Unit tests for CVE fix library functions.
# Runs without docker, grype, or network access.
# Uses the real shipyard repo for integration tests.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

PASS=0 FAIL=0
check() {
  local desc="$1"; shift
  local negate=false
  [[ "$1" == "!" ]] && { negate=true; shift; }
  local rc=0; "$@" 2>/dev/null || rc=$?
  if { [[ "$negate" == false ]] && [[ $rc -eq 0 ]]; } || \
     { [[ "$negate" == true ]] && [[ $rc -ne 0 ]]; }; then
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $desc"; FAIL=$((FAIL + 1))
  fi
}

TMPD=$(mktemp -d)
trap 'rm -rf "$TMPD"' EXIT

# === Pure functions ===
echo "pure functions"
check "state_file_path" bash -c "[[ $(state_file_path /x/shipyard release-0.23) == /tmp/cve-fix-shipyard-release-0-23-*.env ]]"
check "state_file_path devel" bash -c "[[ $(state_file_path /x/admiral devel) == /tmp/cve-fix-admiral-devel-*.env ]]"
check "abbrev github" test "$(abbreviate_package github.com/docker/docker)" = "docker/docker"
check "abbrev x/" test "$(abbreviate_package golang.org/x/net)" = "x/net"
check "abbrev helm" test "$(abbreviate_package helm.sh/helm/v3)" = "helm/v3"
check "abbrev k8s" test "$(abbreviate_package k8s.io/client-go)" = "k8s.io/client-go"
check "abbrev other" test "$(abbreviate_package go.etcd.io/bbolt)" = "go.etcd.io/bbolt"
check "abbrev nested" test "$(abbreviate_package github.com/go-git/go-git/v5)" = "go-git/go-git/v5"
check "abbrev otel" test "$(abbreviate_package go.opentelemetry.io/otel)" = "otel"
check "abbrev otel/sdk" test "$(abbreviate_package go.opentelemetry.io/otel/sdk)" = "otel/sdk"
check "abbrev otel deep" test "$(abbreviate_package go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp)" = "otel/otlptracehttp"
check "abbrev grpc" test "$(abbreviate_package google.golang.org/grpc)" = "grpc"
check "abbrev sigs" test "$(abbreviate_package sigs.k8s.io/controller-runtime)" = "controller-runtime"
CVE_LIST=("GHSA-aaaa" "GHSA-bbbb"); JOINED=$(printf '%s, ' "${CVE_LIST[@]}"); JOINED="${JOINED%, }"
check "CVE join" test "$JOINED" = "GHSA-aaaa, GHSA-bbbb"

# === clean_gomod ===
echo "clean_gomod"
printf 'module t\ngo 1.25.0\ntoolchain go1.25.1\n' > "$TMPD/go.mod"
(cd "$TMPD" && clean_gomod)
check "toolchain removed" test -z "$(grep toolchain "$TMPD/go.mod")"
check "go kept" grep -q "^go 1.25.0" "$TMPD/go.mod"

# === load_state ===
echo "load_state"
check "missing fails" ! load_state /tmp/nonexistent-cve-test.env
printf 'REPO="%s"\nBRANCH="devel"\n' "$TMPD" > "$TMPD/s.env"
check "loads vars" bash -c "source '$SCRIPT_DIR/lib.sh'; load_state '$TMPD/s.env' && [[ \$BRANCH = devel ]]"

# === insert_grype_ignore ===
echo "insert_grype_ignore"
cp "$REPO_ROOT/.grype.yaml" "$TMPD/g.yaml"
insert_grype_ignore "$TMPD/g.yaml" GHSA-test-1234 example.com/pkg "Test reason"
check "before exclude" test "$(grep -n GHSA-test-1234 "$TMPD/g.yaml" | cut -d: -f1)" -lt "$(grep -n '^exclude:' "$TMPD/g.yaml" | cut -d: -f1)"
check "entry present" grep -q GHSA-test-1234 "$TMPD/g.yaml"
check "original kept" grep -q CVE-2015-5237 "$TMPD/g.yaml"
printf -- '---\nignore:\n  - vulnerability: CVE-1\n    package:\n      name: x\n' > "$TMPD/no-exc.yaml"
insert_grype_ignore "$TMPD/no-exc.yaml" GHSA-append example.com/other "No exclude"
check "appends" grep -q GHSA-append "$TMPD/no-exc.yaml"

# === detect.sh (real repo) ===
echo "detect.sh"
DETECT_OUT=$("$SCRIPT_DIR/detect.sh" "$REPO_ROOT" 0.23 2>&1) || true
check "normalizes 0.23" grep -q "release-0.23" <<< "$DETECT_OUT"
STATE=$(tail -1 <<< "$DETECT_OUT")
check "creates state" test -f "$STATE"; rm -f "$STATE"
check "bad repo fails" ! "$SCRIPT_DIR/detect.sh" /nonexistent devel
check "non-git fails" ! "$SCRIPT_DIR/detect.sh" /tmp devel

# === locate.sh (real repo, inline state) ===
echo "locate.sh"
printf 'REPO="%s"\nBRANCH="devel"\nCVE_SCRIPTS="%s"\n' "$REPO_ROOT" "$SCRIPT_DIR" > "$TMPD/loc.env"
LOCATE_OUT=$(bash "$SCRIPT_DIR/locate.sh" "$TMPD/loc.env" k8s.io/client-go 2>&1) || true
check "finds in go.mod" grep -q "Found in:" <<< "$LOCATE_OUT"
LOCATE_OUT=$(bash "$SCRIPT_DIR/locate.sh" "$TMPD/loc.env" github.com/golangci/golangci-lint 2>&1) || true
check "finds in tools" grep -q "tools" <<< "$LOCATE_OUT"
check "missing fails" ! bash "$SCRIPT_DIR/locate.sh" "$TMPD/loc.env" nonexistent/pkg >/dev/null

# === Summary ===
echo ""
TOTAL=$((PASS + FAIL))
echo "$PASS/$TOTAL passed"
if [[ "$FAIL" -gt 0 ]]; then echo "$FAIL FAILED"; exit 1; fi
