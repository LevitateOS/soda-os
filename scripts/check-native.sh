#!/usr/bin/env bash
# Linux source verification and the native Docker boundary used by candidate preparation.
set -Eeuo pipefail

native_fail() {
    printf 'check-native: %s\n' "$*" >&2
    exit 1
}

native_platform() {
    local architecture=${1:-} machine
    machine=$(uname -m)
    case "$architecture:$machine" in
        aarch64:aarch64|aarch64:arm64) native_oci_arch=arm64 ;;
        x86_64:x86_64) native_oci_arch=amd64 ;;
        *) native_fail "select aarch64 or x86_64 on matching native hardware (requested $architecture, host $machine)" ;;
    esac
    native_os=$(uname -s)
    case "$native_os" in Linux|Darwin) ;; *) native_fail "unsupported build host: $native_os" ;; esac
    native_docker_platform=linux/$native_oci_arch
}

require_native_docker() {
    local context endpoint daemon builders worker worker_name
    # Bind all child Docker/buildx calls without changing the operator's configuration.
    [[ -z ${DOCKER_HOST:-} ]] || native_fail "select a local Docker context instead of DOCKER_HOST"
    context=$(docker context show)
    endpoint=$(docker context inspect "$context" | jq -er '.[0].Endpoints.docker.Host')
    [[ "$endpoint" == unix://* ]] || native_fail "candidate builds require a local Docker backend (got $endpoint)"
    export DOCKER_CONTEXT=$context
    daemon=$(docker info --format '{{json .}}')
    jq -e --arg arch "$native_oci_arch" '
        .OSType == "linux" and
        ((.Architecture == $arch) or ($arch == "arm64" and .Architecture == "aarch64") or
         ($arch == "amd64" and .Architecture == "x86_64"))
    ' <<<"$daemon" >/dev/null || native_fail "Docker daemon is not native $native_docker_platform"
    builders=$(docker buildx ls --format '{{json .}}')
    worker=$(jq -sec --arg selected "${BUILDX_BUILDER:-}" '
        unique_by(.Name) | map(select(if $selected == "" then .Current else .Name == $selected end)) |
        if length == 1 then .[0] else error("no unique selected builder") end
    ' <<<"$builders") || native_fail "cannot identify the selected Buildx worker"
    jq -e --arg context "$context" --arg endpoint "$endpoint" '
        .Driver == "docker" and (.Nodes | length == 1) and
        .Nodes[0].Status == "running" and
        (.Nodes[0].Endpoint == $context or .Nodes[0].Endpoint == $endpoint)
    ' <<<"$worker" >/dev/null || native_fail "selected Buildx worker must be a running, single integrated docker worker on the verified context"
    worker_name=$(jq -er '.Name | select(type == "string" and length > 0)' <<<"$worker") \
        || native_fail "selected Buildx worker has no name"
    export BUILDX_BUILDER=$worker_name
    printf '==> Native Docker backend: context=%s builder=%s platform=%s\n' "$DOCKER_CONTEXT" "$BUILDX_BUILDER" "$native_docker_platform" >&2
}

check_native() {
    [[ $# -eq 1 ]] || native_fail "usage: scripts/check-native.sh <aarch64|x86_64>"
    native_platform "$1"
    local root revision status go_version image
    root=$(git rev-parse --show-toplevel)
    cd "$root"
    revision=$(git rev-parse HEAD)
    status=$(git status --porcelain=v1 --untracked-files=all)
    [[ -z "$status" ]] || native_fail "source verification requires a clean exact-commit checkout"
    if [[ "$native_os" == Linux ]]; then
        just check
    else
        require_native_docker
        go_version=$(awk '$1 == "go" {print $2; exit}' go.mod)
        [[ "$go_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || native_fail "go.mod must select an exact Go version"
        image=localhost/soda-check:$1
        printf '==> Building the native Linux development check environment\n' >&2
        docker build --platform "$native_docker_platform" --build-arg "GO_VERSION=$go_version" \
            --file tools/check/Containerfile --tag "$image" .
        # Clone Git objects, not host node_modules, ignored artifacts, or generated frontend files.
        # safe.directory applies only to this disposable process and its read-only source mount.
        docker run --rm --pull=never --platform "$native_docker_platform" \
            --mount "type=bind,source=$root,target=/source,readonly" \
            --env GIT_CONFIG_COUNT=1 --env GIT_CONFIG_KEY_0=safe.directory --env GIT_CONFIG_VALUE_0=/source \
            "$image" bash -euc '
                test "$(uname -s)" = Linux
                test "$(uname -m)" = "$2"
                work=$(mktemp -d)
                git clone --no-local --no-checkout /source "$work/source"
                cd "$work/source"
                git checkout --detach "$1"
                test "$(git rev-parse HEAD)" = "$1"
                test -z "$(git status --porcelain=v1 --untracked-files=all)"
                just check
            ' sh "$revision" "$1"
    fi
    local actual
    actual=$(git rev-parse HEAD) || native_fail "cannot recheck source revision"
    status=$(git status --porcelain=v1 --untracked-files=all) || native_fail "cannot recheck source cleanliness"
    [[ "$actual" == "$revision" ]] || native_fail "source revision changed during verification"
    [[ -z "$status" ]] || native_fail "source changed during verification"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    check_native "$@"
fi
