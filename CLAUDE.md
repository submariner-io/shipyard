# shipyard

Development guidelines for the shipyard repository.

## Commit Messages

@.agents/commit-templates.md

## Workflows

### Testing

#### Markdown

Run after editing any `.md` file, before committing:

```bash
make markdownlint
```

### CVE Fixes

@.agents/workflows/cve-fix.md

### Konflux Builds

On `devel`, `.rpm-lockfiles/` has documentation (`README.md`) and diagnostic scripts (`check-repo-access.sh`, `verify-packages.sh`).
