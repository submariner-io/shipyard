#!/bin/bash
# Kill orphaned CVE fix processes and containers.
# Safe to run anytime — only targets fix-all.sh, grype, and dapper fix containers.
pkill -f fix-all.sh 2>/dev/null || true
docker ps --format '{{.ID}} {{.Image}}' 2>/dev/null | grep -E 'grype:latest|fix-.*-cves-' | awk '{print $1}' | xargs -r docker kill 2>/dev/null || true
