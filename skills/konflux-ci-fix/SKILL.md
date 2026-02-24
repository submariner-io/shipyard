---
name: konflux-ci-fix
description: Diagnose and fix Konflux CI failures in Submariner repositories. Supports PR-based and branch-based execution. Arguments are optional and order-independent.
version: 1.0.0
argument-hint: "[branch|PR-###] [repo]"
user-invocable: true
allowed-tools: Bash, Read, Edit, Grep, Glob
context: fork
---

# Konflux CI Fix Workflow

Diagnose and fix Konflux CI failures by updating Tekton task references to latest versions.

**Usage:**

- `/konflux-ci-fix` - current repo, current branch
- `/konflux-ci-fix 0.21` - current repo, specified branch (short form)
- `/konflux-ci-fix PR-1234` - current repo, specific PR
- `/konflux-ci-fix ../submariner-operator` - specified repo, current branch
- `/konflux-ci-fix release-0.21 ../submariner-operator` - both specified (order doesn't matter)
- `/konflux-ci-fix PR-1234 ../lighthouse` - PR and repo (order doesn't matter)

**Arguments** (both optional, order-independent):

- `branch|PR-###`: Branch name (e.g., `0.21`, `release-0.21`) or PR number (e.g., `PR-1234`, `pr-1234`)
- `repo`: Path to repository (starts with `/`, `./`, `../`, `~/`, or is existing directory)

---

## Step 0: Prerequisites Check

**REQUIRED**: Verify tools and authentication. If this step fails, do not proceed to subsequent steps.

```bash
MISSING_TOOLS=()

command -v gh &>/dev/null || MISSING_TOOLS+=("gh")
command -v oc &>/dev/null || MISSING_TOOLS+=("oc")
command -v jq &>/dev/null || MISSING_TOOLS+=("jq")
command -v curl &>/dev/null || MISSING_TOOLS+=("curl")
command -v sha256sum &>/dev/null || command -v shasum &>/dev/null || MISSING_TOOLS+=("sha256sum")

if [ "${#MISSING_TOOLS[@]}" -gt 0 ]; then
  echo "ERROR: Missing required tools: ${MISSING_TOOLS[*]}"
  echo ""
  echo "Installation instructions:"
  for tool in "${MISSING_TOOLS[@]}"; do
    case "$tool" in
      gh) echo "  gh: https://cli.github.com/manual/installation" ;;
      oc) echo "  oc: https://docs.openshift.com/container-platform/latest/cli_reference/openshift_cli/getting-started-cli.html" ;;
      jq) echo "  jq: https://jqlang.github.io/jq/download/" ;;
      curl) echo "  curl: included in most systems" ;;
      sha256sum) echo "  sha256sum (Linux) or shasum -a 256 (macOS): included in most systems" ;;
    esac
  done
  exit 1
fi

# Check bash version (works in both interactive and non-interactive shells)
BASH_MAJOR=$(bash -c 'echo ${BASH_VERSINFO[0]}' 2>/dev/null)
if [ -z "$BASH_MAJOR" ]; then
  # Fallback: parse from bash --version
  BASH_MAJOR=$(bash --version 2>/dev/null | head -1 | sed -nE 's/.*version ([0-9]+).*/\1/p')
fi

if [ -z "$BASH_MAJOR" ] || [ "$BASH_MAJOR" -lt 4 ]; then
  BASH_VER=$(bash --version 2>/dev/null | head -1 || echo "unknown")
  echo "ERROR: bash 4.0+ required (current: $BASH_VER)"
  echo "This script uses associative arrays (declare -A) which require bash 4.0+."
  echo ""
  echo "macOS users: brew install bash"
  echo "Then ensure the new bash is in your PATH before /bin/bash"
  exit 1
fi

if ! gh auth status &>/dev/null; then
  echo ""
  echo "============================================"
  echo "ERROR: Not authenticated with GitHub"
  echo "============================================"
  echo "This skill requires gh authentication."
  echo "Run: gh auth login"
  echo ""
  echo "Cannot proceed with skill execution."
  exit 1
fi

if ! oc whoami &>/dev/null; then
  echo ""
  echo "============================================"
  echo "ERROR: Not authenticated with Konflux"
  echo "============================================"
  echo "This skill requires oc authentication."
  echo "Run: oc login --web https://api.kflux-prd-rh02.0fk9.p1.openshiftapps.com:6443/"
  echo ""
  echo "Cannot proceed with skill execution."
  exit 1
fi

echo ""
echo "✓ Prerequisites verified: bash, gh, oc, jq, curl, sha256sum/shasum"
```

---

## Step 1: Detect Repository and Target

Parse arguments to determine repository path and target (branch or PR).

```bash
# Ensure prerequisites are met before proceeding
if ! oc whoami &>/dev/null || ! gh auth status &>/dev/null; then
  echo "ERROR: Prerequisites check failed or was skipped."
  echo "Authentication required. Run Step 0 first."
  exit 1
fi

read -r ARG1 ARG2 REST <<<"$ARGUMENTS"

if [ -n "$REST" ]; then
  echo "ERROR: Too many arguments. Usage: /konflux-ci-fix [branch|PR-###] [repo]"
  exit 1
fi

REPO=""
TARGET=""
PR_NUMBER=""

for arg in "$ARG1" "$ARG2"; do
  [ -z "$arg" ] && continue

  # Expand tilde for home directory paths
  arg="${arg/#\~/$HOME}"

  case "$arg" in
    [Pp][Rr]-[0-9]*)
      if [ -n "$TARGET" ]; then
        echo "ERROR: Multiple targets specified"
        exit 1
      fi
      PR_NUMBER="${arg#[Pp][Rr]-}"
      TARGET="PR-${PR_NUMBER}"
      ;;
    /*|./*|../*)
      if [ -n "$REPO" ]; then
        echo "ERROR: Multiple repositories specified"
        exit 1
      fi
      REPO="$arg"
      ;;
    *)
      if [ -d "$arg" ]; then
        if [ -n "$REPO" ]; then
          echo "ERROR: Multiple repositories specified"
          exit 1
        fi
        REPO="$arg"
      else
        if [ -n "$TARGET" ]; then
          echo "ERROR: Multiple targets specified"
          exit 1
        fi
        TARGET="$arg"
      fi
      ;;
  esac
done

REPO="${REPO:-.}"

if [ ! -d "$REPO" ]; then
  echo "ERROR: Repository not found: $REPO"
  exit 1
fi

if ! git -C "$REPO" rev-parse --git-dir &>/dev/null; then
  echo "ERROR: Not a git repository: $REPO"
  exit 1
fi

# Change to repository directory
if [ ! "$REPO" = "." ]; then
  cd "$REPO" || {
    echo "ERROR: Cannot change to directory: $REPO"
    exit 1
  }
  echo "Working in repository: $REPO"
fi

# Determine base branch
if [ -n "$PR_NUMBER" ]; then
  BASE_BRANCH=$(gh pr view "$PR_NUMBER" --json baseRefName --jq '.baseRefName' 2>/dev/null)
  if [ -z "$BASE_BRANCH" ]; then
    echo "ERROR: Could not get base branch for PR #$PR_NUMBER"
    exit 1
  fi
  echo "Target: PR #$PR_NUMBER (base: $BASE_BRANCH)"
else
  if [ -z "$TARGET" ]; then
    BASE_BRANCH=$(git branch --show-current 2>/dev/null)
    if [ -z "$BASE_BRANCH" ]; then
      echo "ERROR: Not on a branch (detached HEAD). Specify branch or PR explicitly."
      exit 1
    fi
    echo "Using current branch: $BASE_BRANCH"
  else
    BASE_BRANCH="$TARGET"
  fi

  # Normalize short version to full branch name (0.22 → release-0.22)
  case "$BASE_BRANCH" in
    [0-9].[0-9]|[0-9].[0-9][0-9]|[0-9][0-9].[0-9]|[0-9][0-9].[0-9][0-9])
      BASE_BRANCH="release-${BASE_BRANCH}"
      echo "Normalized to branch: $BASE_BRANCH"
      ;;
  esac

  echo "Target: branch $BASE_BRANCH"
fi

# Save original branch/commit for cleanup
ORIGINAL_REF=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
if [ "$ORIGINAL_REF" = "HEAD" ]; then
  ORIGINAL_REF=$(git rev-parse HEAD)
fi
echo "$ORIGINAL_REF" > /tmp/konflux-original-ref.txt

# Save BASE_BRANCH and PR_NUMBER for use in subsequent steps
echo "$BASE_BRANCH" > /tmp/konflux-base-branch.txt
echo "${PR_NUMBER:-}" > /tmp/konflux-pr-number.txt

# Fetch to ensure we have latest remote refs
if ! git fetch 2>/dev/null; then
  echo "WARNING: git fetch failed. Continuing with cached remote state."
fi

# Verify .tekton directory exists on target branch
if ! git ls-tree -d "origin/$BASE_BRANCH" .tekton &>/dev/null; then
  echo "ERROR: No .tekton directory found on $BASE_BRANCH"
  echo "This branch doesn't appear to have Konflux CI configured."
  echo ""
  echo "Note: .tekton files must exist on the target branch, not necessarily on your current branch."
  exit 1
fi
```

---

## Step 2: Query Snapshots and Diagnose

Query Konflux for the most recent snapshot and extract diagnostic information.

```bash
BASE_BRANCH=$(cat /tmp/konflux-base-branch.txt 2>/dev/null)
PR_NUMBER=$(cat /tmp/konflux-pr-number.txt 2>/dev/null)

echo ""
echo "=== Querying Konflux Snapshots ==="

if [ -n "$PR_NUMBER" ]; then
  oc get snapshots -n submariner-tenant \
    -l "pac.test.appstudio.openshift.io/pull-request=${PR_NUMBER}" \
    --sort-by=.metadata.creationTimestamp \
    -o json > /tmp/snapshots.json
else
  # Query by branch - normalize branch name for label (release-0.21 → 0-21)
  BRANCH_LABEL="${BASE_BRANCH#release-}"
  BRANCH_LABEL="${BRANCH_LABEL//./-}"
  oc get snapshots -n submariner-tenant \
    -l "appstudio.openshift.io/application=submariner-${BRANCH_LABEL}" \
    --sort-by=.metadata.creationTimestamp \
    -o json > /tmp/snapshots.json
fi

# Get most recent snapshot (last in sorted list)
SNAPSHOT=$(jq -r '.items[-1].metadata.name' /tmp/snapshots.json 2>/dev/null)

if [ -z "$SNAPSHOT" ] || [ "$SNAPSHOT" = "null" ]; then
  if [ -n "$PR_NUMBER" ]; then
    echo "ERROR: No snapshots found for PR #$PR_NUMBER"
  else
    echo "ERROR: No snapshots found for branch: $BASE_BRANCH"
  fi
  echo "Possible causes:"
  echo "  - No builds have run yet"
  echo "  - Wrong branch name (check branch exists and has Konflux config)"
  echo "  - Wrong PR number"
  exit 1
fi

# Extract snapshot metadata
BUILD_LOGS=$(oc get snapshot "$SNAPSHOT" -n submariner-tenant \
  -o jsonpath='{.metadata.annotations.pac\.test\.appstudio\.openshift\.io/log-url}' 2>/dev/null)

oc get snapshot "$SNAPSHOT" -n submariner-tenant \
  -o jsonpath='{.metadata.annotations.test\.appstudio\.openshift\.io/status}' \
  > /tmp/test-status.json 2>/dev/null

TEST_PLR=$(jq -r '.[0].testPipelineRunName' /tmp/test-status.json 2>/dev/null)
TEST_DETAILS=$(jq -r '.[] | "\(.scenario): \(.status) - \(.details)"' /tmp/test-status.json 2>/dev/null)
OVERALL_STATUS=$(oc get snapshot "$SNAPSHOT" -n submariner-tenant \
  -o jsonpath='{.status.conditions[?(@.type=="AppStudioTestSucceeded")].message}' 2>/dev/null)

if [ -z "$TEST_PLR" ]; then
  echo "ERROR: No test pipeline run found for snapshot $SNAPSHOT"
  echo "The snapshot exists but has no test execution data."
  echo "This may indicate:"
  echo "  - Tests haven't started yet (try again later)"
  echo "  - Snapshot annotation is missing test status"
  exit 1
fi

# Save snapshot name and test pipeline run for later steps
echo "$SNAPSHOT" > /tmp/konflux-snapshot-name.txt
echo "$TEST_PLR" > /tmp/konflux-test-plr.txt
echo "$OVERALL_STATUS" > /tmp/konflux-overall-status.txt

# Display diagnostic information
echo "Snapshot: $SNAPSHOT"
echo "Build logs: $BUILD_LOGS"
echo ""
echo "Test results:"
echo "$TEST_DETAILS"
echo ""
echo "Test logs: https://konflux-ui.apps.kflux-prd-rh02.0fk9.p1.openshiftapps.com/ns/submariner-tenant/pipelinerun/$TEST_PLR/logs"
echo ""
echo "Overall status: $OVERALL_STATUS"
echo ""
```

---

## Step 3: Analyze Enterprise Contract Logs

Prompt user to download EC logs and analyze warnings and violations.

```bash
SNAPSHOT=$(cat /tmp/konflux-snapshot-name.txt 2>/dev/null)
TEST_PLR=$(cat /tmp/konflux-test-plr.txt 2>/dev/null)
BASE_BRANCH=$(cat /tmp/konflux-base-branch.txt 2>/dev/null)

echo "=== Reviewing Enterprise Contract Logs ==="
echo "Checking for warnings and violations..."
echo ""

# Get application name from snapshot for verification
VERIFY_APP=$(oc get snapshot "$SNAPSHOT" -n submariner-tenant \
  -o jsonpath='{.spec.application}' 2>/dev/null)

if [ -z "$VERIFY_APP" ]; then
  echo "ERROR: Could not extract application name from snapshot $SNAPSHOT"
  exit 1
fi

# Save for later use
echo "$VERIFY_APP" > /tmp/konflux-verify-app.txt

# Extract version from application name for display
# submariner-0-22 → 0.22
extract_version() {
  local app="$1"
  echo "$app" | sed -E 's/.*([0-9]+)-([0-9]+)$/\1.\2/'
}

EXPECTED_VERSION=$(extract_version "$VERIFY_APP")

# Extract git revisions from snapshot for log verification.
# Each snapshot has a unique set of git SHAs for its components. By comparing
# the git revisions in the EC log with those in the snapshot, we can verify
# the log is from this exact snapshot/test run, not an old log with the same version.
oc get snapshot "$SNAPSHOT" -n submariner-tenant -o json 2>/dev/null | \
  jq -r '.spec.components[].source.git.revision' 2>/dev/null | \
  sort -u > /tmp/konflux-snapshot-revisions.txt

if [ ! -s /tmp/konflux-snapshot-revisions.txt ]; then
  echo "ERROR: Could not extract git revisions from snapshot $SNAPSHOT"
  echo "Cannot verify EC logs without snapshot metadata."
  exit 1
fi

# Search for log matching current snapshot git revisions.
# Each snapshot has a unique set of git SHAs - if they match, it's the right log.
search_for_matching_log() {
  while IFS= read -r log; do
    grep '"revision":' "$log" 2>/dev/null | \
      sed -E 's/.*"revision": "([a-f0-9]{40})".*/\1/' | \
      sort -u > /tmp/konflux-log-revisions-temp.txt

    if diff -q /tmp/konflux-snapshot-revisions.txt /tmp/konflux-log-revisions-temp.txt >/dev/null 2>&1; then
      rm -f /tmp/konflux-log-revisions-temp.txt
      echo "$log"
      return 0
    fi
    rm -f /tmp/konflux-log-revisions-temp.txt
  done < <(find ~/Downloads -maxdepth 1 -name 'submariner-enterprise-*.log' -type f -print0 2>/dev/null | xargs -0 ls -t 2>/dev/null)
  return 1
}

# Find existing log matching this snapshot
LOG_FILE=""
echo "Looking for EC log matching snapshot $SNAPSHOT..."
if LOG_FILE=$(search_for_matching_log); then
  echo "Found: $(basename "$LOG_FILE")"
fi

# Download if needed
if [ -z "$LOG_FILE" ]; then
  echo ""
  echo "=== EC Log Required ==="
  echo ""
  echo "No matching log found in ~/Downloads for snapshot: $SNAPSHOT"
  echo ""
  echo "Download URL:"
  echo "  https://konflux-ui.apps.kflux-prd-rh02.0fk9.p1.openshiftapps.com/ns/submariner-tenant/applications/$VERIFY_APP/pipelineruns/$TEST_PLR/logs"
  echo ""
  echo "Instructions:"
  echo "  1. Open the URL above"
  echo "  2. Click 'Download' button"
  echo "  3. Save to ~/Downloads/"
  echo ""

  # NOTE: Agent cannot download automatically because:
  # - Konflux Web UI requires authentication (WebFetch fails)
  # - Pipeline runs are cleaned up after some time (oc commands fail)
  # - Only manual browser download is reliable

  # AGENT DECISION POINT
  # Use AskUserQuestion tool to ask user to download the log.
  # IMPORTANT: Include the download URL directly in the question text (don't assume user can see bash output).
  # URL format: https://konflux-ui.apps.kflux-prd-rh02.0fk9.p1.openshiftapps.com/ns/submariner-tenant/applications/$VERIFY_APP/pipelineruns/$TEST_PLR/logs
  # Question should include: the full URL, instruction to click Download button, save to ~/Downloads/, and ask user to click Continue when done.
  # After user confirms, search again below.

  # Search again after user has downloaded
  echo "Searching for downloaded log..."
  if ! LOG_FILE=$(search_for_matching_log); then
    echo ""
    echo "ERROR: No EC log found matching snapshot $SNAPSHOT"
    echo ""
    echo "The log must be from the current snapshot (git revisions must match)."
    echo "Please verify you downloaded from the correct URL:"
    echo "  https://konflux-ui.apps.kflux-prd-rh02.0fk9.p1.openshiftapps.com/ns/submariner-tenant/applications/$VERIFY_APP/pipelineruns/$TEST_PLR/logs"
    exit 1
  fi
  echo "Found: $(basename "$LOG_FILE")"
fi

# Log is verified (git revisions matched in search loop above)
echo "Using: $(basename "$LOG_FILE")"

# Extract report section (between "Success:" and "DEBUG OUTPUT")
sed -n '/^[[:space:]]*Success: /,/^----- DEBUG OUTPUT -----/p' "$LOG_FILE" > /tmp/ec-report-full.txt
sed '$d' /tmp/ec-report-full.txt > /tmp/ec-report.txt

# Save for verification step
cp /tmp/ec-report.txt /tmp/konflux-ec-report-before.txt

echo ""
echo "=== EC Report (release-${EXPECTED_VERSION}) ==="

# Show overall result
head -3 /tmp/ec-report.txt
echo ""

# Extract component violation/warning counts from EC report
# Args: $1=component_name, $2=report_file
get_component_counts() {
  grep -A 2 "Name: ${1}$" "$2" 2>/dev/null | \
    awk '/Violations:/ {gsub(/,/, ""); print $2, $4}'
}

# Show components in THIS repo
REPO_COMPONENTS=$(git ls-tree --name-only "origin/$BASE_BRANCH" .tekton/ 2>/dev/null | \
  grep -- '-pull-request\.yaml$' | \
  sed 's|.tekton/\(.*\)-pull-request\.yaml|\1|')

# Save for Step 9 verification
echo "$REPO_COMPONENTS" > /tmp/konflux-repo-components.txt

echo "Components in this repo:"
TOTAL_VIOLATIONS=0
for component in $REPO_COMPONENTS; do
  COUNTS=$(get_component_counts "$component" /tmp/ec-report.txt)
  read VIOLATIONS WARNINGS <<< "${COUNTS:-0 0}"
  TOTAL_VIOLATIONS=$((TOTAL_VIOLATIONS + VIOLATIONS))

  if [ "$VIOLATIONS" -gt 0 ] || [ "$WARNINGS" -gt 0 ]; then
    echo "  $component: $VIOLATIONS violations, $WARNINGS warnings"
  else
    echo "  $component: Clean ✓"
  fi
done
echo "$TOTAL_VIOLATIONS" > /tmp/konflux-total-violations.txt
echo ""

# Show tasks mentioned
FAILING_TASKS=$(grep "Term:" /tmp/ec-report.txt 2>/dev/null | awk '{print $2}' | sort -u)
if [ -n "$FAILING_TASKS" ]; then
  echo "Tasks mentioned in issues:"
  echo "$FAILING_TASKS"
fi
echo ""

# Save failing tasks for Step 6
echo "$FAILING_TASKS" > /tmp/konflux-failing-tasks.txt
```

---

## Step 4: Check for Existing Konflux Bot PRs

Check if Konflux bot has already proposed task updates that might fix the issues.

```bash
BASE_BRANCH=$(cat /tmp/konflux-base-branch.txt 2>/dev/null)

echo ""
echo "=== Checking Konflux Bot PRs ==="

BOT_PRS=$(gh pr list --base "$BASE_BRANCH" --state open --json number,title,url,author \
  --jq '.[] | select(.author.login? // "" | startswith("app/red-hat-konflux")) | "\(.number): \(.title) - \(.url)"')

if [ -n "$BOT_PRS" ]; then
  echo "Found bot PR(s):"
  echo "$BOT_PRS"
  echo ""
  echo "Bot PRs typically update task SHAs. Review if they address the issues above."
else
  echo "No open bot PRs found."
fi
echo ""

# Load status data from previous steps
OVERALL_STATUS=$(cat /tmp/konflux-overall-status.txt 2>/dev/null)
TOTAL_VIOLATIONS=$(cat /tmp/konflux-total-violations.txt 2>/dev/null)

# Display status summary
echo "=== Status Summary ==="
echo "Tests: ${OVERALL_STATUS:-Unknown}"
echo "EC Violations: ${TOTAL_VIOLATIONS:-0}"
if [ -n "$BOT_PRS" ]; then
  echo "Bot PRs: Open (see above)"
else
  echo "Bot PRs: None"
fi
echo ""

# AGENT DECISION POINT
# Ask user: "Proceed with creating fix PR (update task versions and SHAs)?"
# Recommendation logic:
#   - If OVERALL_STATUS="True" AND TOTAL_VIOLATIONS=0 AND no BOT_PRS:
#     Suggest: No (everything is clean, likely bot PR was recently merged)
#   - Otherwise:
#     Suggest: Yes (there are issues to fix)
```

---

## Step 5: Create Fix Branch

Create a fix branch from the base branch.

```bash
BASE_BRANCH=$(cat /tmp/konflux-base-branch.txt 2>/dev/null)

echo "=== Creating Fix Branch ==="
echo ""

# Create fix branch name
DATE=$(date +%Y-%m-%d)
VERSION="${BASE_BRANCH#release-}"
FIX_BRANCH="fix-${VERSION}-konflux-${DATE}"

# Add suffix if branch exists (-v2, -v3, etc.)
FIX_BRANCH_FULL="$FIX_BRANCH"
if git show-ref --verify --quiet refs/heads/"$FIX_BRANCH"; then
  NUM=2
  while git show-ref --verify --quiet refs/heads/"${FIX_BRANCH}-v${NUM}"; do
    NUM=$((NUM + 1))
  done
  FIX_BRANCH_FULL="${FIX_BRANCH}-v${NUM}"
fi

# Create branch from origin
if ! git checkout -b "$FIX_BRANCH_FULL" "origin/$BASE_BRANCH" 2>/dev/null; then
  echo "ERROR: Could not create fix branch from origin/$BASE_BRANCH"
  echo "Branch may not exist remotely."
  exit 1
fi

echo "Created fix branch: $FIX_BRANCH_FULL"
echo ""
```

---

## Step 6: Analyze and Update Task Versions

Check current task versions against latest, update versions, and run pipeline patcher for SHA updates.

```bash
# Pipeline patcher constants (for SHA verification later)
# To update: curl -sL https://raw.githubusercontent.com/simonbaird/konflux-pipeline-patcher/${NEW_SHA}/pipeline-patcher | sha256sum
PATCHER_SHA="b001763bb1cd0286a894cfb570fe12dd7f4504bd"
EXPECTED_SHA256="080ad5d7cf7d0cee732a774b7e4dda0e2ccf26b58e08a8516a3b812bc73beb53"

echo "=== Analyzing Task Versions ==="
echo ""

# Get list of tasks to check (from EC report or all tasks in .tekton files)
FAILING_TASKS=$(cat /tmp/konflux-failing-tasks.txt 2>/dev/null)

if [ -z "$FAILING_TASKS" ]; then
  echo "No specific failing tasks identified. Checking all tasks in .tekton files..."
  FAILING_TASKS=$(grep -h "quay.io/konflux-ci/tekton-catalog/task-" .tekton/*.yaml 2>/dev/null | \
    sed 's/.*task-\([^:]*\):.*/\1/' | sort -u)
fi

# Track which tasks need updates
declare -A TASK_UPDATES

for TASK in $FAILING_TASKS; do
  CURRENT_VERSION=$(grep "task-${TASK}:" .tekton/*.yaml 2>/dev/null | head -1 | \
    sed 's/.*task-[^:]*:\([0-9.]*\).*/\1/')

  if [ -z "$CURRENT_VERSION" ]; then
    echo "  $TASK: not found in .tekton files (skipping)"
    continue
  fi

  LATEST_VERSION=$(curl -sL "https://quay.io/api/v1/repository/konflux-ci/tekton-catalog/task-${TASK}/tag/" 2>/dev/null | \
    jq -r '.tags[].name' 2>/dev/null | \
    grep -E "^[0-9]+\.[0-9]+$" | \
    sort -Vu | \
    tail -1)

  if [ -z "$LATEST_VERSION" ]; then
    echo "  $TASK: could not query latest version (API error)"
    continue
  fi

  if [ ! "$CURRENT_VERSION" = "$LATEST_VERSION" ]; then
    echo "  $TASK: $CURRENT_VERSION → $LATEST_VERSION (update needed)"
    TASK_UPDATES["$TASK"]="$CURRENT_VERSION → $LATEST_VERSION"

    # Use portable sed -i for GNU/BSD compatibility
    for yaml_file in .tekton/*.yaml; do
      [ -f "$yaml_file" ] || continue
      if grep -q "task-${TASK}:${CURRENT_VERSION}" "$yaml_file" 2>/dev/null; then
        sed -i.bak "s/task-${TASK}:${CURRENT_VERSION}/task-${TASK}:${LATEST_VERSION}/g" "$yaml_file"
        rm -f "${yaml_file}.bak"
      fi
    done
  else
    echo "  $TASK: $CURRENT_VERSION (already latest)"
  fi
done

echo ""

# Display update summary
if [ "${#TASK_UPDATES[@]}" -gt 0 ]; then
  echo "Task version updates applied:"
  for task in "${!TASK_UPDATES[@]}"; do
    echo "  $task: ${TASK_UPDATES[$task]}"
  done
  echo ""
else
  echo "All task versions already up to date. Only SHAs will be updated."
  echo ""
fi

# Run pipeline patcher to update SHAs
echo "=== Updating Task SHAs ==="
echo ""
echo "Downloading pipeline patcher..."
SCRIPT=$(curl -sL "https://raw.githubusercontent.com/simonbaird/konflux-pipeline-patcher/${PATCHER_SHA}/pipeline-patcher" 2>/dev/null)

if [ -z "$SCRIPT" ]; then
  echo "ERROR: Failed to download pipeline patcher script"
  exit 1
fi

# Verify checksum for security
if command -v sha256sum &>/dev/null; then
  ACTUAL_SHA256=$(printf %s "$SCRIPT" | sha256sum | cut -d' ' -f1)
else
  ACTUAL_SHA256=$(printf %s "$SCRIPT" | shasum -a 256 | cut -d' ' -f1)
fi

if [ ! "$ACTUAL_SHA256" = "$EXPECTED_SHA256" ]; then
  echo "ERROR: Pipeline patcher checksum mismatch!"
  echo "Expected: $EXPECTED_SHA256"
  echo "Actual:   $ACTUAL_SHA256"
  echo "This may indicate a security issue. Aborting."
  exit 1
fi

echo "Checksum verified. Running pipeline patcher..."
printf %s "$SCRIPT" | bash -s bump-task-refs

echo ""
echo "Task references updated successfully"
```

---

## Step 7: Commit Changes

Stage .tekton files and create commit.

```bash
echo ""
echo "=== Committing Changes ==="
echo ""

# Stage all .tekton files
git add .tekton/*.yaml

# Display what will be committed
echo "Files to be committed:"
git diff --staged --stat
echo ""

# Verify expected files are staged
STAGED_FILES=$(git diff --staged --name-only)
if [ -z "$STAGED_FILES" ]; then
  echo "WARNING: No changes to commit. Task refs may already be up to date."
  echo "Check EC report to see if violations are due to other issues."
  echo ""
  FIX_BRANCH_FULL=$(git rev-parse --abbrev-ref HEAD)
  ORIGINAL_REF=$(cat /tmp/konflux-original-ref.txt 2>/dev/null)
  if [ -n "$ORIGINAL_REF" ]; then
    git checkout "$ORIGINAL_REF" 2>/dev/null
    git branch -D "$FIX_BRANCH_FULL" 2>/dev/null
    echo "Deleted empty fix branch: $FIX_BRANCH_FULL"
  fi
  exit 0
fi

if ! echo "$STAGED_FILES" | grep -q "^.tekton/"; then
  echo "WARNING: Staged changes include files outside .tekton/ directory"
  echo "This is unexpected. Review changes carefully."
  echo ""
fi

# Create commit with standard message
git commit -s -m "Update Tekton task refs to latest versions"

echo "Commit created successfully"
echo ""
```

---

## Step 8: Generate PR Commands

Extract variables and generate PR command for user to execute.

```bash
# Load BASE_BRANCH from Step 1 (single source of truth)
BASE_BRANCH=$(cat /tmp/konflux-base-branch.txt 2>/dev/null)

echo "=== Pull Request Command ==="
echo ""

# Extract variables
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
# Find fork remote (not submariner-io)
FORK_REMOTE=$(git remote -v | grep '(push)' | grep -v 'submariner-io' | head -1 | awk '{print $1}')
# Extract GitHub username from fork URL (handles both SSH and HTTPS formats)
if [ -n "$FORK_REMOTE" ]; then
  FORK_USER=$(git remote get-url "${FORK_REMOTE}" 2>/dev/null | sed -E 's#.*github.com[:/]+([^/]+)/.*#\1#')
else
  FORK_USER=""
fi

if [ -z "$FORK_REMOTE" ] || [ -z "$FORK_USER" ]; then
  REPO_NAME=$(basename "$(pwd)")
  echo "WARNING: Could not detect fork remote. You may need to add your fork as a remote."
  echo "Example: git remote add fork git@github.com:YOUR_USERNAME/${REPO_NAME}.git"
  echo ""
  FORK_REMOTE="<YOUR_FORK_REMOTE>"
  FORK_USER="<YOUR_GITHUB_USERNAME>"
fi

# Save fix branch name for Step 9 (user might switch branches before verification)
echo "$CURRENT_BRANCH" > /tmp/konflux-fix-branch.txt

# Display PR command (don't execute due to SSH auth requirements)
echo "Copy and run the following command to create the pull request:"
echo ""
echo "git push $FORK_REMOTE $CURRENT_BRANCH && \\"
echo "gh pr create \\"
echo "  --title \"Fix Konflux CI failures in $BASE_BRANCH\" \\"
echo "  --body \"Update Tekton task refs to latest versions\" \\"
echo "  --base \"$BASE_BRANCH\" \\"
echo "  --head \"$FORK_USER:$CURRENT_BRANCH\" \\"
echo "  --assignee \"@me\""
echo ""
```

---

## Step 9: Verification (Optional)

After PR is created, poll for new snapshot and verify fix effectiveness.

```bash
echo "=== Verification (Optional) ==="
echo ""
echo "After creating the PR, you can verify the fix by checking the new build."
echo ""
read -p "Have you created the PR? [y/N] " -n 1 -r
echo

if [ ! "$REPLY" = "y" ] && [ ! "$REPLY" = "Y" ]; then
  echo "Verification skipped. Run /konflux-ci-fix again later to verify."
  exit 0
fi

# Load from Step 1 and Step 8 (in case user switched branches)
BASE_BRANCH=$(cat /tmp/konflux-base-branch.txt 2>/dev/null)
CURRENT_BRANCH=$(cat /tmp/konflux-fix-branch.txt 2>/dev/null)

# Fallback if /tmp cleared (rare)
if [ -z "$CURRENT_BRANCH" ]; then
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi

# Get PR number
PR_NUMBER=$(gh pr list --head "$CURRENT_BRANCH" --base "$BASE_BRANCH" \
  --json number --jq '.[0].number' 2>/dev/null)

if [ -z "$PR_NUMBER" ]; then
  echo "ERROR: Could not find PR for branch $CURRENT_BRANCH"
  echo "Make sure you've created the PR first."
  exit 1
fi

echo "Checking PR #$PR_NUMBER"
echo ""

# Poll for new snapshot (max 10 minutes, 30-second intervals)
MAX_ATTEMPTS=20
ATTEMPT=0
SNAPSHOT=""

echo "Waiting for new snapshot..."
while [ "$ATTEMPT" -lt "$MAX_ATTEMPTS" ]; do
  oc get snapshots -n submariner-tenant \
    -l "pac.test.appstudio.openshift.io/pull-request=${PR_NUMBER}" \
    --sort-by=.metadata.creationTimestamp \
    -o json > /tmp/snapshots-verify.json 2>/dev/null

  SNAPSHOT=$(jq -r '.items[-1].metadata.name' /tmp/snapshots-verify.json 2>/dev/null)

  if [ -n "$SNAPSHOT" ] && ! [ "$SNAPSHOT" = "null" ]; then
    OVERALL_STATUS=$(oc get snapshot "$SNAPSHOT" -n submariner-tenant \
      -o jsonpath='{.status.conditions[?(@.type=="AppStudioTestSucceeded")].status}' 2>/dev/null)

    if [ "$OVERALL_STATUS" = "True" ] || [ "$OVERALL_STATUS" = "False" ]; then
      echo "Build complete!"
      break
    fi
  fi

  ATTEMPT=$((ATTEMPT + 1))
  echo "  Attempt $ATTEMPT/$MAX_ATTEMPTS - waiting 30s..."
  sleep 30
done

if [ -z "$SNAPSHOT" ] || [ "$SNAPSHOT" = "null" ]; then
  echo ""
  echo "No snapshot found after waiting."
  echo "Build may not have started yet. Try again later."
  echo ""
  echo "If build doesn't start automatically, you may need to trigger a retest:"
  echo "  /retest branch:$BASE_BRANCH"
  echo "(Comment this on the merge commit in the PR)"
  exit 0
fi

# Display new snapshot info
echo ""
echo "New snapshot: $SNAPSHOT"
echo "Status: $OVERALL_STATUS"

# Identify which component triggered this build
BUILD_LOGS=$(oc get snapshot "$SNAPSHOT" -n submariner-tenant \
  -o jsonpath='{.metadata.annotations.pac\.test\.appstudio\.openshift\.io/log-url}' 2>/dev/null)
COMPONENT=$(echo "$BUILD_LOGS" | sed -nE 's#.*/pipelinerun/([^/]+)-on-(pull-request|push).*#\1#p' 2>/dev/null)
if [ -n "$COMPONENT" ]; then
  echo "Component built: $COMPONENT"
fi
echo ""

# Get test logs URL
oc get snapshot "$SNAPSHOT" -n submariner-tenant \
  -o jsonpath='{.metadata.annotations.test\.appstudio\.openshift\.io/status}' \
  > /tmp/test-status-verify.json 2>/dev/null

TEST_PLR=$(jq -r '.[0].testPipelineRunName' /tmp/test-status-verify.json 2>/dev/null)

if [ -z "$TEST_PLR" ]; then
  echo "WARNING: No test pipeline run found for new snapshot $SNAPSHOT"
  echo "Cannot provide test logs URL. Skipping EC log verification."
  exit 0
fi

echo "Test logs: https://konflux-ui.apps.kflux-prd-rh02.0fk9.p1.openshiftapps.com/ns/submariner-tenant/pipelinerun/$TEST_PLR/logs"
echo ""

# Prompt for new EC log
echo "To verify the fix, download the new EC log:"
echo "  1. Open the test logs URL above"
echo "  2. Click the 'Download' link"
echo "  3. Save to ~/Downloads/ (filename: submariner-enterprise-*.log)"
echo ""
read -p "Press Enter after downloading the new EC log..."
echo ""

# Analyze new EC log
NEW_LOG_FILE=$(ls -t ~/Downloads/submariner-enterprise-*.log 2>/dev/null | head -1)
if [ -z "$NEW_LOG_FILE" ]; then
  echo "WARNING: No new EC log found. Skipping comparison."
  exit 0
fi

echo "Using new EC log: $(basename "$NEW_LOG_FILE")"

# Extract new report
sed -n '/^[[:space:]]*Success: /,/^----- DEBUG OUTPUT -----/p' "$NEW_LOG_FILE" > /tmp/ec-report-new-full.txt
sed '$d' /tmp/ec-report-new-full.txt > /tmp/ec-report-new.txt

echo ""
echo "=== Verification Results ==="
echo ""

# Overall result
echo "New EC status:"
head -3 /tmp/ec-report-new.txt
echo ""

# Extract component violation/warning counts from EC report
# Args: $1=component_name, $2=report_file
get_component_counts() {
  grep -A 2 "Name: ${1}$" "$2" 2>/dev/null | \
    awk '/Violations:/ {gsub(/,/, ""); print $2, $4}'
}

# Compare violations and warnings per component
echo "Component comparison (before → after):"
while IFS= read -r component; do
  COUNTS_BEFORE=$(get_component_counts "$component" /tmp/konflux-ec-report-before.txt)
  read V_BEFORE W_BEFORE <<< "${COUNTS_BEFORE:-0 0}"

  COUNTS_AFTER=$(get_component_counts "$component" /tmp/ec-report-new.txt)
  read V_AFTER W_AFTER <<< "${COUNTS_AFTER:-0 0}"

  TOTAL_BEFORE=$((V_BEFORE + W_BEFORE))
  TOTAL_AFTER=$((V_AFTER + W_AFTER))

  if [ "$TOTAL_AFTER" -eq 0 ] && [ "$TOTAL_BEFORE" -eq 0 ]; then
    echo "  $component: ${V_BEFORE}v+${W_BEFORE}w → ${V_AFTER}v+${W_AFTER}w (clean)"
  elif [ "$TOTAL_AFTER" -eq 0 ]; then
    echo "  $component: ${V_BEFORE}v+${W_BEFORE}w → ${V_AFTER}v+${W_AFTER}w ✓ (fixed)"
  elif [ "$TOTAL_AFTER" -lt "$TOTAL_BEFORE" ]; then
    echo "  $component: ${V_BEFORE}v+${W_BEFORE}w → ${V_AFTER}v+${W_AFTER}w ✓ (improved)"
  elif [ "$TOTAL_AFTER" -eq "$TOTAL_BEFORE" ]; then
    echo "  $component: ${V_BEFORE}v+${W_BEFORE}w → ${V_AFTER}v+${W_AFTER}w (unchanged)"
  else
    echo "  $component: ${V_BEFORE}v+${W_BEFORE}w → ${V_AFTER}v+${W_AFTER}w ✗ (worse)"
  fi
done < /tmp/konflux-repo-components.txt
echo ""

echo "Verification complete!"
```

---

## Summary (Return Value)

When complete, provide a summary including:

1. **Repository**: Path to repository (if not current directory)
2. **Branch**: Fix branch name and base branch
3. **Target**: PR number or branch that was diagnosed
4. **Existing Bot PRs**: List any bot PRs found (if user chose to continue)
5. **Diagnostic Results**:
   - Snapshot name
   - EC status (Pass/Fail)
   - Failing tasks identified
6. **Tasks Updated**: List of tasks updated with version changes (or note if only SHAs updated)
7. **PR Command**: The complete command to create the pull request
8. **Verification Results** (if completed):
   - New snapshot name
   - Component built (which component triggered this snapshot)
   - New EC status
   - Component comparison (violations + warnings, before → after)
   - Note: Other repo components may still show issues if they haven't rebuilt yet from this PR
9. **Status**: Success, partial success, or blocked
10. **Warnings**: Any warnings (e.g., auth issues, etc.)

Example summary:

```text
## Konflux CI Fix Complete: release-0.21

**Repository:** ../submariner-operator
**Branch:** fix-0.21-konflux-2026-02-10
**Target:** PR #1234
**Status:** Success

### Existing Bot PRs
- #3528: Red Hat Konflux update submariner-operator-0-21 (user chose to continue)

### Diagnostic Results
- Snapshot: submariner-0-21-abc123-on-pull-request-xyz
- EC Status: Failed
- Failing Tasks: buildah-remote-oci-ta, prefetch-dependencies-oci-ta

### Tasks Updated
- buildah-remote-oci-ta: 0.4 → 0.6
- prefetch-dependencies-oci-ta: 0.2 → 0.3
- git-clone-oci-ta: 0.1 → 0.1 (SHA only)

### PR Command
git push fork fix-0.21-konflux-2026-02-10 && \
gh pr create \
  --title "Fix Konflux CI failures in release-0.21" \
  --body "Update Tekton task refs to latest versions" \
  --base "release-0.21" \
  --head "dfarrell07:fix-0.21-konflux-2026-02-10" \
  --assignee "@me"

### Verification
- New snapshot: submariner-0-21-def456-on-pull-request-xyz
- Component built: submariner-operator-0-21
- New EC status: Passed
- Component comparison:
  - submariner-operator-0-21: 10v+2w → 0v+0w ✓ (fixed)
  - submariner-bundle-0-21: 3v+0w → 3v+0w (unchanged - not rebuilt in this PR)
```

---

## Common Issues

| Issue | Solution |
| ----- | -------- |
| No .tekton directory | .tekton files must exist on target branch (e.g., release-0.22), not current branch |
| No snapshots found | Check target (PR/branch) exists and has Konflux builds; verify branch name format |
| Auth failures | Run `gh auth login` or `oc login --web https://api.kflux-prd-rh02.0fk9.p1.openshiftapps.com:6443/` |
| EC log not found | Download manually from Konflux UI test logs; save to ~/Downloads/ |
| Pipeline patcher checksum fails | Security issue - do not proceed; report to team |
| Violations persist after fix | Check EC report for other error types (not just outdated tasks); may need manual fixes |
| Build doesn't start | Comment `/retest branch:release-0.X` on merge commit in PR |
| No changes to commit | Task versions already up to date; check if violations are from other issues |
