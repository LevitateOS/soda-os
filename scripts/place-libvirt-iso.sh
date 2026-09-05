#!/usr/bin/env bash
# Places an existing checked ISO only; never builds or publishes.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/check-native.sh"

fail() { printf 'place-libvirt-iso: %s\n' "$*" >&2; exit 1; }
destination=
copy_started=false
report_failure() {
    local status=$?
    if [[ $status -ne 0 && "$copy_started" == true ]]; then
        printf 'Placement failed after copy started; destination may be partial or complete: %s\nNo cleanup or overwrite was attempted.\n' "$destination" >&2
    fi
}
trap report_failure EXIT

main() {
    [[ $# -eq 3 ]] || fail "usage: scripts/place-libvirt-iso.sh <aarch64|x86_64> <ISO> <destination-directory>"
    local architecture=$1 iso=$2 directory=$3 tool source_sum copy_sum label qemu
    native_platform "$architecture"
    [[ "$native_os" == Linux ]] || fail "libvirt placement requires matching native Linux"
    qemu=qemu-system-$architecture
    for tool in "$qemu" sha256sum awk grep sudo stat chmod; do
        command -v "$tool" >/dev/null 2>&1 || fail "missing required command: $tool"
    done
    [[ -f "$iso" && ! -L "$iso" ]] || fail "source ISO must be a regular non-symlink file: $iso"
    [[ -d "$directory" ]] || fail "destination directory is missing: $directory"
    iso=$(cd "$(dirname "$iso")" && printf '%s/%s' "$(pwd -P)" "$(basename "$iso")")
    directory=$(cd "$directory" && pwd -P)
    destination=$directory/$(basename "$iso")
    [[ ! -e "$destination" && ! -L "$destination" ]] || fail "destination ISO already exists: $destination"
    source_sum=$(sha256sum "$iso" | awk '{print $1}')
    grep -Fqx "$source_sum  $(basename "$iso")" "$iso.sha256" || fail "source ISO checksum sidecar is missing or stale"
    # The kernel checks traversal of the actual path, including all its ancestors.
    sudo -n -u qemu -- test -x "$directory"
    copy_started=true
    (set -o noclobber; cat "$iso" > "$destination")
    chmod 0644 "$destination"
    copy_sum=$(sha256sum "$destination" | awk '{print $1}')
    [[ "$source_sum" == "$copy_sum" ]] || fail "copied ISO checksum differs from verified source"
    sudo -n -u qemu -- test -r "$destination"
    label=$(stat -c '%C' "$destination")
    [[ "$label" == *:virt_image_t:* ]] || fail "destination SELinux label is not virt_image_t: $label"
    sudo -n -u qemu -- sh -eu -c '
        tmp=$(mktemp -d)
        trap '\''rm -rf "$tmp"'\'' EXIT
        "$2" -display none -machine none -nodefaults -cdrom "$1" -S -daemonize -pidfile "$tmp/pid"
        kill "$(cat "$tmp/pid")"
    ' sh "$destination" "$qemu"
    printf 'Cockpit-selectable ISO: %s\nISO SHA-256: %s\nNon-booting QEMU open passed; installation remains untested.\n' "$destination" "$copy_sum"
}

main "$@"
