#### Setting up Nettest Builds in Konflux on New Branch

**Prerequisites:**

- Configuration added in konflux-ci/build-definitions repo
- Existing Konflux-configured branch to copy files from (e.g., `release-0.21`)
- Bot has created initial `.tekton/` configuration on branch `konflux-nettest-<X-Y>`

**Placeholders:**

- `<target-branch>`: Your target branch (e.g., `release-0.22`)
- `<X-Y>`: Version with dashes (e.g., `0-22`)

**Important:** Nettest is single-component with RPM-only prefetch (no gomod - no Go code).

##### 1. Checkout Bot's PR Branch

```bash
git checkout konflux-nettest-<X-Y>
```

##### 2. Configure YAMLlint to Ignore Generated Directories

```bash
grep -q "/\.tekton" .yamllint.yml || sed -i '/^ignore: |$/a\  /.tekton' .yamllint.yml
grep -q "/\.rpm-lockfiles" .yamllint.yml || sed -i '/^ignore: |$/a\  /.rpm-lockfiles' .yamllint.yml
git add .yamllint.yml
git commit -s -m "Configure yamllint to ignore /.tekton and /.rpm-lockfiles"
```

**Note**: Uses `/` prefix (`/.tekton` not `.tekton`) to match shipyard's existing style.

##### 3. Add RPM Lockfile Support

```bash
TARGET_VERSION=$(echo "<target-branch>" | grep -oP '(?<=release-0\.)\d+$')
[ -z "$TARGET_VERSION" ] && { echo "ERROR: Invalid target branch format. Expected release-0.XX"; exit 1; }
PREV_VERSION=$((TARGET_VERSION - 1))
git checkout origin/release-0.${PREV_VERSION} -- .rpm-lockfiles/update-lockfile.sh .rpm-lockfiles/nettest/
chmod +x .rpm-lockfiles/update-lockfile.sh
.rpm-lockfiles/update-lockfile.sh nettest
ls .rpm-lockfiles/nettest/rpms.lock.yaml || { echo "ERROR: Lockfile generation failed"; exit 1; }
git add .rpm-lockfiles/
git commit -s -m "Add RPM lockfile support for nettest"
```

**Note**: Script may show `FileNotFoundError` for Dockerfile.nettest.konflux (doesn't exist yet, created in Step 4). Non-blocking.

##### 4. Add Konflux Dockerfile and Configure Tekton to Use It

```bash
# Formula: Submariner 0.X → ACM 2.(X-7), so 0.22 → 2.15
TARGET_VERSION=$(echo "<target-branch>" | grep -oP '(?<=release-0\.)\d+$')
[ -z "$TARGET_VERSION" ] && { echo "ERROR: Invalid target branch format. Expected release-0.XX"; exit 1; }
PREV_VERSION=$((TARGET_VERSION - 1))
ACM_VERSION=$((TARGET_VERSION - 7))

git checkout origin/release-0.${PREV_VERSION} -- package/Dockerfile.nettest.konflux scripts/nettest/metricsproxy.konflux
chmod +x scripts/nettest/metricsproxy.konflux
sed -i "s/release-0.${PREV_VERSION}/<target-branch>/g" package/Dockerfile.nettest.konflux
sed -i "s/cpe=\"cpe:\/a:redhat:acm:[0-9.]*::el9\"/cpe=\"cpe:\/a:redhat:acm:2.${ACM_VERSION}::el9\"/" package/Dockerfile.nettest.konflux

sed -i 's|package/Dockerfile.nettest|package/Dockerfile.nettest.konflux|g' .tekton/*.yaml
git add package/Dockerfile.nettest.konflux scripts/nettest/metricsproxy.konflux .tekton/*.yaml
git commit -s -m "Add Konflux dockerfile for nettest and configure tekton to use it"
```

**Note**: Must copy both Dockerfile AND metricsproxy.konflux (Dockerfile references it).

##### 5. Enable Hermetic Builds

```bash
if ! grep -q "^  - name: hermetic$" .tekton/*.yaml; then
  PREFETCH='[{"type": "rpm", "path": "./.rpm-lockfiles/nettest"}]'
  sed -i "/^  pipelineSpec:$/i\  - name: prefetch-input\n    value: '${PREFETCH}'\n  - name: hermetic\n    value: \"true\"" \
    .tekton/*.yaml
  git add .tekton/*.yaml
  git commit -s -m "Enable hermetic builds with RPM prefetching for nettest"
fi
```

**Note**: RPM-only prefetch (no gomod). Uses variable to meet line length requirements.

##### 6. Add Multi-Platform Support

```bash
if ! grep -q "linux/arm64" .tekton/*.yaml; then
  sed -i '/^    - linux\/x86_64$/a\    - linux/arm64' .tekton/*.yaml
  git add .tekton/*.yaml
  git commit -s -m "Add multi-platform build support for nettest"
fi
```

##### 7. Enable SBOM Generation

```bash
if ! grep -q "^  - name: build-source-image$" .tekton/*.yaml; then
  sed -i '/  - name: hermetic$/,/    value: "true"$/{/    value: "true"$/a\  - name: build-source-image\n    value: "true"
}' .tekton/*.yaml
  git add .tekton/*.yaml
  git commit -s -m "Enable SBOM generation for nettest"
fi
```

##### 8. Update Task References

```bash
bash << 'EOF'
set -e

PATCHER_SHA="b001763bb1cd0286a894cfb570fe12dd7f4504bd"
EXPECTED_SHA256="080ad5d7cf7d0cee732a774b7e4dda0e2ccf26b58e08a8516a3b812bc73beb53"

SCRIPT=$(curl -sL "https://raw.githubusercontent.com/simonbaird/konflux-pipeline-patcher/${PATCHER_SHA}/pipeline-patcher")
ACTUAL_SHA256=$(echo "$SCRIPT" | sha256sum | cut -d' ' -f1)

if [[ "$ACTUAL_SHA256" != "$EXPECTED_SHA256" ]]; then
  echo "ERROR: Script checksum mismatch!"
  exit 1
fi

echo "$SCRIPT" | bash -s bump-task-refs
EOF
git diff --quiet .tekton/*.yaml || \
  { git add .tekton/*.yaml && \
    git commit -s -m "Update Tekton task references to latest versions for nettest"; }
```

**Note**: Updates task references if outdated.

##### 9. Review and Push

```bash
git log origin/<target-branch>..HEAD
git status
git push origin konflux-nettest-<X-Y>
```

Expected: 4-8 commits (bot's initial + 3-7 from steps 2-8), clean working tree.

**Troubleshooting:**

- **Steps 5-7 skipped**: Bot may have already added hermetic, ARM64, or SBOM parameters. Expected.
- **Step 8 no commit**: Task references already up-to-date. Expected.
