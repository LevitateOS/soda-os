#!/usr/bin/env bash
# Builds and PUBLISHES one native OCI image; never constructs an installer or restarts a VM.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/check-native.sh"

registry=ghcr.io/levitateos/soda-os
info() { printf '==> %s\n' "$*" >&2; }
fail() { printf 'prepare-native-image: %s\n' "$*" >&2; exit 1; }

clean_revision() {
    local revision status
    status=$(git status --porcelain=v1 --untracked-files=all) || fail "cannot inspect source cleanliness"
    [[ -z "$status" ]] || fail "checkout is not clean"
    revision=$(git rev-parse HEAD) || fail "cannot resolve HEAD"
    [[ "$revision" =~ ^[0-9a-f]{40}$ ]] || fail "HEAD is not a full source revision"
    printf '%s' "$revision"
}

derive_version() {
    awk '
        $0 == "[identity]" { identity=1; next }
        /^\[/ { identity=0 }
        identity && $1 == "version" && $2 == "=" {
            value=$3; gsub(/^"|"$/, "", value); print value; found=1; exit
        }
        END { if (!found) exit 1 }
    ' distro/soda.toml
}

require_source_unchanged() {
    local actual
    actual=$(clean_revision) || fail "cannot recheck clean source identity"
    [[ "$actual" == "$1" ]] || fail "source revision changed during image preparation"
}

require_runtime_identity() {
    local archive=$1 version=$2 architecture=$3 image_id config_digest manifest_digest
    config_digest=$(skopeo inspect --raw oci-archive:"$archive" | jq -er '.config.digest')
    manifest_digest=$(skopeo inspect --format '{{.Digest}}' oci-archive:"$archive")
    [[ "$config_digest" =~ ^sha256:[0-9a-f]{64}$ && "$manifest_digest" =~ ^sha256:[0-9a-f]{64}$ ]] \
        || fail "OCI archive has no exact config and manifest digests"
    docker load --input "$archive" >/dev/null
    # Classic Docker uses a config ID; the containerd store uses a manifest ID.
    image_id=$(docker image inspect --format '{{.Id}}' "$registry:$version")
    [[ "$image_id" == "$config_digest" || "$image_id" == "$manifest_digest" ]] \
        || fail "loaded image ID differs from archive config and manifest digests"
    docker run --rm --pull=never --platform "$native_docker_platform" --entrypoint /bin/sh "$image_id" -eu -c '
        test "$(uname -s)" = Linux
        test "$(uname -m)" = "$2"
        . /usr/lib/os-release
        test "$NAME" = "Soda OS"
        test "$VERSION" = "$1"
        test "$PRETTY_NAME" = "Soda OS $1"
        for package in soda-release soda-runtime soda-projects soda-runners; do
            identity=$(rpm -q --qf "%{VERSION} %{ARCH}\n" "$package")
            case "$identity" in "$1 $2"|"$1 noarch") ;; *) exit 1 ;; esac
            rpm -q --qf "%{RELEASE}\n" "$package" | grep -Eq "^[0-9]+\\.fc[0-9]+$"
        done
    ' sh "$version" "$architecture"
}

main() {
    [[ $# -eq 1 ]] || fail "usage: scripts/prepare-native-image.sh <aarch64|x86_64> (publishes to GHCR)"
    local architecture=$1 tool root revision version archive metadata
    native_platform "$architecture"
    for tool in awk docker git go grep jq just skopeo vp; do
        command -v "$tool" >/dev/null 2>&1 || fail "missing required command: $tool"
    done
    require_native_docker
    root=$(git rev-parse --show-toplevel)
    cd "$root"
    revision=$(clean_revision)
    version=$(derive_version) || fail "cannot derive Soda version"
    [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "invalid Soda version: $version"
    info "Checking clean committed source $revision on native $architecture"
    check_native "$architecture"
    require_source_unchanged "$revision"
    info "Preparing reviewed native inputs"
    just forgejo-source
    just github-runner "$architecture"
    just mise-rpm
    just tea-source
    just cosign-source
    require_source_unchanged "$revision"
    info "Building native OCI archive (includes RPM construction once)"
    archive=$(go run ./cmd/soda-image --architecture "$architecture" oci \
        --output-dir ".artifacts/images/$architecture/$revision") || fail "OCI construction failed; no publication attempted"
    [[ -f "$archive" ]] || fail "returned OCI archive does not exist: $archive"
    metadata=$(skopeo inspect oci-archive:"$archive")
    jq -e --arg arch "$native_oci_arch" --arg revision "$revision" --arg version "$version" '
        .Os == "linux" and .Architecture == $arch and
        .Labels["org.opencontainers.image.revision"] == $revision and
        .Labels["org.opencontainers.image.version"] == $version
    ' <<<"$metadata" >/dev/null || fail "OCI platform, version or source revision differs"
    require_runtime_identity "$archive" "$version" "$architecture"
    require_source_unchanged "$revision"
    info "Publishing archive revision and advancing dev-$architecture"
    go run ./cmd/soda-image --architecture "$architecture" publish --archive "$archive" \
        || fail "publication failed or is partially complete; inspect reported remote state before an explicit retry; no automatic retry or cleanup"
    printf 'Native OCI publication complete.\nsource commit: %s\narchive: %s\ndevelopment tag: %s:dev-%s\nNo installer was built and no installed update was tested.\n' \
        "$revision" "$archive" "$registry" "$architecture"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
