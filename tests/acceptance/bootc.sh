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
  SODA_ACCEPTANCE_ADMIN_KEY        Private SSH key for the Anaconda administrator
  SODA_ACCEPTANCE_IMAGE_DIGEST     Expected sha256:... release digest for capture
  SODA_ACCEPTANCE_GUEST_HOST       Guest Tailnet IP or MagicDNS name

Additional launch install environment:
  SODA_ACCEPTANCE_ISO              Exact-digest Soda installer ISO
  SODA_ACCEPTANCE_KICKSTART_ISO    Optional test-only OEMDRV automation ISO

Additional workload environment:
  SODA_ACCEPTANCE_WORKSPACE_TARGET Soda person's Linux username, for example vince
  SODA_ACCEPTANCE_PROJECT          Project slug selected through SODA_PROJECT
  SODA_ACCEPTANCE_WORKSPACE_KEY    Registered Soda device private key

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
  SODA_ACCEPTANCE_VNC=127.0.0.1:1  Loopback VNC endpoint for x86-64 graphical install
  SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE=<test-only administrator password file>
  SODA_ACCEPTANCE_DISK=$SODA_ACCEPTANCE_DIR/soda-system.qcow2
  SODA_ACCEPTANCE_DISK_SIZE=40G
  SODA_ACCEPTANCE_RELEASE_INDEX_URL=<optional GitHub release index URL>
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
	workspace_project=${SODA_ACCEPTANCE_PROJECT:-}
	workspace_key=${SODA_ACCEPTANCE_WORKSPACE_KEY:-}
	[ -n "$workspace_target" ] || die "SODA_ACCEPTANCE_WORKSPACE_TARGET is required"
	[ -n "$workspace_project" ] || die "SODA_ACCEPTANCE_PROJECT is required"
	[ -n "$workspace_key" ] || die "SODA_ACCEPTANCE_WORKSPACE_KEY is required"
	need_file "$workspace_key"
	need_file "$(known_hosts_path)"
	ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
		-o "SetEnv=SODA_PROJECT=$workspace_project" \
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
			rm -f "$installer_input" "$eject_tmp"
			if [ "$ejected" != true ] && kill -0 "$qemu_pid" 2>/dev/null; then
				kill -TERM "$qemu_pid" 2>/dev/null || true
			fi
		}
		abort_ejector() {
			trap - 1 2 15
			exit 1
		}
		trap cleanup_ejector 0
		trap abort_ejector 1 2 15

		deadline=$(( $(date +%s) + 600 ))
		until grep -a -E -q 'because of an automated install|explicitly asked for in kickstart' "$acceptance_dir/serial.log" 2>/dev/null; do
			kill -0 "$qemu_pid" 2>/dev/null || exit 1
			[ "$(date +%s)" -lt "$deadline" ] || die "Anaconda did not confirm consuming the Kickstart within 600 seconds"
			sleep 1
		done

		qmp '{"execute":"blockdev-open-tray","arguments":{"id":"soda-oemdrv-device","force":true}}' >"$eject_tmp"
		qmp '{"execute":"blockdev-remove-medium","arguments":{"id":"soda-oemdrv-device"}}' >>"$eject_tmp"
		qmp '{"execute":"query-block"}' >>"$eject_tmp"
		! grep -q '"error"' "$eject_tmp" || die "QEMU rejected removal of the installer input"
		jq -e 'select((.return? | type) == "array") | .return[] | select(.device == "soda-oemdrv" and (has("inserted") | not))' "$eject_tmp" >/dev/null ||
			die "QEMU still exposes the installer input"

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
		if [ -n "$kickstart_iso" ]; then
			need_file "$kickstart_iso"
			need jq
			installer_args="$installer_args -drive if=none,file=$kickstart_iso,format=raw,readonly=on,id=soda-oemdrv -device virtio-scsi-pci,id=soda-oemdrv-scsi -device scsi-cd,drive=soda-oemdrv,id=soda-oemdrv-device"
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
	[ -z "${kickstart_iso:-}" ] || start_installer_input_ejector "$kickstart_iso"
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
	vnc=${SODA_ACCEPTANCE_VNC:-}
	if [ -n "$vnc" ]; then
		case "$vnc" in
			127.0.0.1:[0-9]|127.0.0.1:[0-9][0-9]) ;;
			*) die "SODA_ACCEPTANCE_VNC must be a loopback display such as 127.0.0.1:1" ;;
		esac
		set -- -vnc "$vnc"
		display_command="-vnc $vnc"
	else
		set -- -display none
		display_command="-display none"
	fi
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
$display_command
EOF
	# Word splitting is intentional for the fixed, locally constructed installer arguments.
	# shellcheck disable=SC2086
	[ -z "${kickstart_iso:-}" ] || start_installer_input_ejector "$kickstart_iso"
	exec "$qemu" -machine q35,accel=kvm -cpu host -smp 4 -m 4096 \
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
	while ! curl --fail --silent --show-error --insecure "https://$guest_host:$guest_cockpit_port/healthz" >/dev/null 2>&1; do
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

capture_release_index() {
	index_url=${SODA_ACCEPTANCE_RELEASE_INDEX_URL:-}
	if [ -z "$index_url" ]; then
		echo "release index capture skipped: SODA_ACCEPTANCE_RELEASE_INDEX_URL is unset" >"$1/release-index.skipped"
		return
	fi
	release_dir=$1/release
	mkdir -p "$release_dir"
	curl --fail --silent --show-error -D "$release_dir/index.headers" -o "$release_dir/index.json" "$index_url"
	sha256sum "$release_dir"/* >"$release_dir/sha256sums.txt"
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
	if [ -n "$password_file" ]; then
		admin_ssh "sudo -S -p '' python3 -c 'import sqlite3; print(sqlite3.connect(\"/var/lib/soda/soda.db\").execute(\"PRAGMA user_version\").fetchone()[0])'" \
			<"$password_file" >"$checkpoint/state-schema.txt"
	fi
	qmp '{"execute":"query-status"}' >"$checkpoint/qmp-status.json"
	admin_ssh '
		echo "[identity]"; id
		echo "[time]"; date -u +%Y-%m-%dT%H:%M:%SZ
		echo "[boot-id]"; cat /proc/sys/kernel/random/boot_id
		echo "[kernel]"; uname -a
		echo "[services]"
		for unit in sodad sshd soda-cockpit soda-authd forgejo avahi-daemon tailscaled soda-state-directories.service var-srv-soda-projects.mount opt-soda-toolchains.mount; do
			printf "%s=" "$unit"; systemctl is-active "$unit" 2>/dev/null || true
		done
		echo "[failed-units]"; systemctl --failed --no-legend --plain || true
		echo "[built-in-git]"
		rpm -q soda-release soda-runtime soda-cockpit soda-forgejo
		forgejo --version
		getent passwd git
		test -s /etc/forgejo/app.ini && echo configuration=present
		test -s /var/lib/soda/built-in-git-token && echo automation-token=present
		test -d /var/lib/forgejo/data/repositories/soda && echo repositories=present
		echo "[boot-entries]"; efibootmgr -v 2>/dev/null || true
		echo "[automatic-update]"
		for unit in bootc-fetch-apply-updates.timer bootc-fetch-apply-updates.service; do
			printf "%s=" "$unit"; systemctl is-enabled "$unit" 2>/dev/null || true
		done
		echo "[mounts]"
		for target in /var/lib/soda /srv/soda/projects /opt/soda/toolchains; do
			findmnt "$target" 2>/dev/null || true
		done
		echo "[host-keys]"
		sha256sum /etc/ssh/ssh_host_*_key.pub 2>/dev/null || true
		echo "[console]"
		systemctl show getty@tty1.service autovt@tty1.service -p Id -p Names -p MainPID -p NRestarts
		sysctl kernel.printk
	' >"$checkpoint/guest.txt" 2>"$checkpoint/guest.stderr"
	curl --fail --silent --show-error --insecure "https://$guest_host:$guest_cockpit_port/healthz" >"$checkpoint/cockpit-health.txt"
	capture_release_index "$checkpoint"

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
