#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Usage: tests/acceptance/bootc.sh COMMAND [ARG]

Commands:
  launch install       Create a blank disk and boot the configured installer ISO
  launch installed     Boot the existing acceptance disk without installer media
  wait                 Wait for SSH and Cockpit, then prove key-based admin SSH
  capture NAME         Capture nonprivileged host, guest, QMP, and registry evidence
  workload start       Start the configured forced-command SSH continuity workload
  workload verify      Verify the continuity workload and record its heartbeat
  stop                 Request a clean ACPI shutdown through QMP

Required environment:
  SODA_ACCEPTANCE_DIR              Untracked evidence directory
  SODA_ACCEPTANCE_ARCHITECTURE     Sibling architecture: aarch64 or x86_64
  SODA_ACCEPTANCE_ADMIN_KEY        Public-key login identity for the Anaconda admin
  SODA_ACCEPTANCE_IMAGE_DIGEST     Expected sha256:... release digest for capture

Additional launch install environment:
  SODA_ACCEPTANCE_ISO              Exact-digest Soda installer ISO

Additional workload environment:
  SODA_ACCEPTANCE_WORKSPACE_TARGET Forced-command SSH target, for example soda-p-demo
  SODA_ACCEPTANCE_WORKSPACE_KEY    Registered Soda device private key

Optional environment:
  SODA_ACCEPTANCE_ADMIN=vince
  SODA_ACCEPTANCE_HOST=127.0.0.1
  SODA_ACCEPTANCE_SSH_PORT=2222
  SODA_ACCEPTANCE_COCKPIT_PORT=9090
  SODA_ACCEPTANCE_DISK=$SODA_ACCEPTANCE_DIR/soda-system.qcow2
  SODA_ACCEPTANCE_DISK_SIZE=40G
  SODA_ACCEPTANCE_RELEASE_INDEX_URL=<GitHub release index URL>
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

admin_ssh() {
	admin_key=${SODA_ACCEPTANCE_ADMIN_KEY:-}
	[ -n "$admin_key" ] || die "SODA_ACCEPTANCE_ADMIN_KEY is required"
	need_file "$admin_key"
	need_file "$(known_hosts_path)"
	ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$admin_key" -p "$ssh_port" \
		"$admin@$host" "$@"
}

workspace_ssh() {
	workspace_target=${SODA_ACCEPTANCE_WORKSPACE_TARGET:-}
	workspace_key=${SODA_ACCEPTANCE_WORKSPACE_KEY:-}
	[ -n "$workspace_target" ] || die "SODA_ACCEPTANCE_WORKSPACE_TARGET is required"
	[ -n "$workspace_key" ] || die "SODA_ACCEPTANCE_WORKSPACE_KEY is required"
	need_file "$workspace_key"
	need_file "$(known_hosts_path)"
	ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "UserKnownHostsFile=$(known_hosts_path)" -i "$workspace_key" -p "$ssh_port" \
		"$workspace_target@$host" "$@"
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
		if [ "$architecture" = aarch64 ]; then
			installer_args="-drive file=$iso,media=cdrom,if=virtio,format=raw,readonly=on"
		else
			installer_args="-drive file=$iso,media=cdrom,format=raw,readonly=on"
		fi
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
$qemu -machine virt,accel=hvf -cpu host -smp 4 -m 4096 -bios $firmware
-drive file=$disk,if=virtio,format=qcow2 $installer_args
-netdev user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090
EOF
	# Word splitting is intentional for the fixed, locally constructed installer arguments.
	# shellcheck disable=SC2086
	exec "$qemu" -machine virt,accel=hvf -cpu host -smp 4 -m 4096 -bios "$firmware" \
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
	if [ "$mode" = install ]; then
		cp "$vars_template" "$vars"
	else
		need_file "$vars"
	fi
	cat >"$acceptance_dir/qemu-command.txt" <<EOF
$qemu -machine q35,accel=kvm -cpu host -smp 4 -m 4096
-drive if=pflash,format=raw,readonly=on,file=$firmware
-drive if=pflash,format=raw,file=$vars
-drive file=$disk,if=virtio,format=qcow2 $installer_args
-netdev user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090
EOF
	# Word splitting is intentional for the fixed, locally constructed installer arguments.
	# shellcheck disable=SC2086
	exec "$qemu" -machine q35,accel=kvm -cpu host -smp 4 -m 4096 \
		-drive "if=pflash,format=raw,readonly=on,file=$firmware" \
		-drive "if=pflash,format=raw,file=$vars" \
		-drive "file=$disk,if=virtio,format=qcow2" $installer_args \
		-netdev "user,id=net0,hostfwd=tcp:$host:$ssh_port-:22,hostfwd=tcp:$host:$cockpit_port-:9090" \
		-device virtio-net-pci,netdev=net0 -display none \
		-chardev "stdio,id=serial0,signal=off,logfile=$acceptance_dir/serial.log" -serial chardev:serial0 \
		-monitor none -qmp "unix:$(qmp_path),server=on,wait=off"
}

wait_ready() {
	require_dir
	need nc
	need curl
	started=$(date +%s)
	deadline=$((started + 600))
	while ! nc -z "$host" "$ssh_port" >/dev/null 2>&1; do
		[ "$(date +%s)" -lt "$deadline" ] || die "SSH did not become ready within 600 seconds"
		sleep 2
	done
	while ! curl --fail --silent --show-error --insecure "https://$host:$cockpit_port/healthz" >/dev/null 2>&1; do
		[ "$(date +%s)" -lt "$deadline" ] || die "Cockpit did not become ready within 600 seconds"
		sleep 2
	done
	admin_ssh 'id; cat /proc/sys/kernel/random/boot_id'
	printf 'ready_at=%s\nelapsed_seconds=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(( $(date +%s) - started ))" |
		tee "$acceptance_dir/readiness.txt"
}

valid_name() {
	case "$1" in
		''|*[!a-z0-9-]*) return 1 ;;
		*) return 0 ;;
	esac
}

capture_release_index() {
	index_url=${SODA_ACCEPTANCE_RELEASE_INDEX_URL:-https://github.com/LevitateOS/soda-os/releases/latest/download/soda-os-release-index.json}
	release_dir=$1/release
	mkdir -p "$release_dir"
	curl --fail --silent --show-error -D "$release_dir/index.headers" -o "$release_dir/index.json" "$index_url"
	sha256sum "$release_dir"/* >"$release_dir/sha256sums.txt"
}

capture() {
	name=${1:-}
	valid_name "$name" || die "capture requires a lowercase name containing only letters, digits, and hyphens"
	require_dir
	checkpoint=$acceptance_dir/$name
	[ ! -e "$checkpoint" ] || die "checkpoint $checkpoint already exists"
	mkdir -p "$checkpoint"
	date -u +%Y-%m-%dT%H:%M:%SZ >"$checkpoint/captured-at.txt"
	qmp '{"execute":"query-status"}' >"$checkpoint/qmp-status.json"
	admin_ssh '
		echo "[identity]"; id
		echo "[time]"; date -u +%Y-%m-%dT%H:%M:%SZ
		echo "[boot-id]"; cat /proc/sys/kernel/random/boot_id
		echo "[kernel]"; uname -a
		echo "[services]"
		for unit in sodad sshd soda-cockpit soda-authd forgejo avahi-daemon soda-state-directories.service var-srv-soda-projects.mount opt-soda-toolchains.mount; do
			printf "%s=" "$unit"; systemctl is-active "$unit" 2>/dev/null || true
		done
		echo "[built-in-git]"
		rpm -q soda-forgejo
		forgejo --version
		getent passwd git
		test -s /etc/forgejo/app.ini && echo configuration=present
		test -s /var/lib/soda/built-in-git-token && echo automation-token=present
		test -d /var/lib/forgejo/data/repositories/soda && echo repositories=present
		echo "[bootc-status]"; bootc status --format=json
		echo "[boot-entries]"; efibootmgr -v 2>/dev/null || true
		echo "[automatic-update]"
		for unit in bootc-fetch-apply-updates.timer bootc-fetch-apply-updates.service; do
			printf "%s=" "$unit"; systemctl is-enabled "$unit" 2>/dev/null || true
		done
		echo "[mounts]"
		findmnt /var/lib/soda /srv/soda/projects /opt/soda/toolchains 2>/dev/null || true
		echo "[host-keys]"
		sha256sum /etc/ssh/ssh_host_*_key.pub 2>/dev/null || true
	' >"$checkpoint/guest.txt" 2>"$checkpoint/guest.stderr"
	curl --fail --silent --show-error --insecure "https://$host:$cockpit_port/healthz" >"$checkpoint/cockpit-health.txt"
	capture_release_index "$checkpoint"

	privileged=".local/state/soda-acceptance/$name-privileged.json"
	if admin_ssh "test -r \"\$HOME/$privileged\""; then
		scp -q -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
			-o "UserKnownHostsFile=$(known_hosts_path)" -i "${SODA_ACCEPTANCE_ADMIN_KEY}" -P "$ssh_port" \
			"$admin@$host:$privileged" "$checkpoint/privileged.json"
	else
		echo "missing operator evidence: $privileged" >"$checkpoint/privileged.missing"
	fi

	for artifact in "${SODA_ACCEPTANCE_RELEASE_RECORD:-}" "${SODA_ACCEPTANCE_ISO:-}"; do
		[ -z "$artifact" ] || [ ! -f "$artifact" ] || sha256sum "$artifact" >>"$checkpoint/artifact-sha256sums.txt"
	done
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
	workload) shift; workload "${1:-}" ;;
	stop) shift; [ "$#" -eq 0 ] || die "stop accepts no arguments"; stop_vm ;;
	*) usage >&2; die "unknown command $command" ;;
esac
