#!/bin/sh
set -eu

workflow=.github/workflows/native-acceptance-evidence.yml
test -f "$workflow"

require() {
    if ! grep -Fq -- "$1" "$workflow"; then
        echo "native acceptance workflow is missing required text: $1" >&2
        exit 1
    fi
}

require 'workflow_dispatch:'
require 'x86_summary_b64:'
require 'aarch64_summary_b64:'
require 'aarch64_release_record_b64:'
require "github.repository == 'LevitateOS/soda-os'"
require "github.ref == 'refs/heads/main'"
require 'permissions:'
require 'contents: read'
require 'id-token: write'
require 'cosign-release: v3.0.6'
require 'base64 --decode --strict'
require '-le 12288'
require 'go run ./cmd/soda-acceptance record'
require '--expected-revision "$GITHUB_SHA"'
require '--aarch64-release-record'
require 'https://github.com/LevitateOS/soda-os/.github/workflows/native-acceptance-evidence.yml@refs/heads/main'
require 'https://token.actions.githubusercontent.com'
require 'retention-days: 1'

if [ "$(grep -Ec '^      [a-z0-9_]+:$' "$workflow")" -ne 3 ]; then
    echo 'native acceptance workflow must accept exactly three record inputs' >&2
    exit 1
fi

if [ "$(awk '/^jobs:$/ { in_jobs = 1; next } in_jobs && /^  [A-Za-z0-9_-]+:$/ { count++ } END { print count + 0 }' "$workflow")" -ne 1 ]; then
    echo 'native acceptance workflow must contain exactly one job' >&2
    exit 1
fi

if grep -Eq '^[[:space:]]*uses:[[:space:]]*[^@[:space:]]+@[^[:space:]#]{1,39}([[:space:]#]|$)' "$workflow"; then
    echo 'native acceptance workflow contains an action that is not pinned by full commit SHA' >&2
    exit 1
fi

for forbidden in \
    'secrets.' \
    guest_key \
    auth_key \
    private_key \
    password \
    tailscale \
    qemu \
    'docker push' \
    'cosign sign ' \
    'cosign attest' \
    'gh release' \
    image-stage \
    image-promote; do
    if grep -Fiq -- "$forbidden" "$workflow"; then
        echo "native acceptance workflow contains forbidden behavior: $forbidden" >&2
        exit 1
    fi
done
