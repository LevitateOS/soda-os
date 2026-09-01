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
  fallback seed-a      Seed authoritative mutable state on image A
  fallback capture NAME
                       Capture normalized fallback-preservation evidence
  fallback stage a|b   Download only the configured exact A or B image
  fallback unlock      Create a deployment from the downloaded image
  fallback mutate-b    Mutate authoritative state while booted into image B
  fallback compare EXPECTED ACTUAL
                       Compare two normalized preservation manifests
  workload start       Start the configured direct-workspace SSH continuity workload
  workload verify      Verify the continuity workload and record its heartbeat
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
  SODA_ACCEPTANCE_KICKSTART_ISO    Optional test-only OEMDRV automation ISO

Additional workload environment:
  SODA_ACCEPTANCE_WORKSPACE_TARGET Derived Linux workspace username
  SODA_ACCEPTANCE_WORKSPACE_KEY    Primary user's standard SSH private key

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
	qmp '{"execute":"query-status"}' >"$checkpoint/qmp-status.json"
	admin_ssh '
		set -eu
		echo "[identity]"; id
		echo "[time]"; date -u +%Y-%m-%dT%H:%M:%SZ
		echo "[boot-id]"; cat /proc/sys/kernel/random/boot_id
		echo "[kernel]"; uname -a
		echo "[services]"
		for unit in sodad sshd cockpit.socket forgejo tailscaled soda-state-directories.service opt-soda-toolchains.mount; do
			printf "%s=" "$unit"; systemctl is-active "$unit" 2>/dev/null || true
		done
		echo "[failed-units]"; systemctl --failed --no-legend --plain || true
		echo "[native-git-host]"
		rpm -q soda-release soda-runtime soda-projects soda-forgejo
		forgejo --version
		getent passwd git
		test -s /etc/forgejo/app.ini && echo configuration=present
		echo "[deleted-workspace-control-plane]"
		for unit in soda-authd.service soda-cockpit.service var-srv-soda-projects.mount; do
			if systemctl cat "$unit" >/dev/null 2>&1; then
				echo "unexpected-unit=$unit"
				exit 1
			fi
			echo "$unit=absent"
		done
		for path in /var/lib/soda/soda.db /var/lib/soda/built-in-git-token /var/lib/soda/projects /var/srv/soda/projects /srv/soda/projects /etc/soda/authorized_keys /usr/libexec/soda/soda-ssh; do
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
		echo "[boot-entries]"; efibootmgr -v 2>/dev/null || true
		echo "[automatic-update]"
		for unit in bootc-fetch-apply-updates.timer bootc-fetch-apply-updates.service; do
			printf "%s=" "$unit"; systemctl is-enabled "$unit" 2>/dev/null || true
		done
		echo "[mounts]"
		for target in /var/lib/soda /opt/soda/toolchains; do
			findmnt "$target" 2>/dev/null || true
		done
		echo "[host-keys]"
		sha256sum /etc/ssh/ssh_host_*_key.pub 2>/dev/null || true
		echo "[console]"
		systemctl show getty@tty1.service autovt@tty1.service -p Id -p Names -p MainPID -p NRestarts
		sysctl kernel.printk
	' >"$checkpoint/guest.txt" 2>"$checkpoint/guest.stderr"
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
		capture) [ "$#" -eq 1 ] || die "fallback capture requires one checkpoint name"; fallback_capture "$1" ;;
		stage) [ "$#" -eq 1 ] || die "fallback stage requires a or b"; fallback_stage "$1" ;;
		unlock) [ "$#" -eq 0 ] || die "fallback unlock accepts no arguments"; fallback_unlock ;;
		mutate-b) [ "$#" -eq 0 ] || die "fallback mutate-b accepts no arguments"; fallback_mutate_b ;;
		compare) [ "$#" -eq 2 ] || die "fallback compare requires expected and actual checkpoint names"; fallback_compare "$1" "$2" ;;
		*) die "fallback requires seed-a, capture, stage, unlock, mutate-b, or compare" ;;
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
	stop) shift; [ "$#" -eq 0 ] || die "stop accepts no arguments"; stop_vm ;;
	*) usage >&2; die "unknown command $command" ;;
esac
