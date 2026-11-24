<!-- markdownlint-disable MD041 -->
`make deploy-latest` (used by `upgrade-e2e`) now selects the highest stable patch
release in `BASE_BRANCH`'s minor stream instead of the globally latest GitHub release.

On `devel`, it still selects the highest stable release overall. On `release-X.Y`, it
selects the highest stable `vX.Y.*` from `submariner-io/releases` tags. Before a
stream's first stable release, it falls back to the preceding minor's highest stable
patch. Override with `UPGRADE_BASELINE_VERSION=vX.Y.Z` when needed.
