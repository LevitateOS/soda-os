#!/usr/bin/env bash
set -Eeuo pipefail

architecture=x86_64
oci_arch=amd64
registry=ghcr.io/levitateos/soda-os
destination_dir=${SODA_COCKPIT_ISO_DESTINATION_DIR:-/home/libvirt/images}

info() {
    printf '==> %s\n' "$*" >&2
}

fail() {
    printf 'prepare-x86_64-cockpit-iso-candidate: %s\n' "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_clean_main_checkout() {
    local status head origin
    status=$(git status --porcelain=v1 --untracked-files=all)
    [[ -z "$status" ]] || fail "checkout is not clean"
    head=$(git rev-parse HEAD)
    origin=$(git rev-parse origin/main)
    [[ "$head" == "$origin" ]] || fail "HEAD ($head) does not match origin/main ($origin)"
    [[ ${#head} -eq 40 ]] || fail "HEAD is not a full source revision"
    printf '%s' "$head"
}

derive_version() {
    awk '
        $0 == "[identity]" { identity=1; next }
        /^\[/ { identity=0 }
        identity && $1 == "version" && $2 == "=" {
            value=$3
            gsub(/^"|"$/, "", value)
            print value
            found=1
            exit
        }
        END { if (!found) exit 1 }
    ' distro/soda.toml
}

skopeo_digest() {
    skopeo inspect --format '{{.Digest}}' "$@"
}

skopeo_field() {
    skopeo inspect --format "$1" "$2"
}

require_runtime_identity() {
    local archive=$1 version=$2
    docker load --input "$archive" >/dev/null
    docker run --rm --platform linux/amd64 --entrypoint /bin/sh "$registry:$version" -eu -c '
        . /usr/lib/os-release
        test "$NAME" = "Soda OS"
        test "$VERSION" = "$1"
        test "$PRETTY_NAME" = "Soda OS $1"
        for package in soda-release soda-runtime soda-projects soda-runners; do
            rpm -q "$package" | grep -Eq "^${package}-${1}-[0-9]+\\.fc[0-9]+(\\.x86_64|\\.noarch)$"
        done
    ' sh "$version"
}

copy_without_overwrite() {
    local source=$1 destination=$2
    [[ ! -e "$destination" ]] || fail "destination ISO already exists: $destination"
    set -o noclobber
    cat "$source" > "$destination"
    set +o noclobber
    chmod 0644 "$destination"
}

require_libvirt_destination_preflight() {
    [[ -d "$destination_dir" ]] || fail "destination directory is missing: $destination_dir"
    sudo -n -u qemu -- test -x /home
    sudo -n -u qemu -- test -x /home/libvirt
    sudo -n -u qemu -- test -x "$destination_dir"
}

require_libvirt_readable_iso() {
    local iso=$1 label
    require_libvirt_destination_preflight
    sudo -n -u qemu -- test -r "$iso"
    label=$(stat -c '%C' "$iso")
    [[ "$label" == *:virt_image_t:* ]] || fail "$iso SELinux label is not virt_image_t: $label"
    sudo -n -u qemu -- sh -eu -c '
        tmp=$(mktemp -d)
        trap '\''rm -rf "$tmp"'\'' EXIT
        qemu-system-x86_64 -display none -machine none -nodefaults -cdrom "$1" -S -daemonize -pidfile "$tmp/pid"
        kill "$(cat "$tmp/pid")"
    ' sh "$iso"
}

main() {
    info "Checking native x86_64 host and required commands"
    [[ $(uname -m) == x86_64 ]] || fail "this command must run on a native x86_64 host"
    for tool in awk cat chmod docker git go just qemu-system-x86_64 sha256sum skopeo stat sudo; do
        require_command "$tool"
    done

    local root revision version archive candidate_tag local_digest remote_digest digest_reference iso source_sum copy_sum destination
    root=$(git rev-parse --show-toplevel)
    cd "$root"

    info "Checking clean origin/main source identity and deriving Soda version"
    revision=$(require_clean_main_checkout)
    version=$(derive_version) || fail "could not derive Soda version from distro/soda.toml"
    archive=.artifacts/images/soda-os-$version-$architecture.oci.tar
    iso=.artifacts/images/SodaOS-$version-$architecture.iso
    destination=$destination_dir/SodaOS-$version-$architecture.iso
    candidate_tag=$registry:sha-$revision-$architecture

    info "Checking candidate tag and destination availability"
    [[ ! -e "$destination" ]] || fail "destination ISO already exists: $destination"
    if skopeo inspect --no-creds docker://"$candidate_tag" >/dev/null 2>&1; then
        fail "immutable candidate tag already exists: $candidate_tag"
    fi
    info "Checking libvirt destination traversal permissions"
    require_libvirt_destination_preflight

    info "Running just check"
    just check
    info "Building locked x86_64 RPM inputs"
    just rpm "$architecture"
    info "Building x86_64 OCI archive"
    just oci "$architecture"
    [[ -f "$archive" ]] || fail "expected OCI archive was not created: $archive"

    info "Publishing immutable GHCR candidate with soda-release image-stage"
    go run ./cmd/soda-release image-stage --architecture "$architecture" --archive "$archive"

    info "Verifying anonymous GHCR digest against local OCI digest"
    local_digest=$(skopeo_digest oci-archive:"$archive")
    remote_digest=$(skopeo_digest --no-creds docker://"$candidate_tag")
    [[ "$remote_digest" == "$local_digest" ]] || fail "anonymous GHCR digest $remote_digest differs from local OCI digest $local_digest"
    digest_reference=$registry@$remote_digest

    info "Verifying OCI platform, Soda version, and exact source revision"
    [[ $(skopeo_field '{{.Os}}' oci-archive:"$archive") == linux ]] || fail "OCI metadata is not linux/amd64"
    [[ $(skopeo_field '{{.Architecture}}' oci-archive:"$archive") == "$oci_arch" ]] || fail "OCI metadata is not linux/amd64"
    [[ $(skopeo_field '{{ index .Labels "org.opencontainers.image.version" }}' oci-archive:"$archive") == "$version" ]] || fail "OCI version label differs from $version"
    [[ $(skopeo_field '{{ index .Labels "org.opencontainers.image.revision" }}' oci-archive:"$archive") == "$revision" ]] || fail "OCI source revision label differs from $revision"

    info "Building and inspecting matching graphical network-install ISO"
    just iso "$architecture" "$archive"
    info "Verifying ISO checksum sidecar and exact installer source"
    [[ -f "$iso" ]] || fail "expected ISO was not created: $iso"
    source_sum=$(sha256sum "$iso" | awk '{print $1}')
    grep -Fqx "$source_sum  $(basename "$iso")" "$iso.sha256" || fail "ISO checksum sidecar is missing or stale"
    grep -Fq "bootc --source-imgref=\"docker://$digest_reference\" --target-imgref=\"$digest_reference\"" .artifacts/installer/context/interactive-defaults.ks || fail "installer source is not the exact published digest reference"

    info "Verifying installed OCI os-release and Soda RPM versions"
    require_runtime_identity "$archive" "$version"

    info "Copying ISO without overwrite to $destination"
    copy_without_overwrite "$iso" "$destination"
    info "Comparing source and copied ISO SHA-256 checksums"
    source_sum=$(sha256sum "$iso" | awk '{print $1}')
    copy_sum=$(sha256sum "$destination" | awk '{print $1}')
    [[ "$source_sum" == "$copy_sum" ]] || fail "copied ISO checksum differs from source ISO"
    info "Verifying libvirt traversal, virt_image_t label, and non-booting QEMU open as qemu"
    require_libvirt_readable_iso "$destination"

    cat <<SUMMARY
Soda OS x86_64 Cockpit ISO candidate is ready.
source commit: $revision
OCI candidate tag: $candidate_tag
published digest: $digest_reference
ISO SHA-256: $copy_sum
Cockpit-selectable ISO: $destination
installation source: docker://$digest_reference
SUMMARY
}

main "$@"
