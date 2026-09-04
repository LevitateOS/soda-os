#!/bin/sh
set -eu

workflow=.github/workflows/release.yml
test -f "$workflow"

require() {
    if ! grep -Fq -- "$1" "$workflow"; then
        echo "release workflow is missing required bootstrap text: $1" >&2
        exit 1
    fi
}

require 'name: Bootstrap x86-64 fallback signature'
require 'branches:'
require '- production'
require 'permissions: {}'
require 'cancel-in-progress: false'
require 'bootstrap-fallback-x86_64:'
require "github.event_name == 'push'"
require "github.ref == 'refs/heads/production'"
require "github.repository == 'LevitateOS/soda-os'"
require 'runs-on: ubuntu-24.04'
require 'environment: production-release'
require 'contents: read'
require 'packages: write'
require 'id-token: write'
require 'sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6'
require 'cosign-release: v3.0.6'
require 'ghcr.io/levitateos/soda-os@sha256:d57060e9eb5953043e7ce18b8e002422010f6e17c1408211907d31fd1cfa5edd'
require 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production'
require 'https://token.actions.githubusercontent.com'
require 'docker pull --quiet --platform linux/amd64'
require "'{{.Os}}/{{.Architecture}}'"
require 'cosign download signature'
require 'no signatures associated'
require 'signature material exists but no signature matches the exact production workflow'
require 'signature state is unknown; refusing to sign'
require '--certificate-github-workflow-ref refs/heads/production'
require '--certificate-github-workflow-repository LevitateOS/soda-os'
require '--certificate-github-workflow-sha "$GITHUB_SHA"'
require '--certificate-github-workflow-trigger push'

job_count=$(awk '
    /^jobs:$/ { in_jobs = 1; next }
    in_jobs && /^  [A-Za-z0-9_-]+:$/ { count++ }
    END { print count + 0 }
' "$workflow")
if [ "$job_count" -ne 1 ]; then
    echo 'fallback bootstrap must contain exactly one job' >&2
    exit 1
fi

if [ "$(grep -Fc 'ghcr.io/levitateos/soda-os@sha256:' "$workflow")" -ne 1 ]; then
    echo 'fallback bootstrap must contain exactly one committed image digest' >&2
    exit 1
fi

if [ "$(grep -Fc 'cosign sign --yes "$IMAGE_REFERENCE"' "$workflow")" -ne 1 ]; then
    echo 'fallback bootstrap must contain exactly one signing operation' >&2
    exit 1
fi

for forbidden in \
    workflow_dispatch \
    'inputs:' \
    'matrix:' \
    aarch64 \
    ubuntu-24.04-arm \
    'contents: write' \
    'secrets.' \
    soda-release \
    soda-release-executor \
    tailscale \
    'go run' \
    'just ' \
    'gh release' \
    image-stage \
    image-promote \
    'cosign attest' \
    actions/checkout \
    actions/upload-artifact \
    actions/download-artifact \
    '0.5.0-' \
    __PENDING; do
    if grep -Fq -- "$forbidden" "$workflow"; then
        echo "fallback bootstrap contains forbidden operation or placeholder: $forbidden" >&2
        exit 1
    fi
done

if grep -Eq '^[[:space:]]*uses:[[:space:]]*[^@[:space:]]+@[^[:space:]#]{1,39}([[:space:]#]|$)' "$workflow"; then
    echo 'release workflow contains an action that is not pinned by full commit SHA' >&2
    exit 1
fi

if grep -Eqi '(TS_AUTHKEY|tailscale.*authkey|authkey:)' "$workflow"; then
    echo 'release CI must not contain a reusable authentication key' >&2
    exit 1
fi
