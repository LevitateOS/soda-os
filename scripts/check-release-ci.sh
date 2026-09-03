#!/bin/sh
set -eu

workflow=.github/workflows/release.yml
test -f "$workflow"
test -x scripts/release-create-vm-auth-key.sh
test -x scripts/soda-release-executor

grep -Fq 'branches:' "$workflow"
grep -Fq -- '- production' "$workflow"
grep -Fq 'cancel-in-progress: false' "$workflow"
grep -Fq 'id-token: write' "$workflow"
grep -Fq 'packages: write' "$workflow"
grep -Fq 'contents: write' "$workflow"
grep -Fq 'ubuntu-24.04-arm' "$workflow"
grep -Fq 'tag:soda-release-ci' "$workflow"
grep -Fq 'soda-release-executor' "$workflow"
grep -Fq 'prepare) prepare "$@"' scripts/soda-release-executor
grep -Fq 'emit-record) emit_record "$@"' scripts/soda-release-executor
grep -Fq 'emit-bundle) emit_bundle "$@"' scripts/soda-release-executor
grep -Fq 'promote) promote "$@"' scripts/soda-release-executor
grep -Fq 'upload) upload "$@"' scripts/soda-release-executor
grep -Fq 'needs: promote' "$workflow"
grep -Fq 'needs: draft' "$workflow"
test "$(grep -Fc 'soda-release-executor promote' "$workflow")" -eq 1
test "$(grep -Fc 'soda-release-executor emit-bundle' "$workflow")" -eq 1
test "$(grep -Fc 'soda-release-executor upload' "$workflow")" -eq 1
test "$(grep -Fc 'soda-release-record-bundle-' "$workflow")" -eq 3
grep -Fq 'SSH command must invoke soda-release-executor' scripts/soda-release-executor
grep -Fq 'PATH=/usr/local/bin:/usr/bin:/bin' scripts/soda-release-executor

if grep -Eq '^[[:space:]]*uses:[[:space:]]*[^@[:space:]]+@[^[:space:]#]{1,39}([[:space:]#]|$)' "$workflow"; then
    echo 'release workflow contains an action that is not pinned by full commit SHA' >&2
    exit 1
fi

if grep -Eq '(TS_AUTHKEY|tailscale.*authkey|authkey:)' "$workflow" scripts/release-create-vm-auth-key.sh; then
    echo 'release CI must use workload identity federation, not a reusable auth key' >&2
    exit 1
fi

if grep -Eq '(^|[[:space:]])(eval|sh -c|bash -c)([[:space:]]|$)' scripts/soda-release-executor; then
    echo 'release executor must not evaluate caller-provided commands' >&2
    exit 1
fi
