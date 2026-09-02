#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Private helper for tests/acceptance/unattended.sh. It is not a public workflow.

Commands:
  launch install       Create a blank disk and boot the configured installer ISO
  launch installed     Boot the existing acceptance disk without installer media
  wait                 Wait for SSH and Cockpit, then prove key-based admin SSH
  capture NAME         Capture nonprivileged host, guest, QMP, and registry evidence
  fallback seed-a      Seed authoritative mutable state on image A
  fallback seed-b      Seed current authoritative mutable state on image B
  fallback registry-enable
                       Permit only the disposable host-loopback registry
  fallback registry-disable
                       Remove the disposable registry configuration
  fallback capture NAME
                       Capture normalized fallback-preservation evidence
  fallback stage a|b   Download only the configured exact A or B image
  fallback unlock      Create a deployment from the downloaded image
  fallback mutate-b    Mutate authoritative state while booted into image B
  fallback compare EXPECTED ACTUAL
                       Compare two normalized preservation manifests
  workload start       Start the configured direct-workspace SSH continuity workload
  workload verify      Verify the continuity workload and record its heartbeat
  project-workspace ID Print the current user's derived username for one project
  scenario product     Exercise multi-user, transport, port, and deletion outcomes
  stop                 Request a clean ACPI shutdown through QMP

Required environment:
  SODA_ACCEPTANCE_DIR              Untracked evidence directory
  SODA_ACCEPTANCE_ARCHITECTURE     Sibling architecture: aarch64 or x86_64
  SODA_ACCEPTANCE_ADMIN_KEY        Private SSH key for the Anaconda administrator
  SODA_ACCEPTANCE_IMAGE_DIGEST     Expected sha256:... release digest for capture
  SODA_ACCEPTANCE_GUEST_HOST       Guest Tailnet IP or MagicDNS name

Additional fallback environment:
  SODA_ACCEPTANCE_IMAGE_A_REFERENCE Exact registry digest reference for image A
  SODA_ACCEPTANCE_IMAGE_B_REFERENCE Exact registry digest reference for image B

Additional launch install environment:
  SODA_ACCEPTANCE_ISO              Exact-digest Soda installer ISO
  SODA_ACCEPTANCE_KICKSTART_ISO    Protected generated OEMDRV installer input

Additional workspace/toolset environment:
  SODA_ACCEPTANCE_WORKSPACE_TARGET Derived Linux workspace username
  SODA_ACCEPTANCE_WORKSPACE_KEY    Primary user's standard SSH private key
  SODA_ACCEPTANCE_REQUIRE_WORKSPACE_TOOLSET
                                    Set to 1 for a milestone capture that must
                                    exercise the derived workspace account

Before capture NAME, create privileged bootc evidence in the guest:
  mkdir -p "$HOME/.local/state/soda-acceptance"
  sudo bootc status --format=json >"$HOME/.local/state/soda-acceptance/NAME-privileged.json"

Optional environment:
  SODA_ACCEPTANCE_ADMIN=vince
  SODA_ACCEPTANCE_HOST=127.0.0.1
  SODA_ACCEPTANCE_SSH_PORT=2222
  SODA_ACCEPTANCE_COCKPIT_PORT=9090
  SODA_ACCEPTANCE_GUEST_SSH_PORT=22
  SODA_ACCEPTANCE_GUEST_COCKPIT_PORT=9090
  SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE=<test-only administrator password file>
  SODA_ACCEPTANCE_DISK=$SODA_ACCEPTANCE_DIR/soda-system.qcow2
  SODA_ACCEPTANCE_DISK_SIZE=40G
  SODA_ACCEPTANCE_RELEASE_RECORD=<release record to hash during capture>
  SODA_ACCEPTANCE_ISO=<installer ISO to hash during capture>
  SODA_QEMU=<platform QEMU executable>
  SODA_QEMU_IMG=qemu-img
  SODA_QEMU_FIRMWARE=<AArch64 firmware or x86-64 OVMF code image>
  SODA_QEMU_VARS=<x86-64 OVMF writable variable-store template>
EOF
}

die() {
	echo "bootc-acceptance: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command $1 is unavailable"
}

need_file() {
	[ -f "$1" ] || die "required file $1 is unavailable"
}

acceptance_dir=${SODA_ACCEPTANCE_DIR:-}
architecture=${SODA_ACCEPTANCE_ARCHITECTURE:-}
admin=${SODA_ACCEPTANCE_ADMIN:-vince}
host=${SODA_ACCEPTANCE_HOST:-127.0.0.1}
ssh_port=${SODA_ACCEPTANCE_SSH_PORT:-2222}
cockpit_port=${SODA_ACCEPTANCE_COCKPIT_PORT:-9090}
guest_host=${SODA_ACCEPTANCE_GUEST_HOST:-}
guest_ssh_port=${SODA_ACCEPTANCE_GUEST_SSH_PORT:-22}
guest_cockpit_port=${SODA_ACCEPTANCE_GUEST_COCKPIT_PORT:-9090}

password_file() {
	path=${SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE:-}
	[ -n "$path" ] || die "SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE is required"
	need_file "$path"
	printf '%s\n' "$path"
}

valid_exact_image_reference() {
	reference=$1
	case "$reference" in
		*@sha256:*) ;;
		*) return 1 ;;
	esac
	repository=${reference%@sha256:*}
	digest=${reference##*@sha256:}
	printf '%s\n' "$repository" | LC_ALL=C grep -Eq \
		'^[a-z0-9][a-z0-9.-]*(:[0-9]{1,5})?/[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$' || return 1
	[ "${#digest}" -eq 64 ] || return 1
	printf '%s\n' "$digest" | LC_ALL=C grep -Eq '^[0-9a-f]{64}$'
}

fallback_reference() {
	case "$1" in
		a) reference=${SODA_ACCEPTANCE_IMAGE_A_REFERENCE:-} ;;
		b) reference=${SODA_ACCEPTANCE_IMAGE_B_REFERENCE:-} ;;
		*) die "fallback image must be a or b" ;;
	esac
	valid_exact_image_reference "$reference" ||
		die "SODA_ACCEPTANCE_IMAGE_$(printf '%s' "$1" | tr 'ab' 'AB')_REFERENCE must be an exact registry digest reference"
	printf '%s\n' "$reference"
}

require_fallback_references() {
	fallback_reference a >/dev/null
	fallback_reference b >/dev/null
}

fallback_root() {
	printf '%s/fallback\n' "$acceptance_dir"
}

fallback_operations_dir() {
	directory=$(fallback_root)/operations
	mkdir -p "$directory"
	printf '%s\n' "$directory"
}

privileged_bootc_status() {
	credentials=$(password_file)
	admin_ssh "sudo -k -S -p '' /usr/bin/bootc status --format=json --format-version=1" <"$credentials"
}

run_privileged_script() {
	emitter=$1
	credentials=$(password_file)
	{
		cat "$credentials"
		"$emitter"
	} | admin_ssh 'sudo -k -S -p "" /bin/bash -eu -o pipefail -s'
}

require_dir() {
	case "$architecture" in
		aarch64|x86_64) ;;
		*) die "SODA_ACCEPTANCE_ARCHITECTURE must be aarch64 or x86_64" ;;
	esac
	host_architecture=$(uname -m)
	case "$architecture:$host_architecture" in
		aarch64:aarch64|aarch64:arm64|x86_64:x86_64|x86_64:amd64) ;;
		*) die "Soda $architecture artifact operations require matching native hardware; running on $host_architecture" ;;
	esac
	[ -n "$acceptance_dir" ] || die "SODA_ACCEPTANCE_DIR is required"
	mkdir -p "$acceptance_dir"
	acceptance_dir=$(CDPATH= cd -- "$acceptance_dir" && pwd)
}

known_hosts_path() {
	printf '%s/known-hosts\n' "$acceptance_dir"
}

qmp_path() {
	printf '%s/qmp.sock\n' "$acceptance_dir"
}

require_guest_endpoint() {
	[ -n "$guest_host" ] || die "SODA_ACCEPTANCE_GUEST_HOST must name the enrolled guest on the Tailnet"
}

admin_ssh() {
	require_guest_endpoint
	admin_key=${SODA_ACCEPTANCE_ADMIN_KEY:-}
	[ -n "$admin_key" ] || die "SODA_ACCEPTANCE_ADMIN_KEY is required"
	need_file "$admin_key"
	need_file "$(known_hosts_path)"
	ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$admin_key" -p "$guest_ssh_port" \
		"$admin@$guest_host" "$@"
}

workspace_ssh() {
	require_guest_endpoint
	workspace_target=${SODA_ACCEPTANCE_WORKSPACE_TARGET:-}
	workspace_key=${SODA_ACCEPTANCE_WORKSPACE_KEY:-}
	[ -n "$workspace_target" ] || die "SODA_ACCEPTANCE_WORKSPACE_TARGET is required"
	[ -n "$workspace_key" ] || die "SODA_ACCEPTANCE_WORKSPACE_KEY is required"
	need_file "$workspace_key"
	need_file "$(known_hosts_path)"
	ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$workspace_key" -p "$guest_ssh_port" \
		"$workspace_target@$guest_host" "$@"
}

qmp() {
	qmp_socket=$(qmp_path)
	[ -S "$qmp_socket" ] || die "QMP socket $qmp_socket is unavailable"
	if command -v socat >/dev/null 2>&1; then
		printf '%s\n%s\n' '{"execute":"qmp_capabilities"}' "$1" | \
			socat -T 2 - "UNIX-CONNECT:$qmp_socket"
	elif command -v nc >/dev/null 2>&1; then
		printf '%s\n%s\n' '{"execute":"qmp_capabilities"}' "$1" | nc -U "$qmp_socket"
	else
		die "QMP requires socat or netcat with Unix-socket support"
	fi
}

start_installer_input_ejector() {
	installer_input=$1
	qemu_pid=$$
	(
		ejected=false
		eject_tmp=$acceptance_dir/.installer-input-eject.$$.jsonl
		cleanup_ejector() {
			if [ "$ejected" != true ] && kill -0 "$qemu_pid" 2>/dev/null; then
				kill -TERM "$qemu_pid" 2>/dev/null || true
				deadline=$(( $(date +%s) + 10 ))
				while kill -0 "$qemu_pid" 2>/dev/null; do
					if [ "$(date +%s)" -ge "$deadline" ]; then
						kill -KILL "$qemu_pid" 2>/dev/null || true
						break
					fi
					sleep 1
				done
			fi
			rm -f "$installer_input" "$eject_tmp"
		}
		abort_ejector() {
			trap - 1 2 15
			exit 1
		}
		trap cleanup_ejector 0
		trap abort_ejector 1 2 15

		deadline=$(( $(date +%s) + 600 ))
		while :; do
			kill -0 "$qemu_pid" 2>/dev/null || exit 1
			[ "$(date +%s)" -lt "$deadline" ] || die "Anaconda did not eject the parsed Kickstart input within 600 seconds"
			if [ -S "$(qmp_path)" ]; then
				if qmp '{"execute":"query-block","id":"soda-oemdrv-guest-ejected"}' >"$eject_tmp" 2>/dev/null; then
					if jq -s -e 'any(.[]; has("error"))' "$eject_tmp" >/dev/null; then
						die "QEMU rejected installer-input inspection"
					fi
					if jq -s -e '
						[ .[]
						  | select(.id? == "soda-oemdrv-guest-ejected")
						  | .return?
						  | select(type == "array")
						  | .[]
						  | select(
						      .device == "soda-oemdrv"
						      and .qdev == "soda-oemdrv-device"
						      and .removable == true
						      and .tray_open == true
						      and .locked == false
						    )
						] as $matches
						| ($matches | length) == 1
					' "$eject_tmp" >/dev/null; then
						if jq -s -e '
							[ .[]
							  | select(.id? == "soda-oemdrv-guest-ejected")
							  | .return?
							  | select(type == "array")
							  | .[]
							  | select(.device == "soda-oemdrv" and .qdev == "soda-oemdrv-device")
							] | length == 1 and (.[0] | has("inserted"))
						' "$eject_tmp" >/dev/null; then
							qmp '{"execute":"blockdev-remove-medium","arguments":{"id":"soda-oemdrv-device"},"id":"soda-oemdrv-remove-medium"}' >>"$eject_tmp" 2>/dev/null ||
								die "QEMU could not complete guest-requested installer-input removal"
						fi
						qmp '{"execute":"query-block","id":"soda-oemdrv-medium-absent"}' >>"$eject_tmp" 2>/dev/null ||
							die "QEMU could not verify installer-input removal"
						if jq -s -e 'any(.[]; has("error"))' "$eject_tmp" >/dev/null; then
							die "QEMU rejected guest-requested installer-input removal"
						fi
						jq -s -e '
							[ .[]
							  | select(.id? == "soda-oemdrv-medium-absent")
							  | .return?
							  | select(type == "array")
							  | .[]
							  | select(
							      .device == "soda-oemdrv"
							      and .qdev == "soda-oemdrv-device"
							      and .removable == true
							      and .tray_open == true
							      and .locked == false
							      and (has("inserted") | not)
							    )
							] | length == 1
						' "$eject_tmp" >/dev/null || die "QEMU still exposes the installer input"
						break
					fi
				fi
			fi
			sleep 1
		done

		rm -f "$installer_input"
		mv "$eject_tmp" "$acceptance_dir/installer-input-eject.jsonl"
		ejected=true
		trap - 0 1 2 15
	) &
}

launch() {
	mode=${1:-}
	[ "$mode" = install ] || [ "$mode" = installed ] || die "launch requires install or installed"
	require_dir
	qemu_img=${SODA_QEMU_IMG:-qemu-img}
	disk=${SODA_ACCEPTANCE_DISK:-$acceptance_dir/soda-system.qcow2}
	need "$qemu_img"
	[ ! -e "$(qmp_path)" ] || die "QMP socket already exists in $acceptance_dir"

	installer_args=
	if [ "$mode" = install ]; then
		iso=${SODA_ACCEPTANCE_ISO:-}
		[ -n "$iso" ] || die "SODA_ACCEPTANCE_ISO is required for launch install"
		need_file "$iso"
		[ ! -e "$disk" ] || die "refusing to install over existing disk $disk"
		"$qemu_img" create -f qcow2 "$disk" "${SODA_ACCEPTANCE_DISK_SIZE:-40G}"
		rm -f "$acceptance_dir/serial.log" "$acceptance_dir/installer-input-eject.jsonl"
		if [ "$architecture" = aarch64 ]; then
			installer_args="-drive file=$iso,media=cdrom,if=virtio,format=raw,readonly=on"
		else
			installer_args="-drive file=$iso,media=cdrom,format=raw,readonly=on"
		fi
		kickstart_iso=${SODA_ACCEPTANCE_KICKSTART_ISO:-}
		[ -n "$kickstart_iso" ] || die "SODA_ACCEPTANCE_KICKSTART_ISO is required for launch install"
		need_file "$kickstart_iso"
		need jq
		installer_args="$installer_args -drive if=none,file=$kickstart_iso,format=raw,readonly=on,id=soda-oemdrv -device virtio-scsi-pci,id=soda-oemdrv-scsi -device scsi-cd,drive=soda-oemdrv,id=soda-oemdrv-device"
	else
		need_file "$disk"
	fi

	case "$architecture" in
		aarch64) launch_aarch64 "$disk" "$installer_args" ;;
		x86_64) launch_x86_64 "$mode" "$disk" "$installer_args" ;;
	esac
}

launch_aarch64() {
	disk=$1
	installer_args=$2
	qemu=${SODA_QEMU:-qemu-system-aarch64}
	firmware=${SODA_QEMU_FIRMWARE:-/opt/homebrew/share/qemu/edk2-aarch64-code.fd}
	need "$qemu"
	need_file "$firmware"
	cat >"$acceptance_dir/qemu-command.txt" <<EOF
$qemu -machine virt,accel=hvf -cpu host -smp 4 -m 8192 -bios $firmware
-drive file=$disk,if=virtio,format=qcow2 $installer_args
-netdev user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090
EOF
	# Word splitting is intentional for the fixed, locally constructed installer arguments.
	# shellcheck disable=SC2086
	[ -z "${kickstart_iso:-}" ] || start_installer_input_ejector "$kickstart_iso"
	exec "$qemu" -machine virt,accel=hvf -cpu host -smp 4 -m 8192 -bios "$firmware" \
		-drive "file=$disk,if=virtio,format=qcow2" $installer_args \
		-device virtio-gpu-pci -display cocoa -device qemu-xhci -device usb-kbd -device usb-tablet \
		-netdev "user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090" \
		-device virtio-net-pci,netdev=net0 -serial "file:$acceptance_dir/serial.log" \
		-monitor none -qmp "unix:$(qmp_path),server=on,wait=off"
}

launch_x86_64() {
	mode=$1
	disk=$2
	installer_args=$3
	qemu=${SODA_QEMU:-/usr/libexec/qemu-kvm}
	firmware=${SODA_QEMU_FIRMWARE:-/usr/share/edk2/ovmf/OVMF_CODE.fd}
	vars_template=${SODA_QEMU_VARS:-/usr/share/edk2/ovmf/OVMF_VARS.fd}
	vars=$acceptance_dir/OVMF_VARS.fd
	need_file "$qemu"
	need_file "$firmware"
	need_file "$vars_template"
	set -- -display none
	display_command="-display none"
	boot_command=
	if [ "$mode" = install ]; then
		set -- -boot order=c,once=d "$@"
		boot_command="-boot order=c,once=d"
	else
		set -- -boot order=c "$@"
		boot_command="-boot order=c"
	fi
	if [ "$mode" = install ]; then
		cp "$vars_template" "$vars"
	else
		need_file "$vars"
	fi
	cat >"$acceptance_dir/qemu-command.txt" <<EOF
$qemu -machine q35,accel=kvm -cpu host -smp 4 -m 8192
-drive if=pflash,format=raw,readonly=on,file=$firmware
-drive if=pflash,format=raw,file=$vars
-drive file=$disk,if=virtio,format=qcow2 $installer_args
-netdev user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090
$display_command
$boot_command
EOF
	# Word splitting is intentional for the fixed, locally constructed installer arguments.
	# shellcheck disable=SC2086
	[ -z "${kickstart_iso:-}" ] || start_installer_input_ejector "$kickstart_iso"
	exec "$qemu" -machine q35,accel=kvm -cpu host -smp 4 -m 8192 \
		-drive "if=pflash,format=raw,readonly=on,file=$firmware" \
		-drive "if=pflash,format=raw,file=$vars" \
		-drive "file=$disk,if=virtio,format=qcow2" $installer_args \
		-netdev "user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090" \
		-device virtio-net-pci,netdev=net0 "$@" \
		-chardev "stdio,id=serial0,signal=off,logfile=$acceptance_dir/serial.log" -serial chardev:serial0 \
		-monitor none -qmp "unix:$(qmp_path),server=on,wait=off"
}

wait_ready() {
	require_dir
	require_guest_endpoint
	need curl
	need ssh-keyscan
	need ssh-keygen
	started=$(date +%s)
	deadline=$((started + 1200))
	known_hosts=$(known_hosts_path)
	known_hosts_tmp=$acceptance_dir/.known-hosts.$$
	trap 'rm -f "$known_hosts_tmp"' 0 1 2 15
	while :; do
		[ "$(date +%s)" -lt "$deadline" ] || die "installed guest SSH did not become ready within 1200 seconds"
		ssh-keyscan -T 5 -t ed25519 -p "$guest_ssh_port" "$guest_host" >"$known_hosts_tmp" 2>/dev/null || true
		if [ -s "$known_hosts_tmp" ]; then
			mv "$known_hosts_tmp" "$known_hosts"
			if admin_ssh 'id; cat /proc/sys/kernel/random/boot_id'; then
				break
			fi
		fi
		sleep 30
	done
	trap - 0 1 2 15
	ssh-keygen -lf "$known_hosts" >"$acceptance_dir/ssh-host-key-fingerprint.txt"
	while ! curl --fail --silent --show-error --insecure "https://$guest_host:$guest_cockpit_port/ping" >/dev/null 2>&1; do
		[ "$(date +%s)" -lt "$deadline" ] || die "Cockpit did not become ready within 1200 seconds"
		sleep 2
	done
	printf 'ready_at=%s\nelapsed_seconds=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(( $(date +%s) - started ))" |
		tee "$acceptance_dir/readiness.txt"
}

valid_name() {
	case "$1" in
		''|*[!a-z0-9-]*) return 1 ;;
		*) return 0 ;;
	esac
}

emit_toolset_smoke() {
	cat <<'SODA_TOOLSET_SMOKE'
set -eu

toolset=/usr/share/soda/toolset-commands.txt
test -r "$toolset"
work=$(mktemp -d "${TMPDIR:-/tmp}/soda-toolset.XXXXXX")
trap 'rm -rf "$work"' 0 1 2 15

cat >"$work/expected-toolset.txt" <<'SODA_EXPECTED_TOOLSET'
go
gofmt
python3
uv
uvx
rustc
cargo
rustfmt
cargo-clippy
node
npm
npx
bun
gcc
g++
cpp
as
ld
ar
make
cmake
ninja
pkg-config
git
git-lfs
gh
tea
ssh
scp
sftp
rsync
podman
buildah
skopeo
sqlite3
jq
yq
curl
wget
openssl
patch
rg
fd
fzf
shellcheck
just
tar
gzip
bzip2
xz
zstd
zip
unzip
vim
nano
SODA_EXPECTED_TOOLSET

echo "[identity]"
id
echo "[command-resolution]"
cmp "$work/expected-toolset.txt" "$toolset"
while IFS= read -r tool; do
	test -n "$tool"
	command -v "$tool"
done <"$toolset"

echo "[language-smoke]"
cat >"$work/main.go" <<'SODA_GO_SOURCE'
package main

import "fmt"

func main() {
	fmt.Println("go-ok")
}
SODA_GO_SOURCE
(
	cd "$work"
	GOTOOLCHAIN=local GO111MODULE=off GOCACHE="$work/go-cache" \
		GOPATH="$work/go-path" GOMODCACHE="$work/go-mod-cache" \
		go run main.go
)
python3 -c 'print("python-ok")'
cat >"$work/main.rs" <<'SODA_RUST_SOURCE'
fn main() {
    println!("rust-ok");
}
SODA_RUST_SOURCE
rustc "$work/main.rs" -o "$work/rust-smoke"
"$work/rust-smoke"
node -e 'console.log("node-ok")'
bun -e 'console.log("bun-ok")'
cat >"$work/main.c" <<'SODA_C_SOURCE'
#include <stdio.h>

int main(void) {
    puts("c-ok");
    return 0;
}
SODA_C_SOURCE
gcc "$work/main.c" -o "$work/c-smoke"
"$work/c-smoke"
cat >"$work/main.cpp" <<'SODA_CPP_SOURCE'
#include <iostream>

int main() {
    std::cout << "c++-ok" << std::endl;
    return 0;
}
SODA_CPP_SOURCE
g++ "$work/main.cpp" -o "$work/cpp-smoke"
"$work/cpp-smoke"

echo "[representative-tools]"
go version
python3 --version
rustc --version
node --version
bun --version
gcc --version | sed -n '1p'
g++ --version | sed -n '1p'
git --version
git-lfs version
gh --version | sed -n '1p'
tea --version | sed -n '1p'
tea --help >/dev/null
ssh -V 2>&1
cmake --version | sed -n '1p'
ninja --version
make --version | sed -n '1p'
pkg-config --version
curl --version | sed -n '1p'
wget --version | sed -n '1p'
openssl version
yq --version
rg --version | sed -n '1p'
fd --version
fzf --version
shellcheck --version | sed -n '1,2p'
just --version
vim --version | sed -n '1p'
nano --version | sed -n '1p'

echo "[archive-and-data-smoke]"
(
	cd "$work"
	printf 'tar-ok\n' >tar-source.txt
	tar -czf archive.tar.gz tar-source.txt
	rm tar-source.txt
	tar -xzf archive.tar.gz
	grep -Fx tar-ok tar-source.txt
	printf 'zip-ok\n' >zip-source.txt
	zip -q archive.zip zip-source.txt
	rm zip-source.txt
	unzip -q archive.zip
	grep -Fx zip-ok zip-source.txt
)
sqlite3 "$work/smoke.db" "CREATE TABLE smoke(value TEXT); INSERT INTO smoke VALUES ('sqlite-ok');"
test "$(sqlite3 "$work/smoke.db" 'SELECT value FROM smoke;')" = sqlite-ok
printf '{"status":"jq-ok"}\n' | jq -e '.status == "jq-ok"'
printf 'needle\n' >"$work/search.txt"
rg -x needle "$work/search.txt"
fd -t f '^search[.]txt$' "$work"

echo "[rootless-podman]"
podman info --format json >"$work/podman-info.json"
jq -e '.host.security.rootless == true' "$work/podman-info.json"
podman unshare /bin/sh -c 'test "$(id -u)" -eq 0'

echo "toolset-smoke=ok"
SODA_TOOLSET_SMOKE
}

emit_installer_provisioning_absence() {
	cat <<'SODA_INSTALLER_PROVISIONING_ABSENCE'
set -eu

for path in \
	/root/anaconda-ks.cfg \
	/root/original-ks.cfg \
	/run/soda-installer \
	/var/lib/soda-install \
	/usr/libexec/soda/soda-installer-finalize \
	/usr/libexec/soda/soda-installer-input \
	/usr/share/anaconda/addons/org_fedoraproject_soda \
	/usr/share/anaconda/dbus/confs/org.fedoraproject.Anaconda.Addons.SodaInstaller.conf \
	/usr/share/anaconda/dbus/services/org.fedoraproject.Anaconda.Addons.SodaInstaller.service; do
	if test -e "$path"; then
		echo "unexpected-path=$path"
		exit 1
	fi
	echo "$path=absent"
done
enrollment_state=$(systemctl is-enabled soda-tailscale-enroll.service 2>/dev/null || true)
test "$enrollment_state" = disabled
echo "soda-tailscale-enroll.service=disabled"
SODA_INSTALLER_PROVISIONING_ABSENCE
}

emit_installed_ownership_checks() {
	cat <<'EOF'
set -eu
rules=$(nft list chain inet soda_ingress input)
printf '%s\n' "$rules"
printf '%s\n' "$rules" | grep -F 'iifname { "lo", "tailscale0" } tcp dport { 22, 9090, 30000 } accept' >/dev/null
printf '%s\n' "$rules" | grep -F 'tcp dport { 22, 9090, 30000 } reject with tcp reset' >/dev/null
members=$(getent group soda-workspaces | cut -d: -f4 | tr ',' ' ')
for username in $members; do
	id -nG "$username" | tr ' ' '\n' | grep -Fx soda-workspaces >/dev/null
	! id -nG "$username" | tr ' ' '\n' | grep -Fx wheel >/dev/null
	marker=$(getent passwd "$username" | cut -d: -f5)
	printf '%s\n' "$marker" | grep -Eq '^soda-workspace=[a-z][a-z0-9-]{0,23}/[a-z][a-z0-9-]{0,23}$'
	done
EOF
}

emit_home_context_check() {
	cat <<'SODA_HOME_CONTEXT_CHECK'
set -eu

username=$(id -un)
physical_home=$(readlink -f "$HOME")
test "$physical_home" = "/var/home/$username"
context_type() {
	stat -c %C "$1" | cut -d: -f3
}
home_type=$(context_type "$physical_home")
ssh_type=$(context_type "$physical_home/.ssh")
key_type=$(context_type "$physical_home/.ssh/authorized_keys")
test "$home_type" = user_home_dir_t
test "$ssh_type" = ssh_home_t
test "$key_type" = ssh_home_t
printf 'physical-home=%s\nhome-type=%s\nssh-type=%s\nkey-type=%s\n' \
	"$physical_home" "$home_type" "$ssh_type" "$key_type"
SODA_HOME_CONTEXT_CHECK
}

capture() {
	name=${1:-}
	valid_name "$name" || die "capture requires a lowercase name containing only letters, digits, and hyphens"
	require_dir
	need jq
	image_digest=${SODA_ACCEPTANCE_IMAGE_DIGEST:-}
	case "$image_digest" in
		sha256:????????????????????????????????????????????????????????????????) ;;
		*) die "SODA_ACCEPTANCE_IMAGE_DIGEST must be an exact sha256 digest" ;;
	esac
	require_workspace_toolset=${SODA_ACCEPTANCE_REQUIRE_WORKSPACE_TOOLSET:-0}
	case "$require_workspace_toolset" in
	0|1) ;;
	*) die "SODA_ACCEPTANCE_REQUIRE_WORKSPACE_TOOLSET must be 0 or 1" ;;
	esac
	verify_workspace_toolset=0
	if [ "$require_workspace_toolset" = 1 ] || [ -n "${SODA_ACCEPTANCE_WORKSPACE_TARGET:-}" ] || [ -n "${SODA_ACCEPTANCE_WORKSPACE_KEY:-}" ]; then
		[ -n "${SODA_ACCEPTANCE_WORKSPACE_TARGET:-}" ] || die "SODA_ACCEPTANCE_WORKSPACE_TARGET is required when verifying workspace tools"
		[ -n "${SODA_ACCEPTANCE_WORKSPACE_KEY:-}" ] || die "SODA_ACCEPTANCE_WORKSPACE_KEY is required when verifying workspace tools"
		verify_workspace_toolset=1
	fi
	privileged_tmp=$acceptance_dir/."$name-privileged.$$.json"
	trap 'rm -f "$privileged_tmp"' 0 1 2 15
	password_file=${SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE:-}
	if [ -n "$password_file" ]; then
		need_file "$password_file"
		admin_ssh "sudo -S -p '' bootc status --format=json" <"$password_file" >"$privileged_tmp"
	else
		privileged=".local/state/soda-acceptance/$name-privileged.json"
		admin_ssh "test -r \"\$HOME/$privileged\"" || die "missing operator evidence: $privileged"
		scp -q -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
			-o "UserKnownHostsFile=$(known_hosts_path)" -i "${SODA_ACCEPTANCE_ADMIN_KEY}" -P "$guest_ssh_port" \
			"$admin@$guest_host:$privileged" "$privileged_tmp"
	fi
	booted_digest=$(jq -r '.status.booted.image.imageDigest // empty' "$privileged_tmp")
	[ "$booted_digest" = "$image_digest" ] || die "booted digest $booted_digest does not match expected $image_digest"
	checkpoint=$acceptance_dir/$name
	[ ! -e "$checkpoint" ] || die "checkpoint $checkpoint already exists"
	mkdir -p "$checkpoint"
	date -u +%Y-%m-%dT%H:%M:%SZ >"$checkpoint/captured-at.txt"
	mv "$privileged_tmp" "$checkpoint/privileged.json"
	trap - 0 1 2 15
	qmp '{"execute":"query-status"}' >"$checkpoint/qmp-status.json"
	admin_ssh '
		set -eu
		echo "[identity]"; id
		test "$(getenforce)" = Enforcing
		for path in "$HOME" "$HOME/.ssh" "$HOME/.ssh/authorized_keys"; do
			matchpathcon -V "$path"
		done
		echo "[time]"; date -u +%Y-%m-%dT%H:%M:%SZ
		echo "[boot-id]"; cat /proc/sys/kernel/random/boot_id
		echo "[kernel]"; uname -a
		echo "[services]"
		for unit in sshd cockpit.socket forgejo tailscaled; do
			state=$(systemctl is-active "$unit")
			test "$state" = active
			printf "%s=%s\n" "$unit" "$state"
		done
		echo "[failed-units]"
		failed_units=$(systemctl --failed --no-legend --plain || true)
		if test -n "$failed_units"; then
			printf "%s\n" "$failed_units"
			printf "%s\n" "$failed_units" | awk '{print $1}' | while IFS= read -r failed_unit; do
				systemctl status --no-pager --full -- "$failed_unit" || true
				journalctl --boot --no-pager --unit "$failed_unit" --lines 100 || true
			done
			exit 1
		fi
		echo none
		echo "[stock-cockpit]"
		rpm -q cockpit-ws cockpit-system cockpit-storaged cockpit-networkmanager
		for manifest in \
			/usr/share/cockpit/storaged/manifest.json \
			/usr/share/cockpit/networkmanager/manifest.json \
			/usr/share/cockpit/soda-projects/manifest.json; do
			test -s "$manifest"
			echo "$manifest=present"
		done
		test -s /usr/share/cockpit/branding/sodaos/branding.css
		echo "/usr/share/cockpit/branding/sodaos/branding.css=present"
		echo "[native-git-host]"
		rpm -q soda-release soda-runtime soda-projects soda-forgejo soda-bun soda-tea
		expected_tea_files="/usr/bin/tea
/usr/share/licenses/soda-tea/LICENSE"
		test "$(rpm -ql soda-tea)" = "$expected_tea_files"
		echo "soda-tea-ownership=executable-and-license-only"
		for path in /etc/gh /var/lib/gh /etc/soda/gh /var/lib/soda/gh /etc/tea /var/lib/tea /etc/soda/tea /var/lib/soda/tea; do
			if test -e "$path"; then
				echo "unexpected-forge-cli-state=$path"
				exit 1
			fi
			echo "$path=absent"
		done
		forgejo --version
		getent passwd git
		test -s /etc/forgejo/app.ini && echo configuration=present
		echo "[deleted-workspace-control-plane]"
		for unit in soda-authd.service soda-cockpit.service avahi-daemon.service var-srv-soda-projects.mount; do
			if systemctl cat "$unit" >/dev/null 2>&1; then
				echo "unexpected-unit=$unit"
				exit 1
			fi
			echo "$unit=absent"
		done
		for path in \
			/var/lib/soda/soda.db \
			/var/lib/soda/built-in-git-token \
			/var/lib/soda/projects \
			/var/lib/soda/certs \
			/var/srv/soda/projects \
			/srv/soda/projects \
			/etc/soda/authorized_keys \
			/etc/ssh/sshd_config.d/41-soda-project-accounts.conf \
			/etc/avahi/services/soda-cockpit.service \
			/etc/pam.d/soda-cockpit \
			/usr/libexec/soda/soda-ssh \
			/usr/libexec/soda/soda-authd \
			/usr/libexec/soda/soda-cockpit \
			/var/log/soda/soda-authd \
			/var/log/soda/soda-cockpit; do
			if test -e "$path"; then
				echo "unexpected-path=$path"
				exit 1
			fi
			echo "$path=absent"
		done
		if getent group soda-people >/dev/null; then
			echo "unexpected-group=soda-people"
			exit 1
		fi
		echo "soda-people=absent"
		echo "[deleted-residual-control-plane]"
		if systemctl cat sodad.service >/dev/null 2>&1; then
			echo "unexpected-unit=sodad.service"
			exit 1
		fi
		echo "sodad.service=absent"
		for path in /usr/libexec/soda/sodad /usr/bin/sodactl /run/soda/sodad.sock /var/log/soda; do
			if test -e "$path"; then
				echo "unexpected-path=$path"
				exit 1
			fi
			echo "$path=absent"
		done
		if getent group soda-api >/dev/null; then
			echo "unexpected-group=soda-api"
			exit 1
		fi
		echo "soda-api=absent"
		if rpm -ql soda-runtime | grep -E "(^|/)(sodad|sodactl)(/|$)|soda-api|^/var/log/soda(/|$)" >/dev/null; then
			echo "unexpected-runtime-control-plane-ownership"
			exit 1
		fi
		echo "soda-runtime-control-plane-ownership=absent"
		echo "[deleted-toolchain-control-plane]"
		for unit in soda-state-directories.service opt-soda-toolchains.mount; do
			if systemctl cat "$unit" >/dev/null 2>&1; then
				echo "unexpected-unit=$unit"
				exit 1
			fi
			echo "$unit=absent"
		done
		for path in /opt/soda/toolchains /var/lib/soda/toolchains; do
			if test -e "$path"; then
				echo "unexpected-path=$path"
				exit 1
			fi
			echo "$path=absent"
		done
		echo "[installer-scratch]"
		if test -e /usr/lib/systemd/system/var-tmp.mount; then
			echo "unexpected-path=/usr/lib/systemd/system/var-tmp.mount"
			exit 1
		fi
		if find /var/tmp -maxdepth 1 -name "container_images_*" -print -quit | grep -q .; then
			echo "unexpected-path=/var/tmp/container_images_*"
			exit 1
		fi
		echo "var-tmp.mount=absent"
		echo "container-images-scratch=absent"
		echo "[boot-entries]"; efibootmgr -v 2>/dev/null || true
		echo "[automatic-update]"
		timer_state=$(systemctl is-enabled bootc-fetch-apply-updates.timer 2>/dev/null || true)
		test "$timer_state" = masked
		printf "bootc-fetch-apply-updates.timer=%s\n" "$timer_state"
		printf "bootc-fetch-apply-updates.service="
		systemctl is-enabled bootc-fetch-apply-updates.service 2>/dev/null || true
		echo "[soda-state-filesystem]"
		findmnt /var/lib/soda 2>/dev/null || true
		echo "[host-keys]"
		sha256sum /etc/ssh/ssh_host_*_key.pub 2>/dev/null || true
		echo "[console]"
		systemctl show getty@tty1.service autovt@tty1.service -p Id -p Names -p MainPID -p NRestarts
		sysctl kernel.printk
	' >"$checkpoint/guest.txt" 2>"$checkpoint/guest.stderr"
	emit_toolset_smoke | admin_ssh /bin/sh -s \
		>"$checkpoint/primary-toolset.txt" 2>"$checkpoint/primary-toolset.stderr"
	emit_home_context_check | admin_ssh /bin/sh -s \
		>"$checkpoint/primary-home-contexts.txt" 2>"$checkpoint/primary-home-contexts.stderr"
	if [ "$verify_workspace_toolset" = 1 ]; then
		emit_toolset_smoke | workspace_ssh /bin/sh -s \
			>"$checkpoint/workspace-toolset.txt" 2>"$checkpoint/workspace-toolset.stderr"
		emit_home_context_check | workspace_ssh /bin/sh -s \
			>"$checkpoint/workspace-home-contexts.txt" 2>"$checkpoint/workspace-home-contexts.stderr"
	fi
	if [ -n "$password_file" ]; then
		run_privileged_script emit_installer_provisioning_absence \
			>"$checkpoint/installer-provisioning.txt" 2>"$checkpoint/installer-provisioning.stderr"
		run_privileged_script emit_installed_ownership_checks \
			>"$checkpoint/native-ownership.txt" 2>"$checkpoint/native-ownership.stderr"
	else
		emit_installer_provisioning_absence | admin_ssh 'sudo -n /bin/sh -s' \
			>"$checkpoint/installer-provisioning.txt" 2>"$checkpoint/installer-provisioning.stderr"
		emit_installed_ownership_checks | admin_ssh 'sudo -n /bin/sh -s' \
			>"$checkpoint/native-ownership.txt" 2>"$checkpoint/native-ownership.stderr"
	fi
	curl --fail --silent --show-error --insecure "https://$guest_host:$guest_cockpit_port/ping" >"$checkpoint/cockpit-health.txt"

	for artifact in "${SODA_ACCEPTANCE_RELEASE_RECORD:-}" "${SODA_ACCEPTANCE_ISO:-}"; do
		[ -z "$artifact" ] || [ ! -f "$artifact" ] || sha256sum "$artifact" >>"$checkpoint/artifact-sha256sums.txt"
	done
}

fallback_assert_booted() {
	target=$1
	reference=$(fallback_reference "$target")
	expected_digest=${reference##*@}
	status_tmp=$(fallback_root)/.booted-$$.json
	mkdir -p "$(fallback_root)"
	trap 'rm -f "$status_tmp"' 0 1 2 15
	privileged_bootc_status >"$status_tmp"
	jq -e --arg digest "$expected_digest" \
		'.status.booted.image.imageDigest == $digest' "$status_tmp" >/dev/null ||
		die "guest is not booted into configured image $target digest"
	rm -f "$status_tmp"
	trap - 0 1 2 15
}

project_password_request() {
	action=$1
	project_id=$2
	display_name=$3
	credentials=$(password_file)
	case "$action" in
		create-forgejo)
			jq -cn --rawfile password "$credentials" --arg id "$project_id" --arg display_name "$display_name" \
				'{id:$id,display_name:$display_name,password:($password|gsub("[\\r\\n]+$";""))}' |
				admin_ssh /usr/libexec/soda/soda-projects create-forgejo
			;;
		setup)
			jq -cn --rawfile password "$credentials" --arg id "$project_id" --arg username "$admin" \
				'{id:$id,git_username:$username,git_password:($password|gsub("[\\r\\n]+$";""))}' |
				admin_ssh /usr/libexec/soda/soda-projects setup
			;;
		*) die "unsupported password-bearing project request $action" ;;
	esac
}

project_request() {
	action=$1
	project_id=$2
	case "$action" in
		remove)
			jq -cn --arg id "$project_id" '{id:$id}' |
				admin_ssh /usr/libexec/soda/soda-projects remove
			;;
		*) die "unsupported project request $action" ;;
	esac
}

emit_seed_accounts() {
	cat <<'EOF'
test "$(id -u)" -eq 0
test "$(getent passwd soda-test | cut -d: -f6)" = /home/soda-test
id -nG soda-test | tr ' ' '\n' | grep -Fx wheel >/dev/null
! id -nG soda-test | tr ' ' '\n' | grep -Fx soda-workspaces >/dev/null
test -s /home/soda-test/.ssh/authorized_keys
ssh-keygen -l -f /home/soda-test/.ssh/authorized_keys >/dev/null
jq -e 'length == 0' /var/lib/soda/catalog/projects.json >/dev/null
test -z "$(getent group soda-workspaces | cut -d: -f4)"
for username in alice obsolete bob; do
	! getent passwd "$username" >/dev/null
done

for username in alice obsolete; do
	/usr/sbin/useradd --create-home --user-group --shell /bin/bash -- "$username"
	home=$(getent passwd "$username" | cut -d: -f6)
	group=$(id -gn "$username")
	install -d -m 0700 -o "$username" -g "$group" "$home/.ssh"
	install -m 0600 -o "$username" -g "$group" /home/soda-test/.ssh/authorized_keys "$home/.ssh/authorized_keys"
	/usr/sbin/runuser --user "$username" -- /usr/bin/env HOME="$home" STATE="$username" /bin/sh -c \
		' umask 077; printf "seed-a:%s\n" "$STATE" >"$HOME/soda-acceptance-state.txt" '
	restorecon -RF "$home"
done
/usr/sbin/runuser --user soda-test -- /usr/bin/env HOME=/home/soda-test /bin/sh -c \
	' umask 077; printf "seed-a:soda-test\n" >"$HOME/soda-acceptance-state.txt" '
restorecon -RF /home/soda-test
alice_password=$(openssl rand -base64 32 | tr -d '\n')
obsolete_password=$(openssl rand -base64 32 | tr -d '\n')
printf 'alice:%s\nobsolete:%s\n' "$alice_password" "$obsolete_password" | /usr/sbin/chpasswd
unset alice_password obsolete_password
EOF
}

emit_seed_workspace_files() {
	cat <<'EOF'
test "$(id -u)" -eq 0
for project in kept removed; do
	marker="soda-workspace=soda-test/$project"
	workspace=$(getent passwd | awk -F: -v marker="$marker" '$5 == marker {print $1}')
	test -n "$workspace"
	test "$(printf '%s\n' "$workspace" | wc -l)" -eq 1
	home=$(getent passwd "$workspace" | cut -d: -f6)
	group=$(id -gn "$workspace")
	/usr/sbin/runuser --user "$workspace" -- /usr/bin/env HOME="$home" PROJECT="$project" /bin/sh -c \
		' umask 077; printf "seed-a:%s\n" "$PROJECT" >"$HOME/Projects/$PROJECT/soda-acceptance-state.txt" '
	chown "$workspace:$group" "$home/Projects/$project/soda-acceptance-state.txt"
	restorecon -RF "$home"
done
EOF
}

fallback_seed_a() {
	require_dir
	require_guest_endpoint
	require_fallback_references
	need jq
	[ "$admin" = soda-test ] || die "fallback seed-a requires the installer administrator soda-test"
	fallback_assert_booted a
	operations=$(fallback_operations_dir)
	[ ! -e "$operations/seed-a.complete" ] || die "fallback seed-a has already completed"
	run_privileged_script emit_seed_accounts >"$operations/seed-a-accounts.txt"
	for project in kept removed; do
		case "$project" in kept) display_name=Kept ;; removed) display_name=Removed ;; esac
		project_password_request create-forgejo "$project" "$display_name" >"$operations/seed-a-$project-create.json"
		project_password_request setup "$project" "" >"$operations/seed-a-$project-setup.json"
	done
	run_privileged_script emit_seed_workspace_files >"$operations/seed-a-workspaces.txt"
	capture_forgejo_state >"$operations/seed-a-forgejo.json"
	jq -e '[.repositories[] | select((.name == "kept" or .name == "removed") and .owner == "soda-test" and .empty == true)] | length == 2' \
		"$operations/seed-a-forgejo.json" >/dev/null || die "Forgejo did not retain both native empty seed repositories"
	date -u +%Y-%m-%dT%H:%M:%SZ >"$operations/seed-a.complete"
}

fallback_seed_b() {
	require_dir
	require_guest_endpoint
	require_fallback_references
	need jq
	[ "$admin" = soda-test ] || die "fallback seed-b requires the installer administrator soda-test"
	fallback_assert_booted b
	operations=$(fallback_operations_dir)
	[ ! -e "$operations/seed-b.complete" ] || die "fallback seed-b has already completed"
	run_privileged_script emit_seed_accounts >"$operations/seed-b-accounts.txt"
	for project in kept removed; do
		case "$project" in kept) display_name=Kept ;; removed) display_name=Removed ;; esac
		project_password_request create-forgejo "$project" "$display_name" >"$operations/seed-b-$project-create.json"
		project_password_request setup "$project" "" >"$operations/seed-b-$project-setup.json"
	done
	run_privileged_script emit_seed_workspace_files >"$operations/seed-b-workspaces.txt"
	run_privileged_script emit_mutate_accounts >"$operations/seed-b-current-accounts.txt"
	kept_url=$(admin_ssh "jq -er '.[] | select(.id == \"kept\") | .canonical_url' /var/lib/soda/catalog/projects.json")
	jq -cn --arg id kept --arg display_name 'Kept on B' --arg canonical_url "$kept_url" \
		'{id:$id,display_name:$display_name,canonical_url:$canonical_url}' |
		admin_ssh /usr/libexec/soda/soda-projects edit >"$operations/seed-b-kept-edit.json"
	run_privileged_script emit_mutate_workspace >"$operations/seed-b-kept-workspace.txt"
	project_password_request create-forgejo new 'New on B' >"$operations/seed-b-new-create.json"
	project_password_request setup new '' >"$operations/seed-b-new-setup.json"
	project_request remove removed >"$operations/seed-b-removed-remove.json"
	capture_forgejo_state >"$operations/seed-b-forgejo.json"
	jq -e 'any(.repositories[]; .name == "removed" and .owner == "soda-test")' \
		"$operations/seed-b-forgejo.json" >/dev/null || die "project removal deleted the canonical Forgejo repository"
	date -u +%Y-%m-%dT%H:%M:%SZ >"$operations/seed-b.complete"
}

fallback_stage() {
	target=${1:-}
	require_dir
	require_guest_endpoint
	need jq
	require_fallback_references
	reference=$(fallback_reference "$target")
	digest=${reference##*@}
	operations=$(fallback_operations_dir)
	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	credentials=$(password_file)
	admin_ssh "sudo -k -S -p '' /usr/bin/bootc switch --download-only '$reference'" <"$credentials" \
		>"$operations/stage-$target-$stamp.stdout" 2>"$operations/stage-$target-$stamp.stderr"
	privileged_bootc_status >"$operations/stage-$target-$stamp.json"
	jq -e --arg reference "$reference" --arg digest "$digest" '
		.status.staged.image.image.image == $reference and
		.status.staged.image.imageDigest == $digest and
		.status.staged.downloadOnly == true
	' "$operations/stage-$target-$stamp.json" >/dev/null ||
		die "bootc did not retain configured image $target as a downloaded-only deployment"
}

fallback_unlock() {
	require_dir
	require_guest_endpoint
	need jq
	require_fallback_references
	operations=$(fallback_operations_dir)
	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	before=$operations/unlock-$stamp-before.json
	after=$operations/unlock-$stamp-after.json
	privileged_bootc_status >"$before"
	staged_reference=$(jq -r '.status.staged.image.image.image // empty' "$before")
	staged_digest=$(jq -r '.status.staged.image.imageDigest // empty' "$before")
	[ "$(jq -r '.status.staged.downloadOnly // false' "$before")" = true ] ||
		die "bootc has no downloaded-only deployment to unlock"
	if [ "$staged_reference" != "$(fallback_reference a)" ] && [ "$staged_reference" != "$(fallback_reference b)" ]; then
		die "downloaded deployment is not configured image A or B"
	fi
	credentials=$(password_file)
	admin_ssh "sudo -k -S -p '' /usr/bin/bootc switch --from-downloaded" <"$credentials" \
		>"$operations/unlock-$stamp.stdout" 2>"$operations/unlock-$stamp.stderr"
	privileged_bootc_status >"$after"
	jq -e --arg reference "$staged_reference" --arg digest "$staged_digest" '
		.status.staged.image.image.image == $reference and
		.status.staged.image.imageDigest == $digest and
		.status.staged.downloadOnly == false
	' "$after" >/dev/null || die "bootc did not create the configured staged deployment"
}

emit_registry_enable() {
	registry=$1
	cat <<EOF
install -d -m 0755 /etc/containers/registries.conf.d
cat > /etc/containers/registries.conf.d/99-soda-acceptance.conf <<'SODA_REGISTRY'
[[registry]]
location = "$registry"
insecure = true
SODA_REGISTRY
chmod 0644 /etc/containers/registries.conf.d/99-soda-acceptance.conf
EOF
}

emit_registry_disable() {
	cat <<'EOF'
file=/etc/containers/registries.conf.d/99-soda-acceptance.conf
test -f "$file"
rm -- "$file"
EOF
}

fallback_registry() {
	action=$1
	require_dir
	require_guest_endpoint
	require_fallback_references
	reference=$(fallback_reference b)
	registry=${reference%%/*}
	printf '%s\n' "$registry" | LC_ALL=C grep -Eq '^10\.0\.2\.2:[0-9]{1,5}$' ||
		die "fallback registry must be the fixed QEMU host endpoint"
	operations=$(fallback_operations_dir)
	case "$action" in
		enable)
			credentials=$(password_file)
			{
				cat "$credentials"
				emit_registry_enable "$registry"
			} | admin_ssh 'sudo -k -S -p "" /bin/bash -eu -o pipefail -s' >"$operations/registry-enable.txt"
			;;
		disable)
			run_privileged_script emit_registry_disable >"$operations/registry-disable.txt"
			;;
		*) die "fallback registry action must be enable or disable" ;;
	esac
}

emit_mutate_accounts() {
	cat <<'EOF'
test "$(id -u)" -eq 0
getent passwd alice >/dev/null
getent passwd obsolete >/dev/null
! getent passwd bob >/dev/null
alice_password=$(openssl rand -base64 32 | tr -d '\n')
printf 'alice:%s\n' "$alice_password" | /usr/sbin/chpasswd
unset alice_password
/usr/sbin/usermod --append --groups wheel -- alice

/usr/sbin/useradd --create-home --user-group --shell /bin/bash -- bob
bob_home=$(getent passwd bob | cut -d: -f6)
bob_group=$(id -gn bob)
install -d -m 0700 -o bob -g "$bob_group" "$bob_home/.ssh"
install -m 0600 -o bob -g "$bob_group" /home/soda-test/.ssh/authorized_keys "$bob_home/.ssh/authorized_keys"
/usr/sbin/runuser --user bob -- /usr/bin/env HOME="$bob_home" /bin/sh -c \
	' umask 077; printf "mutate-b:bob\n" >"$HOME/soda-acceptance-state.txt" '
bob_password=$(openssl rand -base64 32 | tr -d '\n')
printf 'bob:%s\n' "$bob_password" | /usr/sbin/chpasswd
unset bob_password
restorecon -RF "$bob_home"

loginctl terminate-user obsolete 2>/dev/null || true
/usr/sbin/userdel --remove -- obsolete
! getent passwd obsolete >/dev/null
test ! -e /home/obsolete
EOF
}

emit_mutate_workspace() {
	cat <<'EOF'
test "$(id -u)" -eq 0
marker=soda-workspace=soda-test/kept
workspace=$(getent passwd | awk -F: -v marker="$marker" '$5 == marker {print $1}')
test -n "$workspace"
home=$(getent passwd "$workspace" | cut -d: -f6)
/usr/sbin/runuser --user "$workspace" -- /usr/bin/env HOME="$home" /bin/sh -c \
	' printf "mutate-b:kept\n" >>"$HOME/Projects/kept/soda-acceptance-state.txt" '
restorecon -RF "$home"
EOF
}

fallback_mutate_b() {
	require_dir
	require_guest_endpoint
	require_fallback_references
	need jq
	[ "$admin" = soda-test ] || die "fallback mutate-b requires the installer administrator soda-test"
	fallback_assert_booted b
	operations=$(fallback_operations_dir)
	need_file "$operations/seed-a.complete"
	need_file "$(fallback_root)/a-installed/manifest.json"
	need_file "$(fallback_root)/b-updated/manifest.json"
	fallback_compare a-installed b-updated
	[ ! -e "$operations/mutate-b.complete" ] || die "fallback mutate-b has already completed"
	run_privileged_script emit_mutate_accounts >"$operations/mutate-b-accounts.txt"
	kept_url=$(admin_ssh "jq -er '.[] | select(.id == \"kept\") | .canonical_url' /var/lib/soda/catalog/projects.json")
	jq -cn --arg id kept --arg display_name 'Kept after B' --arg canonical_url "$kept_url" \
		'{id:$id,display_name:$display_name,canonical_url:$canonical_url}' |
		admin_ssh /usr/libexec/soda/soda-projects edit >"$operations/mutate-b-kept-edit.json"
	run_privileged_script emit_mutate_workspace >"$operations/mutate-b-kept-workspace.txt"
	project_password_request create-forgejo new 'New on B' >"$operations/mutate-b-new-create.json"
	project_password_request setup new '' >"$operations/mutate-b-new-setup.json"
	project_request remove removed >"$operations/mutate-b-removed-remove.json"
	capture_forgejo_state >"$operations/mutate-b-forgejo.json"
	jq -e 'any(.repositories[]; .name == "removed" and .owner == "soda-test")' \
		"$operations/mutate-b-forgejo.json" >/dev/null || die "project removal deleted the canonical Forgejo repository"
	date -u +%Y-%m-%dT%H:%M:%SZ >"$operations/mutate-b.complete"
}

emit_fallback_state() {
	cat <<'EOF'
account_json() {
	username=$1
	if ! record=$(getent passwd -- "$username"); then
		expected_home=/home/$username
		if test -e "$expected_home"; then home_present=true; else home_present=false; fi
		jq -cn --arg username "$username" --arg expected_home "$expected_home" --argjson home_present "$home_present" \
			'{username:$username,present:false,expected_home:{path:$expected_home,present:$home_present}}'
		return
	fi
	IFS=: read -r actual _ uid gid gecos home shell <<<"$record"
	test "$actual" = "$username"
	groups=$(id -nG "$username" | tr ' ' '\n' | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))')
	shadow_record=$(getent shadow -- "$username")
	test -n "$shadow_record"
	shadow_sha=$(printf '%s\n' "$shadow_record" | sha256sum | awk '{print $1}')
	if test -d "$home"; then
		read -r home_uid home_gid home_mode <<<"$(stat -c '%u %g %a' "$home")"
		home_state=$(jq -cn --argjson uid "$home_uid" --argjson gid "$home_gid" --arg mode "$home_mode" \
			'{present:true,uid:$uid,gid:$gid,mode:$mode}')
	else
		home_state='{"present":false}'
	fi
	keys=$home/.ssh/authorized_keys
	if test -f "$keys"; then
		keys_sha=$(sha256sum "$keys" | awk '{print $1}')
		keys_state=$(jq -cn --arg sha256 "$keys_sha" '{present:true,sha256:$sha256}')
	else
		keys_state='{"present":false}'
	fi
	fixture=$home/soda-acceptance-state.txt
	if test -f "$fixture"; then
		fixture_sha=$(sha256sum "$fixture" | awk '{print $1}')
		fixture_state=$(jq -cn --arg sha256 "$fixture_sha" '{present:true,sha256:$sha256}')
	else
		fixture_state='{"present":false}'
	fi
	jq -cn --arg username "$username" --argjson uid "$uid" --argjson gid "$gid" \
		--arg gecos "$gecos" --arg home "$home" --arg shell "$shell" --arg shadow_sha "$shadow_sha" \
		--argjson groups "$groups" --argjson home_state "$home_state" --argjson keys_state "$keys_state" \
		--argjson fixture_state "$fixture_state" \
		'{username:$username,present:true,uid:$uid,gid:$gid,gecos:$gecos,home:$home,shell:$shell,
		  groups:$groups,shadow_record_sha256:$shadow_sha,home_state:$home_state,
		  authorized_keys:$keys_state,fixture:$fixture_state}'
}

workspace_json() {
	username=$1
	account=$(account_json "$username")
	marker=$(jq -r '.gecos' <<<"$account")
	case "$marker" in
		soda-workspace=*/*) ;;
		*) echo "invalid workspace marker for $username" >&2; return 1 ;;
	esac
	association=${marker#soda-workspace=}
	primary=${association%%/*}
	project=${association#*/}
	home=$(jq -r '.home' <<<"$account")
	checkout=$home/Projects/$project
	test -d "$checkout"
	read -r checkout_uid checkout_gid checkout_mode <<<"$(stat -c '%u %g %a' "$checkout")"
	files=$(
		find "$checkout" -path "$checkout/.git" -prune -o -type f -printf '%P\n' |
			LC_ALL=C sort |
			while IFS= read -r relative; do
				test -n "$relative" || continue
				file=$checkout/$relative
				read -r file_uid file_gid file_mode file_size <<<"$(stat -c '%u %g %a %s' "$file")"
				file_sha=$(sha256sum "$file" | awk '{print $1}')
				jq -cn --arg path "$relative" --argjson uid "$file_uid" --argjson gid "$file_gid" \
					--arg mode "$file_mode" --argjson size "$file_size" --arg sha256 "$file_sha" \
					'{path:$path,uid:$uid,gid:$gid,mode:$mode,size:$size,sha256:$sha256}'
			done | jq -sc '.'
	)
	run_as_workspace=(/usr/sbin/runuser --user "$username" -- /usr/bin/env HOME="$home" /usr/bin/git -C "$checkout")
	remote=$("${run_as_workspace[@]}" remote get-url origin)
	refs=$( { "${run_as_workspace[@]}" show-ref --head 2>/dev/null || true; } | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))')
	status=$("${run_as_workspace[@]}" status --porcelain=v1 --untracked-files=all | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))')
	jq -cn --argjson account "$account" --arg primary "$primary" --arg project "$project" \
		--arg checkout "$checkout" --argjson checkout_uid "$checkout_uid" --argjson checkout_gid "$checkout_gid" \
		--arg checkout_mode "$checkout_mode" --argjson files "$files" --arg remote "$remote" \
		--argjson refs "$refs" --argjson status "$status" \
		'{account:$account,primary:$primary,project_id:$project,checkout:$checkout,
		  checkout_state:{uid:$checkout_uid,gid:$checkout_gid,mode:$checkout_mode},
		  files:$files,git:{remote:$remote,refs:$refs,status:$status}}'
}

accounts=$(for username in soda-test alice obsolete bob; do account_json "$username"; done | jq -sc 'sort_by(.username)')
workspace_assertions=$(
	for project in kept removed new; do
		digest=$(printf 'soda-test\0%s' "$project" | sha256sum | awk '{print substr($1,1,24)}')
		account_json "soda-w-$digest" | jq -c --arg project "$project" '. + {expected_project_id:$project}'
	done | jq -sc 'sort_by(.expected_project_id)'
)
members=$(getent group soda-workspaces | cut -d: -f4 | tr ',' '\n' | sed '/^$/d' | LC_ALL=C sort)
workspaces=$(for username in $members; do workspace_json "$username"; done | jq -sc 'sort_by(.account.username)')

tailscale_status=$(tailscale status --json)
tailnet=$(jq -ce '
	if .BackendState != "Running" or .Self == null then error("Tailscale is not running") else
	  .Self | {node_id:.ID,public_key:.PublicKey,dns_name:.DNSName,addresses:(.TailscaleIPs|sort)}
	end
' <<<"$tailscale_status")
state_name=$(systemctl show tailscaled.service --property=StateDirectory --value | awk '{print $1}')
test -n "$state_name"
case "$state_name" in
	/*) tailscale_state_path=$state_name ;;
	*) tailscale_state_path=/var/lib/$state_name ;;
esac
test -d "$tailscale_state_path"
tailnet=$(jq -cn --argjson identity "$tailnet" --arg state_path "$tailscale_state_path" \
	'{identity:$identity,state_path:$state_path,state_path_present:true}')

host_keys=$(
	for key in /etc/ssh/ssh_host_*_key.pub; do
		test -f "$key"
		fingerprint=$(ssh-keygen -lf "$key")
		jq -cn --arg name "$(basename "$key")" --arg fingerprint "$fingerprint" \
			'{name:$name,fingerprint:$fingerprint}'
	done | jq -sc 'sort_by(.name)'
)
timer_state=$(systemctl is-enabled bootc-fetch-apply-updates.timer 2>/dev/null || true)
test "$timer_state" = masked

jq -cn --argjson accounts "$accounts" --argjson workspace_assertions "$workspace_assertions" \
	--argjson workspaces "$workspaces" --argjson tailnet "$tailnet" \
	--argjson host_keys "$host_keys" --arg timer_state "$timer_state" \
	'{accounts:$accounts,workspace_assertions:$workspace_assertions,workspaces:$workspaces,tailnet:$tailnet,ssh_host_keys:$host_keys,
	  automatic_update_timer:$timer_state}'
EOF
}

capture_forgejo_state() {
	admin_ssh '
		set -eu
		forgejo_url=$(printf "{}\n" | /usr/libexec/soda/soda-projects list | jq -er .forgejo_url)
		forgejo_get() {
			path=$1
			curl --fail --silent --show-error --request GET --url "$forgejo_url$path"
		}
		user=$(forgejo_get /api/v1/users/soda-test)
		repositories=$(forgejo_get "/api/v1/users/soda-test/repos?limit=100")
		jq -cn --argjson user "$user" --argjson repositories "$repositories" '\''
			{user:($user|{id,login,active,restricted}),
			 repositories:($repositories |
			   map(select(.name == "kept" or .name == "removed" or .name == "new") |
			       {id,name,owner:.owner.login,empty,clone_url,ssh_url}) |
			   sort_by(.name))}
		'\''
	'
}

fallback_workspace_ssh() {
	workspace_target=$1
	shift
	printf '%s\n' "$workspace_target" | LC_ALL=C grep -Eq '^soda-w-[0-9a-f]{24}$' ||
		die "invalid derived workspace username $workspace_target"
	workspace_key=${SODA_ACCEPTANCE_WORKSPACE_KEY:-${SODA_ACCEPTANCE_ADMIN_KEY:-}}
	[ -n "$workspace_key" ] || die "SODA_ACCEPTANCE_WORKSPACE_KEY or SODA_ACCEPTANCE_ADMIN_KEY is required"
	need_file "$workspace_key"
	need_file "$(known_hosts_path)"
	ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$workspace_key" -p "$guest_ssh_port" \
		"$workspace_target@$guest_host" "$@"
}

fallback_workspace_process() {
	checkpoint=$1
	workspace_target=$(printf '{}\n' | admin_ssh /usr/libexec/soda/soda-projects list |
		jq -er '.projects[] | select(.id == "kept") | .workspace_username')
	fallback_workspace_ssh "$workspace_target" '
		set -eu
		state=$HOME/.local/state/soda-acceptance
		mkdir -p "$state"
		boot_id=$(cat /proc/sys/kernel/random/boot_id)
		running=false
		if test -s "$state/fallback-workload.boot-id" && test "$(cat "$state/fallback-workload.boot-id")" = "$boot_id" && test -s "$state/fallback-workload.pid"; then
			pid=$(cat "$state/fallback-workload.pid")
			if kill -0 "$pid" 2>/dev/null && grep -Fzq soda-fallback-heartbeat "/proc/$pid/cmdline"; then
				running=true
			fi
		fi
		if test "$running" != true; then
			rm -f "$state/fallback-workload.pid" "$state/fallback-workload.heartbeat"
			nohup /bin/sh -c '\''while :; do date +%s >"$HOME/.local/state/soda-acceptance/fallback-workload.heartbeat"; sleep 2; done'\'' soda-fallback-heartbeat \
				</dev/null >"$state/fallback-workload.log" 2>&1 &
			pid=$!
			printf "%s\n" "$pid" >"$state/fallback-workload.pid"
			printf "%s\n" "$boot_id" >"$state/fallback-workload.boot-id"
		fi
		deadline=$(( $(date +%s) + 10 ))
		until test -s "$state/fallback-workload.heartbeat"; do
			test "$(date +%s)" -lt "$deadline"
			sleep 1
		done
		process_uid=$(stat -c %u "/proc/$pid")
		test "$process_uid" -eq "$(id -u)"
		printf "workspace=%s\nuid=%s\npid=%s\nboot_id=%s\nheartbeat=%s\n" \
			"$(id -un)" "$process_uid" "$pid" "$boot_id" "$(cat "$state/fallback-workload.heartbeat")"
	' >"$checkpoint/workspace-process.txt"
}

fallback_capture() {
	name=${1:-}
	valid_name "$name" || die "fallback capture requires a lowercase name containing only letters, digits, and hyphens"
	require_dir
	require_guest_endpoint
	require_fallback_references
	for command in curl diff jq sha256sum ssh; do need "$command"; done
	root=$(fallback_root)
	checkpoint=$root/$name
	[ ! -e "$checkpoint" ] || die "fallback checkpoint $checkpoint already exists"
	mkdir -p "$checkpoint"
	privileged_bootc_status >"$checkpoint/deployment.json"
	booted_digest=$(jq -r '.status.booted.image.imageDigest // empty' "$checkpoint/deployment.json")
	a_digest=$(fallback_reference a); a_digest=${a_digest##*@}
	b_digest=$(fallback_reference b); b_digest=${b_digest##*@}
	[ "$booted_digest" = "$a_digest" ] || [ "$booted_digest" = "$b_digest" ] ||
		die "booted deployment is neither configured image A nor image B"
	admin_ssh 'cat /proc/sys/kernel/random/boot_id' >"$checkpoint/boot-id.txt"
	admin_ssh 'cat /var/lib/soda/catalog/projects.json' >"$checkpoint/catalog.json"
	jq -e '
		type == "array" and
		all(.[]; type == "object" and keys == ["canonical_url","display_name","id"]) and
		([.[].id] == ([.[].id] | sort))
	' "$checkpoint/catalog.json" >/dev/null || die "installed catalog is not the exact sorted three-field representation"
	run_privileged_script emit_fallback_state >"$checkpoint/system.json"
	capture_forgejo_state >"$checkpoint/forgejo.json"
	jq -e '.user.login == "soda-test"' "$checkpoint/forgejo.json" >/dev/null ||
		die "Forgejo native user evidence is missing"
	fallback_workspace_process "$checkpoint"
	catalog_sha=$(sha256sum "$checkpoint/catalog.json" | awk '{print $1}')
	jq -S -n --argjson system "$(cat "$checkpoint/system.json")" \
		--argjson forgejo "$(cat "$checkpoint/forgejo.json")" \
		--argjson catalog "$(cat "$checkpoint/catalog.json")" --arg catalog_sha "$catalog_sha" \
		'$system + {catalog:{sha256:$catalog_sha,entries:$catalog},forgejo:$forgejo}' \
		>"$checkpoint/manifest.json"
}

fallback_compare() {
	expected=${1:-}
	actual=${2:-}
	valid_name "$expected" || die "fallback compare requires a valid expected checkpoint name"
	valid_name "$actual" || die "fallback compare requires a valid actual checkpoint name"
	require_dir
	need diff
	expected_manifest=$(fallback_root)/$expected/manifest.json
	actual_manifest=$(fallback_root)/$actual/manifest.json
	need_file "$expected_manifest"
	need_file "$actual_manifest"
	diff -u "$expected_manifest" "$actual_manifest"
}

fallback() {
	action=${1:-}
	shift || true
	case "$action" in
		seed-a) [ "$#" -eq 0 ] || die "fallback seed-a accepts no arguments"; fallback_seed_a ;;
		seed-b) [ "$#" -eq 0 ] || die "fallback seed-b accepts no arguments"; fallback_seed_b ;;
		registry-enable) [ "$#" -eq 0 ] || die "fallback registry-enable accepts no arguments"; fallback_registry enable ;;
		registry-disable) [ "$#" -eq 0 ] || die "fallback registry-disable accepts no arguments"; fallback_registry disable ;;
		capture) [ "$#" -eq 1 ] || die "fallback capture requires one checkpoint name"; fallback_capture "$1" ;;
		stage) [ "$#" -eq 1 ] || die "fallback stage requires a or b"; fallback_stage "$1" ;;
		unlock) [ "$#" -eq 0 ] || die "fallback unlock accepts no arguments"; fallback_unlock ;;
		mutate-b) [ "$#" -eq 0 ] || die "fallback mutate-b accepts no arguments"; fallback_mutate_b ;;
		compare) [ "$#" -eq 2 ] || die "fallback compare requires expected and actual checkpoint names"; fallback_compare "$1" "$2" ;;
		*) die "fallback requires seed-a, seed-b, registry-enable, registry-disable, capture, stage, unlock, mutate-b, or compare" ;;
	esac
}

project_workspace() {
	project_id=${1:-}
	valid_name "$project_id" || die "project-workspace requires a valid project ID"
	require_dir
	require_guest_endpoint
	printf '{}\n' | admin_ssh /usr/libexec/soda/soda-projects list |
		jq -er --arg id "$project_id" '.projects[] | select(.id == $id) | .workspace_username | select(length > 0)'
}

primary_project_request() {
	username=$1
	action=$2
	request=$3
	printf '%s\n' "$username" | LC_ALL=C grep -Eq '^[a-z][a-z0-9-]{0,23}$' || die "invalid primary username $username"
	need_file "$(known_hosts_path)"
	printf '%s\n' "$request" | ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$SODA_ACCEPTANCE_ADMIN_KEY" -p "$guest_ssh_port" \
		"$username@$guest_host" "/usr/libexec/soda/soda-projects '$action'"
}

missing_key_project_request() {
	request=$1
	need_file "$(known_hosts_path)"
	printf '%s\n' "$request" | ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$SODA_ACCEPTANCE_ADMIN_KEY" -p "$guest_ssh_port" \
		"nokey@$guest_host" 'rm -f "$HOME/.ssh/authorized_keys"; exec /usr/libexec/soda/soda-projects setup'
}

emit_product_accounts() {
	cat <<'EOF'
test "$(id -u)" -eq 0
for username in nokey charlie dana; do
	! getent passwd "$username" >/dev/null
	/usr/sbin/useradd --create-home --user-group --shell /bin/bash -- "$username"
done
for username in nokey charlie dana; do
	home=$(getent passwd "$username" | cut -d: -f6)
	group=$(id -gn "$username")
	install -d -m 0700 -o "$username" -g "$group" "$home/.ssh"
	install -m 0600 -o "$username" -g "$group" /home/soda-test/.ssh/authorized_keys "$home/.ssh/authorized_keys"
	restorecon -RF "$home"
done
EOF
}

emit_generic_delete() {
	cat <<'EOF'
test "$(id -u)" -eq 0
uid=$(id -u dana)
/usr/bin/loginctl terminate-user dana >/dev/null 2>&1 || true
deadline=$(( $(date +%s) + 10 ))
while /usr/bin/pgrep -u "$uid" >/dev/null 2>&1; do
	[ "$(date +%s)" -lt "$deadline" ] || {
		echo "Dana still owns processes after native logind termination" >&2
		exit 1
	}
	sleep 1
done
/usr/sbin/userdel --remove -- dana
! getent passwd dana >/dev/null
test ! -e /home/dana
EOF
}

emit_generic_workspace_preserved() {
	cat <<EOF
test "\$(id -u)" -eq 0
getent passwd '$dana_workspace' >/dev/null
test -d '/home/$dana_workspace/Projects/generic'
EOF
}

emit_keyless_delete() {
	cat <<'EOF'
test "$(id -u)" -eq 0
uid=$(id -u nokey)
/usr/bin/loginctl terminate-user nokey >/dev/null 2>&1 || true
deadline=$(( $(date +%s) + 10 ))
while /usr/bin/pgrep -u "$uid" >/dev/null 2>&1; do
	[ "$(date +%s)" -lt "$deadline" ] || {
		echo "Keyless fixture still owns processes after native logind termination" >&2
		exit 1
	}
	sleep 1
done
/usr/sbin/userdel --remove -- nokey
! getent passwd nokey >/dev/null
test ! -e /home/nokey
EOF
}

set_workspace_marker() {
	username=$1
	marker=$2
	printf '%s\n' "$username" | LC_ALL=C grep -Eq '^soda-w-[0-9a-f]{24}$' || die "invalid workspace fixture username"
	case "$marker" in
		unexpected|soda-workspace=soda-test/failure-order) ;;
		*) die "invalid workspace fixture marker" ;;
	esac
	credentials=$(password_file)
	{
		cat "$credentials"
		printf '/usr/sbin/usermod --comment %s -- %s\n' "$marker" "$username"
	} | admin_ssh 'sudo -k -S -p "" /bin/bash -eu -o pipefail -s'
}

scenario_product() {
	require_dir
	require_guest_endpoint
	for command in curl jq scp sftp ssh; do need "$command"; done
	operations=$acceptance_dir/product-scenarios
	[ ! -e "$operations" ] || die "product scenarios already ran"
	mkdir -p "$operations"

	# The installed catalog is the exact sorted three-field product fact.
	admin_ssh 'cat /var/lib/soda/catalog/projects.json' >"$operations/catalog-before.json"
	jq -e 'type == "array" and all(.[]; keys == ["canonical_url","display_name","id"]) and ([.[].id] == ([.[].id] | sort))' \
		"$operations/catalog-before.json" >/dev/null

	# Exercise stock Cockpit authentication without placing passwords in argv.
	password=$(password_file)
	{
		printf 'user = "%s:%s"\n' "$admin" "$(tr -d '\r\n' <"$password")"
		printf 'insecure\nsilent\nshow-error\noutput = "%s"\nwrite-out = "%%{http_code}"\n' "$operations/cockpit-primary.body"
	} | curl --config - --request GET "https://$guest_host:$guest_cockpit_port/cockpit/login" \
		>"$operations/cockpit-primary.status"
	[ "$(cat "$operations/cockpit-primary.status")" = 200 ] || die "primary Cockpit authentication failed"
	kept_workspace=$(project_workspace kept)
	{
		printf 'user = "%s:locked-workspace-password"\n' "$kept_workspace"
		printf 'insecure\nsilent\nshow-error\noutput = "%s"\nwrite-out = "%%{http_code}"\n' "$operations/cockpit-workspace.body"
	} | curl --config - --request GET "https://$guest_host:$guest_cockpit_port/cockpit/login" \
		>"$operations/cockpit-workspace.status"
	[ "$(cat "$operations/cockpit-workspace.status")" = 401 ] || die "workspace Cockpit authentication was not rejected"

	run_privileged_script emit_product_accounts >"$operations/accounts.txt"
	# Missing standard keys must fail before creating a derived account.
	if missing_key_project_request '{"id":"kept","git_username":"","git_password":""}' \
		>"$operations/missing-key.stdout" 2>"$operations/missing-key.stderr"; then
		die "workspace setup unexpectedly accepted a primary account without keys"
	fi
	nokey_digest=$(printf 'nokey\0kept' | sha256sum | awk '{print substr($1,1,24)}')
	admin_ssh "! getent passwd 'soda-w-$nokey_digest' >/dev/null"

	# Two humans set up the same native repository as independent Linux users.
	for username in alice bob; do
		primary_project_request "$username" setup '{"id":"kept","git_username":"","git_password":""}' \
			>"$operations/$username-setup.json"
	done
	alice_workspace=$(jq -er '.workspace_username' "$operations/alice-setup.json")
	bob_workspace=$(jq -er '.workspace_username' "$operations/bob-setup.json")
	[ "$alice_workspace" != "$bob_workspace" ] || die "two humans received the same workspace account"
	for workspace in "$alice_workspace" "$bob_workspace"; do
		printf '%s\n' "$workspace" | LC_ALL=C grep -Eq '^soda-w-[0-9a-f]{24}$' || die "invalid workspace username"
		fallback_workspace_ssh "$workspace" 'test "$(id -u)" -eq "$(stat -c %u "$HOME")"; test -d "$HOME/Projects/kept/.git"; printf "%s\n" "$(id -un):$(id -u):$HOME"'
	done >"$operations/workspace-identities.txt"
	[ "$(fallback_workspace_ssh "$alice_workspace" 'id -u')" != "$(fallback_workspace_ssh "$bob_workspace" 'id -u')" ] ||
		die "two humans received the same workspace UID"
	fallback_workspace_ssh "$alice_workspace" 'printf alice-private >"$HOME/Projects/kept/alice-private.txt"; nohup sleep 300 </dev/null > /dev/null 2>&1 & echo $! >"$HOME/alice-process.pid"'
	fallback_workspace_ssh "$bob_workspace" 'test ! -e "$HOME/Projects/kept/alice-private.txt"; printf bob-private >"$HOME/Projects/kept/bob-private.txt"'

	# Direct command, SCP, and SFTP use ordinary OpenSSH as the derived UID.
	fallback_workspace_ssh "$alice_workspace" 'test "$(id -un)" = '"$alice_workspace"'; pwd' >"$operations/direct-command.txt"
	printf 'id -un\nexit\n' | ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "${SODA_ACCEPTANCE_ADMIN_KEY}" -p "$guest_ssh_port" \
		"$alice_workspace@$guest_host" >"$operations/direct-shell.txt"
	grep -Fx "$alice_workspace" "$operations/direct-shell.txt" >/dev/null || die "direct workspace shell did not run as the derived UID"
	printf 'scp-product-evidence\n' >"$operations/scp-input.txt"
	scp -q -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "${SODA_ACCEPTANCE_ADMIN_KEY}" -P "$guest_ssh_port" \
		"$operations/scp-input.txt" "$alice_workspace@$guest_host:scp-input.txt"
	fallback_workspace_ssh "$alice_workspace" 'test "$(cat "$HOME/scp-input.txt")" = scp-product-evidence'
	printf 'pwd\nls -l scp-input.txt\nquit\n' | sftp -q -b - \
		-o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "${SODA_ACCEPTANCE_ADMIN_KEY}" -P "$guest_ssh_port" \
		"$alice_workspace@$guest_host" >"$operations/sftp.txt"

	# Projects choose non-conflicting host ports without a Soda port registry.
	fallback_workspace_ssh "$alice_workspace" 'nohup python3 -m http.server 18080 --directory "$HOME/Projects/kept" </dev/null >"$HOME/port.log" 2>&1 & echo $! >"$HOME/port.pid"'
	fallback_workspace_ssh "$bob_workspace" 'nohup python3 -m http.server 18081 --directory "$HOME/Projects/kept" </dev/null >"$HOME/port.log" 2>&1 & echo $! >"$HOME/port.pid"'
	for port_and_file in 18080:alice-private.txt 18081:bob-private.txt; do
		port=${port_and_file%%:*}
		file=${port_and_file#*:}
		deadline=$(( $(date +%s) + 20 ))
		until curl --fail --silent --show-error "http://$guest_host:$port/$file" >"$operations/$file"; do
			[ "$(date +%s)" -lt "$deadline" ] || die "project-owned port $port did not become reachable"
			sleep 1
		done
	done

	# Catalog edits do not reconcile an already published workspace.
	before_remote=$(fallback_workspace_ssh "$kept_workspace" 'git -C "$HOME/Projects/kept" remote get-url origin')
	kept_url=$(jq -er '.[] | select(.id == "kept") | .canonical_url' "$operations/catalog-before.json")
	jq -cn --arg id kept --arg display_name 'Kept after catalog edit' --arg canonical_url "$kept_url" \
		'{id:$id,display_name:$display_name,canonical_url:$canonical_url}' |
		admin_ssh /usr/libexec/soda/soda-projects edit >"$operations/catalog-edit.json"
	[ "$(fallback_workspace_ssh "$kept_workspace" 'git -C "$HOME/Projects/kept" remote get-url origin')" = "$before_remote" ] ||
		die "catalog edit reconciled an existing checkout"

	# Soda-aware human deletion is cascading and deletes the primary last.
	capture_forgejo_state >"$operations/forgejo-before-human-delete.json"
	primary_project_request charlie setup '{"id":"kept","git_username":"","git_password":""}' >"$operations/charlie-setup.json"
	charlie_workspace=$(jq -er '.workspace_username' "$operations/charlie-setup.json")
	jq -cn '{username:"charlie"}' | admin_ssh /usr/libexec/soda/soda-projects delete-human >"$operations/charlie-delete.json"
	admin_ssh "! getent passwd charlie >/dev/null; ! getent passwd '$charlie_workspace' >/dev/null; test ! -e /home/charlie; test ! -e '/home/$charlie_workspace'"
	capture_forgejo_state >"$operations/forgejo-after-human-delete.json"
	diff -u "$operations/forgejo-before-human-delete.json" "$operations/forgejo-after-human-delete.json"

	# Generic Linux deletion is deliberately non-cascading.
	project_password_request create-forgejo generic 'Generic deletion fixture' >"$operations/generic-create.json"
	primary_project_request dana setup '{"id":"generic","git_username":"","git_password":""}' >"$operations/dana-setup.json"
	dana_workspace=$(jq -er '.workspace_username' "$operations/dana-setup.json")
	printf '%s\n' "$dana_workspace" | LC_ALL=C grep -Eq '^soda-w-[0-9a-f]{24}$' || die "invalid Dana workspace username"
	run_privileged_script emit_generic_delete >"$operations/dana-generic-delete.txt"
	run_privileged_script emit_generic_workspace_preserved >"$operations/dana-workspace-preserved.txt"
	project_request remove generic >"$operations/generic-remove.json"
	admin_ssh "! getent passwd '$dana_workspace' >/dev/null; test ! -e '/home/$dana_workspace'"

	# Project removal terminates every derived account and preserves Forgejo.
	project_password_request create-forgejo removable 'Removal fixture' >"$operations/removable-create.json"
	project_password_request setup removable '' >"$operations/removable-admin-setup.json"
	primary_project_request alice setup '{"id":"removable","git_username":"","git_password":""}' >"$operations/removable-alice-setup.json"
	removable_admin=$(jq -er '.workspace_username' "$operations/removable-admin-setup.json")
	removable_alice=$(jq -er '.workspace_username' "$operations/removable-alice-setup.json")
	fallback_workspace_ssh "$removable_alice" 'nohup sleep 300 </dev/null >/dev/null 2>&1 &'
	project_request remove removable >"$operations/removable-remove.json"
	admin_ssh "! getent passwd '$removable_admin' >/dev/null; ! getent passwd '$removable_alice' >/dev/null; jq -e 'all(.[]; .id != \"removable\")' /var/lib/soda/catalog/projects.json >/dev/null"
	admin_ssh 'forgejo_url=$(printf "{}\n" | /usr/libexec/soda/soda-projects list | jq -er .forgejo_url); curl --fail --silent "$forgejo_url/api/v1/repos/soda-test/removable" >/dev/null'

	# An ambiguous Linux association makes removal fail with the catalog intact.
	project_password_request create-forgejo failure-order 'Failure ordering fixture' >"$operations/failure-order-create.json"
	project_password_request setup failure-order '' >"$operations/failure-order-setup.json"
	failure_workspace=$(jq -er '.workspace_username' "$operations/failure-order-setup.json")
	set_workspace_marker "$failure_workspace" unexpected
	if project_request remove failure-order >"$operations/failure-order-remove.stdout" 2>"$operations/failure-order-remove.stderr"; then
		die "project removal accepted ambiguous workspace state"
	fi
	admin_ssh "jq -e 'any(.[]; .id == \"failure-order\")' /var/lib/soda/catalog/projects.json >/dev/null"
	set_workspace_marker "$failure_workspace" soda-workspace=soda-test/failure-order
	project_request remove failure-order >"$operations/failure-order-cleanup.json"

	# Clean the keyless fixture through ordinary Linux ownership.
	run_privileged_script emit_keyless_delete >"$operations/nokey-delete.txt"
	date -u +%Y-%m-%dT%H:%M:%SZ >"$operations/pass.txt"
}

scenario() {
	case "${1:-}" in
		product) scenario_product ;;
		*) die "scenario requires product" ;;
	esac
}

workload() {
	action=${1:-}
	require_dir
	case "$action" in
		start)
			workspace_ssh 'mkdir -p "$HOME/.local/state/soda-acceptance"; test ! -s "$HOME/.local/state/soda-acceptance/workload.pid" || ! kill -0 "$(cat "$HOME/.local/state/soda-acceptance/workload.pid")" 2>/dev/null; nohup sh -c '\''while :; do date +%s > "$HOME/.local/state/soda-acceptance/workload.heartbeat"; sleep 2; done'\'' >"$HOME/.local/state/soda-acceptance/workload.log" 2>&1 & echo $! > "$HOME/.local/state/soda-acceptance/workload.pid"'
			workload verify
			;;
		verify)
			workspace_ssh 'pid=$(cat "$HOME/.local/state/soda-acceptance/workload.pid"); kill -0 "$pid"; printf "pid=%s\nheartbeat=%s\n" "$pid" "$(cat "$HOME/.local/state/soda-acceptance/workload.heartbeat")"' |
				tee "$acceptance_dir/workload-$(date -u +%Y%m%dT%H%M%SZ).txt"
			;;
		*) die "workload requires start or verify" ;;
	esac
}

stop_vm() {
	require_dir
	qmp '{"execute":"system_powerdown"}' | tee "$acceptance_dir/qmp-powerdown.json"
}

command=${1:-help}
case "$command" in
	help|-h|--help) usage ;;
	launch) shift; launch "${1:-}" ;;
	wait) shift; [ "$#" -eq 0 ] || die "wait accepts no arguments"; wait_ready ;;
	capture) shift; capture "${1:-}" ;;
	fallback) shift; fallback "$@" ;;
	workload) shift; workload "${1:-}" ;;
	project-workspace) shift; [ "$#" -eq 1 ] || die "project-workspace requires one ID"; project_workspace "$1" ;;
	scenario) shift; [ "$#" -eq 1 ] || die "scenario requires one name"; scenario "$1" ;;
	stop) shift; [ "$#" -eq 0 ] || die "stop accepts no arguments"; stop_vm ;;
	*) usage >&2; die "unknown command $command" ;;
esac
