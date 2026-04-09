# shellcheck shell=bash
# Shared functions for CVE fix scripts

# Compute state file path from repo path and branch name
state_file_path() {
  local repo_basename branch_sanitized
  repo_basename=$(basename "$1")
  branch_sanitized=$(echo "$2" | tr './' '-')
  echo "/tmp/cve-fix-${repo_basename}-${branch_sanitized}-$$.env"
}

# Load state from file, cd to repo
load_state() {
  local state_file="${1:?Usage: load_state STATE_FILE}"
  if [[ ! -f "$state_file" ]]; then
    echo "ERROR: State file not found: $state_file" >&2
    echo "Run detect.sh first." >&2
    return 1
  fi
  # shellcheck source=/dev/null
  source "$state_file"
  cd "$REPO" || return 1
}

# Detect container runtime: docker, podman, or empty string
detect_container_cmd() {
  if command -v docker &>/dev/null && docker info &>/dev/null; then
    echo "docker"
  elif command -v podman &>/dev/null && podman info &>/dev/null; then
    echo "podman"
  else
    echo ""
  fi
}

# Run grype scan via local install or container fallback
# Usage: run_grype [--fresh] [--no-update] [--json]
# --fresh: ensure DB is up-to-date (grype auto-updates if stale)
# --no-update: skip DB update check (use after a recent --fresh scan)
# --json: output JSON instead of table
run_grype() {
  local fresh=false no_update=false output_format="table"
  for arg in "$@"; do
    case "$arg" in
      --fresh) fresh=true ;;
      --no-update) no_update=true ;;
      --json) output_format="json" ;;
    esac
  done

  # Prefer local grype (fast, uses host-cached DB)
  if [[ "$HAS_LOCAL_GRYPE" == "true" ]]; then
    if [[ "$no_update" == "true" ]]; then
      GRYPE_DB_AUTO_UPDATE=false GRYPE_DB_VALIDATE_AGE=false \
        grype . --config .grype.yaml -o "$output_format"
    else
      grype . --config .grype.yaml -o "$output_format"
    fi
    return $?
  fi

  # Fall back to container
  if [[ -n "$CONTAINER_CMD" ]]; then
    if [[ "$fresh" == "true" ]]; then
      $CONTAINER_CMD volume create grype-db >/dev/null 2>&1 || true
      echo "Updating vulnerability database (container)..." >&2
      if ! $CONTAINER_CMD run --pull=always --rm \
        -v grype-db:/.cache/grype anchore/grype:latest db update >&2; then
        echo "WARNING: DB update failed. Scan will use cached data." >&2
      fi
    fi
    local -a env_args=()
    if [[ "$no_update" == "true" ]]; then
      env_args=(-e GRYPE_DB_AUTO_UPDATE=false -e GRYPE_DB_VALIDATE_AGE=false)
    fi
    if $CONTAINER_CMD run --rm "${env_args[@]}" \
      -v grype-db:/.cache/grype \
      -v "$(pwd)":/src \
      anchore/grype:latest /src --config /src/.grype.yaml -o "$output_format"; then
      return 0
    fi
    echo "WARNING: Container scan failed." >&2
  fi

  echo "ERROR: No scanner available." >&2
  echo "  Install grype locally or docker/podman." >&2
  return 1
}

# Abbreviate Go package path for commit messages
# github.com/docker/docker -> docker/docker
# golang.org/x/net -> x/net
# helm.sh/helm/v3 -> helm/v3
# go.opentelemetry.io/otel -> otel
# go.opentelemetry.io/otel/exporters/otlp/*/X -> otel/X
# google.golang.org/grpc -> grpc
# sigs.k8s.io/controller-runtime -> controller-runtime
# k8s.io/* stays as-is
abbreviate_package() {
  local pkg="$1"
  case "$pkg" in
    github.com/*) echo "${pkg#github.com/}" ;;
    golang.org/x/*) echo "x/${pkg#golang.org/x/}" ;;
    helm.sh/*) echo "${pkg#helm.sh/}" ;;
    go.opentelemetry.io/otel/exporters/otlp/*)
      echo "otel/$(basename "$pkg")" ;;
    go.opentelemetry.io/*) echo "${pkg#go.opentelemetry.io/}" ;;
    google.golang.org/*) echo "${pkg#google.golang.org/}" ;;
    sigs.k8s.io/*) echo "${pkg#sigs.k8s.io/}" ;;
    *) echo "$pkg" ;;
  esac
}

# Find all go.mod files in the repo (excluding vendor and gitignored dirs)
find_gomods() {
  git ls-files --cached --others --exclude-standard '*/go.mod' 'go.mod' 2>/dev/null || \
    find . -name go.mod -not -path '*/vendor/*'
}

# Remove artifacts added by go mod tidy from all go.mod files:
# - toolchain directive (Shipyard image controls the build toolchain;
#   uses sed because go mod edit has no -droptoolchain flag)
# - consecutive blank lines (tidy sometimes adds extra whitespace)
clean_gomod() {
  local gomod
  while IFS= read -r gomod; do
    sed -i '/^toolchain/d' "$gomod"
    sed -i '/^$/{N;/^\n$/s/\n//;}' "$gomod"
  done < <(find_gomods)
}

# Insert an ignore entry into a .grype.yaml file.
# Inserts before exclude: section if present, otherwise appends.
# Usage: insert_grype_ignore FILE CVE_ID PACKAGE REASON
insert_grype_ignore() {
  local file="$1" cve_id="$2" package="$3" reason="$4"
  local entry
  entry="  # $reason
  - vulnerability: $cve_id
    package:
      name: $package"

  if grep -qn '^exclude:' "$file" 2>/dev/null; then
    local exclude_line
    exclude_line=$(grep -n '^exclude:' "$file" | head -1 | cut -d: -f1)
    {
      head -n "$((exclude_line - 1))" "$file"
      printf '%s\n' "$entry"
      tail -n +"$exclude_line" "$file"
    } > "${file}.tmp"
    mv "${file}.tmp" "$file"
  else
    printf '\n%s\n' "$entry" >> "$file"
  fi
}

# Print a section header
banner() {
  echo ""
  echo "--- $1 ---"
}
