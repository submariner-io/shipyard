<!-- markdownlint-disable MD041 -->
`make deploy-latest` now selects the highest stable patch release in `BASE_BRANCH`'s minor stream.
For `devel`, it selects the highest stable release overall. Before a release branch's first stable release,
it uses the preceding minor's highest stable release and reports this choice.

Use `make deploy-latest UPGRADE_BASELINE_VERSION=v0.22.2` to choose an exact baseline explicitly.
Prereleases and draft releases are excluded from automatic selection, and API failures stop the deployment.

Published baseline deployments use a separate subctl installation ahead of local builds on PATH and verify
the installed binary, component image tags, and completed rollouts. The shared `check-deployed-version.sh`
helper waits up to 300 seconds per cluster for image reconciliation and rollouts; set `VERSION_CHECK_TIMEOUT`
to override this limit. Checks require workloads for the installed Submariner and ServiceDiscovery resources
and recheck image tags after rollouts. Consumers that build subctl before deployment should skip
that prerequisite when `RELEASED_SUBCTL=true`, while retaining their usual behavior for ordinary deployments.

Operator-based upgrade tests can set `SUBCTL_IMAGE_VERSION` to deploy and verify an explicit target image
version for both the operator and components. The `upgrade-e2e` action exposes this as its optional
`image-version` input. The published baseline phase always clears this setting.
