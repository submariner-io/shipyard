#!/bin/bash
# Detect repository configuration for CVE fixing and optionally set up fix branch.
# Usage: detect.sh [repo] [branch] [--setup-branch]
# Prints state file path as last line of stdout.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

# --- Argument parsing (order-independent) ---
REPO=""
BRANCH=""
SETUP_BRANCH=false

for arg in "$@"; do
  case "$arg" in
    --setup-branch) SETUP_BRANCH=true ;;
    *)
      # Expand tilde
      arg="${arg/#\~/$HOME}"
      # Try sibling directory for bare names (e.g., "subctl" -> "../subctl")
      if [[ "$arg" != */* ]] && [[ -d "../$arg" ]]; then
        arg="../$arg"
      fi
      if [[ "$arg" == /* ]] || [[ "$arg" == ./* ]] || [[ "$arg" == ../* ]] || [[ -d "$arg" ]]; then
        [[ -n "$REPO" ]] && { echo "ERROR: Multiple repositories specified" >&2; exit 1; }
        REPO="$arg"
      else
        [[ -n "$BRANCH" ]] && { echo "ERROR: Multiple branches specified" >&2; exit 1; }
        BRANCH="$arg"
      fi
      ;;
  esac
done

REPO="${REPO:-.}"

# --- Validate repo ---
[[ -d "$REPO" ]] || { echo "ERROR: Repository not found: $REPO" >&2; exit 1; }
git -C "$REPO" rev-parse --git-dir &>/dev/null || { echo "ERROR: Not a git repository: $REPO" >&2; exit 1; }

# --- Resolve branch ---
if [[ -z "$BRANCH" ]]; then
  BRANCH=$(git -C "$REPO" branch --show-current 2>/dev/null)
  [[ -n "$BRANCH" ]] || { echo "ERROR: Not on a branch (detached HEAD). Specify branch explicitly." >&2; exit 1; }
  echo "Using current branch: $BRANCH"
fi

# Normalize short version (0.23 -> release-0.23)
if [[ "$BRANCH" =~ ^[0-9]+\.[0-9]+$ ]]; then
  BRANCH="release-${BRANCH}"
  echo "Normalized to branch: $BRANCH"
fi

# --- Change to repo (absolute path) ---
REPO="$(cd "$REPO" && pwd)"
cd "$REPO"

REPO_NAME=$(basename "$REPO")
echo ""
echo "=== CVE Fix: $REPO_NAME/$BRANCH ==="

# --- Detect config ---
HAS_TOOLS_GOMOD=false
test -f tools/go.mod && HAS_TOOLS_GOMOD=true

GENERATED_FILE=""
DIFF_IGNORE_ARGS=""
if grep -rql "controller-gen.kubebuilder.io/version" --include="*.go" . 2>/dev/null; then
  DIFF_IGNORE_ARGS="-Icontroller-gen.kubebuilder.io/version"
  GENERATED_FILE=$(grep -rl "controller-gen.kubebuilder.io/version" --include="*.go" . 2>/dev/null | head -1)
elif find . -name "*.pb.go" -type f 2>/dev/null | head -1 | grep -q .; then
  DIFF_IGNORE_ARGS="-I^//"
  GENERATED_FILE=$(find . -name "*.pb.go" -type f 2>/dev/null | head -1)
fi

NEEDS_BUILD_FOR_SCAN=false
grep -q '^build:' Makefile 2>/dev/null && NEEDS_BUILD_FOR_SCAN=true

CONTAINER_CMD=$(detect_container_cmd)

HAS_LOCAL_GRYPE=false
command -v grype &>/dev/null && HAS_LOCAL_GRYPE=true

# Shipyard build image
if [[ "$BRANCH" == "devel" ]]; then
  SHIPYARD_TAG="devel"
elif [[ "$BRANCH" =~ ^release- ]]; then
  SHIPYARD_TAG="$BRANCH"
else
  echo "WARNING: Unknown branch pattern, assuming devel build image"
  SHIPYARD_TAG="devel"
fi

SHIPYARD_IMAGE="quay.io/submariner/shipyard-dapper-base:${SHIPYARD_TAG}"
SHIPYARD_GO_VERSION="unknown"
if [[ -n "$CONTAINER_CMD" ]] && [[ -n "$($CONTAINER_CMD image ls -q "$SHIPYARD_IMAGE" 2>/dev/null)" ]]; then
  SHIPYARD_GO_VERSION=$($CONTAINER_CMD run --rm "$SHIPYARD_IMAGE" go version 2>/dev/null || echo "unknown")
fi

# --- Branch setup (optional) ---
ORIGINAL_REF=""
FIX_BRANCH=""
FETCH_FAILED=false

if [[ "$SETUP_BRANCH" == "true" ]]; then
  if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
    echo "ERROR: Working tree has uncommitted changes. Commit or stash first." >&2
    exit 1
  fi

  ORIGINAL_REF=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
  [[ "$ORIGINAL_REF" == "HEAD" ]] && ORIGINAL_REF=$(git rev-parse HEAD)

  echo "Fetching latest changes..."
  if ! GIT_SSH_COMMAND="ssh -o BatchMode=yes" git fetch 2>/dev/null; then
    echo "WARNING: git fetch failed. Continuing with cached remote state."
    FETCH_FAILED=true
  fi

  DATE=$(date +%Y-%m-%d)
  VERSION="${BRANCH//release-/}"
  FIX_BRANCH="fix-${VERSION}-cves-${DATE}"

  SUFFIX=""
  while git show-ref --verify --quiet "refs/heads/${FIX_BRANCH}${SUFFIX}"; do
    if [[ -z "$SUFFIX" ]]; then SUFFIX="-v2"; else NUM=${SUFFIX#-v}; SUFFIX="-v$((NUM+1))"; fi
  done
  FIX_BRANCH="${FIX_BRANCH}${SUFFIX}"

  if ! git checkout -b "$FIX_BRANCH" "origin/$BRANCH" 2>/dev/null; then
    echo "ERROR: Could not create fix branch from origin/$BRANCH" >&2
    exit 1
  fi
  echo "Fix branch: $FIX_BRANCH"
fi

# --- Write state file ---
STATE_FILE=$(state_file_path "$REPO" "$BRANCH")
cat > "$STATE_FILE" <<EOF
STATE_FILE="$STATE_FILE"
REPO="$REPO"
BRANCH="$BRANCH"
FIX_BRANCH="$FIX_BRANCH"
ORIGINAL_REF="$ORIGINAL_REF"
HAS_TOOLS_GOMOD=$HAS_TOOLS_GOMOD
GENERATED_FILE="$GENERATED_FILE"
DIFF_IGNORE_ARGS="$DIFF_IGNORE_ARGS"
NEEDS_BUILD_FOR_SCAN=$NEEDS_BUILD_FOR_SCAN
CONTAINER_CMD="$CONTAINER_CMD"
HAS_LOCAL_GRYPE=$HAS_LOCAL_GRYPE
SHIPYARD_IMAGE="$SHIPYARD_IMAGE"
SHIPYARD_GO_VERSION="$SHIPYARD_GO_VERSION"
FETCH_FAILED=$FETCH_FAILED
CVE_SCRIPTS="$SCRIPT_DIR"
EOF

# Last line is the state file path (captured by callers)
echo "$STATE_FILE"
