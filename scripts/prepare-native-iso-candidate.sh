#!/usr/bin/env bash
# Builds and PUBLISHES one native OCI candidate, then builds its exact-bound network ISO.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/check-native.sh"

registry=ghcr.io/levitateos/soda-os
publication_state=not-started
candidate_tag=
digest_reference=

info() { printf '==> %s\n' "$*" >&2; }
fail() { printf 'prepare-native-iso-candidate: %s\n' "$*" >&2; exit 1; }

report_failure() {
    local status=$?
    if [[ $status -ne 0 && "$publication_state" != not-started ]]; then
        printf 'Candidate preparation failed: publication=%s\ncandidate tag: %s\nexpected image: %s\nNo automatic retry or cleanup; inspect remote state before taking further action.\n' \
            "$publication_state" "$candidate_tag" "$digest_reference" >&2
    fi
}
trap report_failure EXIT

require_clean_main_checkout() {
    local head origin status
    status=$(git status --porcelain=v1 --untracked-files=all) || fail "cannot inspect source cleanliness"
    [[ -z "$status" ]] || fail "checkout is not clean"
    head=$(git rev-parse HEAD) || fail "cannot resolve HEAD"
    origin=$(git rev-parse origin/main) || fail "cannot resolve local origin/main"
    [[ "$head" == "$origin" ]] || fail "HEAD ($head) does not match local origin/main ($origin); this command does not fetch"
    [[ "$head" =~ ^[0-9a-f]{40}$ ]] || fail "HEAD is not a full source revision"
    printf '%s' "$head"
}

require_source_unchanged() {
    local actual
    actual=$(require_clean_main_checkout) || fail "cannot recheck clean source identity"
    [[ "$actual" == "$1" ]] || fail "source revision changed during candidate preparation"
}

require_absent() {
    local path
    for path in "$@"; do
        [[ ! -e "$path" && ! -L "$path" ]] || fail "candidate output already exists: $path"
    done
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

require_runtime_identity() {
    local archive=$1 version=$2 architecture=$3 image_id
    image_id=$(skopeo inspect --raw oci-archive:"$archive" | jq -er '.config.digest')
    [[ "$image_id" =~ ^sha256:[0-9a-f]{64}$ ]] || fail "OCI archive has no exact image config digest"
    docker load --input "$archive" >/dev/null
    [[ $(docker image inspect --format '{{.Id}}' "$image_id") == "$image_id" ]] || fail "loaded image ID differs from archive config digest"
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
    [[ $# -eq 1 ]] || fail "usage: scripts/prepare-native-iso-candidate.sh <aarch64|x86_64> (publishes to GHCR)"
    local architecture=$1 tool root revision version archive iso local_digest remote_digest metadata tags source_sum
    native_platform "$architecture"
    for tool in awk docker git go grep jq just sha256sum skopeo vp; do
        command -v "$tool" >/dev/null 2>&1 || fail "missing required command: $tool"
    done
    require_native_docker
    root=$(git rev-parse --show-toplevel)
    cd "$root"
    info "Checking clean origin/main source identity and selected inputs"
    revision=$(require_clean_main_checkout)
    version=$(derive_version) || fail "could not derive Soda version"
    [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "invalid Soda version: $version"
    archive=.artifacts/images/soda-os-$version-$architecture.oci.tar
    iso=.artifacts/images/SodaOS-$version-$architecture.iso
    candidate_tag=$registry:sha-$revision-$architecture
    require_absent "$archive" "$iso" "$iso.sha256"
    # A failed lookup is not evidence of absence. Stop on authentication/network errors.
    tags=$(skopeo list-tags docker://"$registry")
    jq -e --arg tag "sha-$revision-$architecture" '.Tags | type == "array" and (index($tag) == null)' <<<"$tags" >/dev/null \
        || fail "immutable candidate tag exists or tag listing is invalid: $candidate_tag"
    go run ./cmd/soda-image --architecture "$architecture" check

    info "Running complete native Linux just check"
    check_native "$architecture"
    require_source_unchanged "$revision"
    require_absent "$archive" "$iso" "$iso.sha256"
    info "Building native $architecture OCI archive (includes RPM construction)"
    just oci "$architecture"
    [[ -f "$archive" ]] || fail "expected OCI archive was not created: $archive"

    info "Verifying OCI identity and real Linux runtime before publication"
    metadata=$(skopeo inspect oci-archive:"$archive")
    jq -e --arg arch "$native_oci_arch" --arg version "$version" --arg revision "$revision" '
        .Os == "linux" and .Architecture == $arch and
        .Labels["org.opencontainers.image.version"] == $version and
        .Labels["org.opencontainers.image.revision"] == $revision
    ' <<<"$metadata" >/dev/null || fail "OCI platform, version, or source revision differs"
    local_digest=$(jq -er '.Digest' <<<"$metadata")
    [[ "$local_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || fail "invalid OCI digest"
    digest_reference=$registry@$local_digest
    require_runtime_identity "$archive" "$version" "$architecture"
    require_source_unchanged "$revision"
    require_absent "$iso" "$iso.sha256"

    info "Publishing immutable GHCR candidate: $candidate_tag ($digest_reference)"
    publication_state=attempted
    go run ./cmd/soda-release image-stage --architecture "$architecture" --archive "$archive"
    publication_state=confirmed
    info "Publication succeeded: $candidate_tag -> $digest_reference"
    remote_digest=$(skopeo inspect --no-creds --format '{{.Digest}}' docker://"$candidate_tag")
    [[ "$remote_digest" == "$local_digest" ]] || fail "anonymous GHCR digest differs from local OCI digest"
    require_source_unchanged "$revision"
    require_absent "$iso" "$iso.sha256"
    info "Building and deeply inspecting exact-bound network ISO"
    just iso "$architecture" "$archive"
    [[ -f "$iso" ]] || fail "expected ISO was not created: $iso"
    source_sum=$(sha256sum "$iso" | awk '{print $1}')
    grep -Fqx "$source_sum  $(basename "$iso")" "$iso.sha256" || fail "ISO checksum sidecar is missing or stale"
    grep -Fq "bootc --source-imgref=\"docker://$digest_reference\" --target-imgref=\"$digest_reference\"" \
        .artifacts/installer/context/interactive-defaults.ks || fail "installer source is not the exact published digest"
    require_source_unchanged "$revision"
    cat <<SUMMARY
Soda OS $architecture ISO candidate is ready (not installation-tested).
source commit: $revision
OCI candidate tag: $candidate_tag
published digest: $digest_reference
ISO: $root/$iso
ISO SHA-256: $source_sum
installation source: docker://$digest_reference
SUMMARY
}

main "$@"
