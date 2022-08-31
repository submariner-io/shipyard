<!-- markdownlint-disable MD041 -->
Deploying using Shipyard now uses specific image overrides determined by the `PRELOAD_IMAGES` variable.
For developers using `make deploy` (or `make e2e`) there won't be a noticeable change in behavior, but only those images specified by the
`PRELOAD_IMAGES` will be preloaded and used.
Developers can also specify the variable explicitly to further control which images get preloaded and used.
