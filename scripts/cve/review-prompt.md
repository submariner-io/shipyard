# CVE Fix Review: ${REPO} ${BRANCH}

You are reviewing CVE fix results for ${REPO} on ${BRANCH}.
The deterministic phase has already attempted to fix all CVEs.
All evidence has been pre-fetched below.

## Fix Results

${FIX_SUMMARY}

## Current Scan

${CURRENT_SCAN}

## Unfixed CVE Details

${LOCATE_OUTPUT}

## Build Environment

Go version in Shipyard build image: ${SHIPYARD_GO_VERSION}

## Task

Review ALL CVE outcomes:

1. **Verify fixes**: Do the committed fixes look correct? Any concerns?

2. **Handle unfixed CVEs**: For each CVE that was not fixed:
   - Try a different approach if possible (different version, drop replace directive)
   - If fix would break the branch: add to .grype.yaml ignore list.
     **Batch all CVEs for the same package into one ignore.sh call** (it accepts multiple CVE IDs).
     Run: `bash ${CVE_SCRIPTS}/ignore.sh ${STATE_FILE} PACKAGE SEVERITY "reason" CVE_ID [CVE_ID...]`
   - Note anything that needs team discussion

3. **Check for regressions**: Did any fix introduce new CVEs?

## Available Actions

ONLY use these scripts. Do NOT run go/git/sed commands directly.

- `bash ${CVE_SCRIPTS}/fix-package.sh ${STATE_FILE} PACKAGE VERSION CVE_IDS...`
- `bash ${CVE_SCRIPTS}/fix-stdlib.sh ${STATE_FILE} GO_VERSION CVE_IDS...`
- `bash ${CVE_SCRIPTS}/ignore.sh ${STATE_FILE} PACKAGE SEVERITY "reason" CVE_ID [CVE_ID...]`
- `bash ${CVE_SCRIPTS}/scan.sh ${STATE_FILE}`

If a script exits with NEEDS_REVIEW, do NOT attempt the same fix manually.
Use ignore.sh to document it, or report it as UNRESOLVED.

## Output

End with a summary:

```text
FIXED: N packages (list)
IGNORED: M packages (list with reasons)
UNRESOLVED: P packages (list — need team input)
```
