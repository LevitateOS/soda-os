#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Usage: tests/acceptance/libvirt.sh COMMAND MODEL

Commands:
  launch staging   Define and start the persistent Cockpit-managed staging VM
  launch test      Define and start the unattended acceptance VM
  wait test        Wait for the unattended VM and prove key-based SSH/Cockpit
  address MODEL    Print the current libvirt DHCP address
  status MODEL     Record and print libvirt domain status
  start MODEL      Start an existing libvirt domain
  stop MODEL       Request a clean shutdown of an existing libvirt domain

Models:
  staging          Persistent appliance controlled by the owner through Cockpit
  test             Disposable unattended appliance controlled by the test harness

Required environment for launch:
  SODA_ACCEPTANCE_DIR              Untracked evidence directory
  SODA_ACCEPTANCE_ISO              Exact Soda installer ISO

Additional test-model launch environment:
  SODA_ACCEPTANCE_RELEASE_RECORD   Release record for the exact ISO and image digest

Optional environment:
  SODA_ACCEPTANCE_ARCHITECTURE=x86_64
  SODA_ACCEPTANCE_DOMAIN_STAGING=soda-staging
  SODA_ACCEPTANCE_DOMAIN_TEST=soda-acceptance
  SODA_ACCEPTANCE_ADMIN=soda-test
  SODA_ACCEPTANCE_ADMIN_NAME=Soda Acceptance
  SODA_ACCEPTANCE_ADMIN_EMAIL=acceptance@soda.invalid
  SODA_ACCEPTANCE_DISK_SIZE=40G
  SODA_ACCEPTANCE_LIBVIRT_URI=qemu:///system
  SODA_ACCEPTANCE_STORAGE_DIR=/home/libvirt/images

The staging model uses the stock graphical installer and has no test credential.
The test model creates a per-run password and SSH key under SODA_ACCEPTANCE_DIR,
then supplies them through a separate OEMDRV Kickstart ISO. The Soda installer ISO
is not modified.
EOF
}

die() {
	echo "soda-libvirt-acceptance: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command $1 is unavailable"
}

need_file() {
	[ -f "$1" ] || die "required file $1 is unavailable"
}

model_domain() {
	case "$1" in
		staging) printf '%s\n' "${SODA_ACCEPTANCE_DOMAIN_STAGING:-soda-staging}" ;;
		test) printf '%s\n' "${SODA_ACCEPTANCE_DOMAIN_TEST:-soda-acceptance}" ;;
		*) die "model must be staging or test" ;;
	esac
}

require_native_x86() {
	architecture=${SODA_ACCEPTANCE_ARCHITECTURE:-x86_64}
	[ "$architecture" = x86_64 ] || die "the libvirt staging models currently require x86_64"
	case "$(uname -m)" in
		x86_64|amd64) ;;
		*) die "x86_64 libvirt staging requires matching native hardware" ;;
	esac
}

require_dir() {
	acceptance_dir=${SODA_ACCEPTANCE_DIR:-}
	[ -n "$acceptance_dir" ] || die "SODA_ACCEPTANCE_DIR is required"
	mkdir -p "$acceptance_dir"
	acceptance_dir=$(CDPATH= cd -- "$acceptance_dir" && pwd)
}

virsh_system() {
	sudo -n virsh -c "${SODA_ACCEPTANCE_LIBVIRT_URI:-qemu:///system}" "$@"
}

domain_exists() {
	virsh_system dominfo "$1" >/dev/null 2>&1
}

record_command() {
	model=$1
	domain=$2
	disk=$3
	seed=${4:-}
	cat >"$acceptance_dir/virt-install-command.txt" <<EOF
model=$model
domain=$domain
disk=$disk
installer=$iso
automation=${seed:-none}
connection=${SODA_ACCEPTANCE_LIBVIRT_URI:-qemu:///system}
EOF
}

prepare_disk() {
	domain=$1
	storage_dir=${SODA_ACCEPTANCE_STORAGE_DIR:-/home/libvirt/images}
	disk=$storage_dir/$domain.qcow2
	[ ! -e "$disk" ] || die "refusing to overwrite existing disk $disk"
	sudo -n install -d -m 0711 "$storage_dir"
	sudo -n qemu-img create -f qcow2 "$disk" "${SODA_ACCEPTANCE_DISK_SIZE:-40G}" >/dev/null
	printf '%s\n' "$disk"
}

prepare_test_identity() {
	admin=${SODA_ACCEPTANCE_ADMIN:-soda-test}
	admin_name=${SODA_ACCEPTANCE_ADMIN_NAME:-Soda Acceptance}
	admin_email=${SODA_ACCEPTANCE_ADMIN_EMAIL:-acceptance@soda.invalid}
	admin_key=$acceptance_dir/admin
	password_file=$acceptance_dir/admin-password
	if [ ! -e "$admin_key" ] && [ ! -e "$admin_key.pub" ]; then
		ssh-keygen -q -t ed25519 -N '' -C "$admin@$domain" -f "$admin_key"
	fi
	need_file "$admin_key"
	need_file "$admin_key.pub"
	if [ ! -e "$password_file" ]; then
		umask 077
		openssl rand -base64 24 | tr -d '\n' >"$password_file"
		printf '\n' >>"$password_file"
	fi
	chmod 0600 "$admin_key" "$password_file"
}

render_test_kickstart() {
	record=${SODA_ACCEPTANCE_RELEASE_RECORD:-}
	[ -n "$record" ] || die "SODA_ACCEPTANCE_RELEASE_RECORD is required for the test model"
	need_file "$record"
	platform=$(jq -r '.platform // empty' "$record")
	[ "$platform" = linux/amd64 ] || die "release record platform $platform is not linux/amd64"
	image_reference=$(jq -r '.soda_image_reference // empty' "$record")
	case "$image_reference" in
		*@sha256:????????????????????????????????????????????????????????????????) ;;
		*) die "release record does not contain an exact Soda image reference" ;;
	esac
	expected_iso=$(jq -r '.iso_sha256 // empty' "$record")
	actual_iso=$(sha256sum "$iso" | awk '{print $1}')
	[ "$actual_iso" = "$expected_iso" ] || die "installer ISO does not match the release record"
	password_hash=$(openssl passwd -6 -stdin <"$password_file")
	public_key=$(awk 'NF >= 2 {print $1 " " $2; exit}' "$admin_key.pub")
	kickstart=$acceptance_dir/ks.cfg
	cat >"$kickstart" <<EOF
# Generated test-only Soda OS unattended installation.
cmdline
lang en_US.UTF-8
keyboard us
timezone UTC --utc
network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda-acceptance
zerombr
clearpart --all --initlabel
autopart --type=plain --fstype=ext4
rootpw --lock
user --name=$admin --gecos="$admin_name" --groups=wheel --password=$password_hash --iscrypted
firstboot --disable
eula --agreed
bootc --source-imgref="containers-storage:$image_reference" --target-imgref="$image_reference"
reboot

%addon org_fedoraproject_soda
username=$admin
name=$admin_name
email=$admin_email
%end

%post
install -d -m 0755 /etc/soda/authorized_keys
printf '%s\n' '$public_key' >/etc/soda/authorized_keys/$admin
chown root:root /etc/soda/authorized_keys/$admin
chmod 0644 /etc/soda/authorized_keys/$admin
restorecon -F /etc/soda/authorized_keys/$admin || true
%end
EOF
	chmod 0600 "$kickstart"
	seed=$acceptance_dir/oemdrv.iso
	xorriso -as mkisofs -quiet -V OEMDRV -o "$seed" "$kickstart"
	if command -v chcon >/dev/null 2>&1; then
		sudo -n chcon -t virt_content_t "$seed" "$iso"
	fi
	printf '%s\n' "$seed"
}

launch_domain() {
	model=$1
	domain=$2
	disk=$3
	seed=${4:-}
	set -- sudo -n virt-install --connect "${SODA_ACCEPTANCE_LIBVIRT_URI:-qemu:///system}" \
		--name "$domain" --metadata "description=Soda OS $model appliance" \
		--memory 4096 --vcpus 4 --cpu host-passthrough --machine q35 \
		--import --boot uefi,hd,cdrom,menu=on \
		--disk "path=$disk,format=qcow2,bus=virtio,cache=none" \
		--disk "path=$iso,device=cdrom,readonly=on,bus=sata" \
		--network network=default,model=virtio \
		--graphics vnc,listen=127.0.0.1 --video virtio --rng /dev/urandom \
		--osinfo detect=on,require=off --events on_reboot=restart,on_poweroff=destroy,on_crash=preserve \
		--noautoconsole
	if [ -n "$seed" ]; then
		set -- "$@" --disk "path=$seed,device=cdrom,readonly=on,bus=sata"
	fi
	if [ "$model" = staging ]; then
		set -- "$@" --autostart
	fi
	"$@"
	virsh_system dumpxml "$domain" >"$acceptance_dir/domain.xml"
	virsh_system dominfo "$domain" >"$acceptance_dir/domain-info.txt"
}

launch() {
	model=$1
	require_native_x86
	require_dir
	need sudo
	need virsh
	need virt-install
	need qemu-img
	iso=${SODA_ACCEPTANCE_ISO:-}
	[ -n "$iso" ] || die "SODA_ACCEPTANCE_ISO is required"
	need_file "$iso"
	iso=$(CDPATH= cd -- "$(dirname "$iso")" && pwd)/$(basename "$iso")
	if command -v chcon >/dev/null 2>&1; then
		sudo -n chcon -t virt_content_t "$iso"
	fi
	domain=$(model_domain "$model")
	domain_exists "$domain" && die "libvirt domain $domain already exists"
	disk=$(prepare_disk "$domain")
	seed=
	if [ "$model" = test ]; then
		need jq
		need openssl
		need ssh-keygen
		need xorriso
		prepare_test_identity
		seed=$(render_test_kickstart)
		record=${SODA_ACCEPTANCE_RELEASE_RECORD:-}
		record=$(CDPATH= cd -- "$(dirname "$record")" && pwd)/$(basename "$record")
		printf '%s\n' "$record" >"$acceptance_dir/release-record-path.txt"
		jq -r '.soda_image_reference' "$record" | sed 's/.*@//' >"$acceptance_dir/image-digest.txt"
	fi
	printf '%s\n' "$iso" >"$acceptance_dir/iso-path.txt"
	record_command "$model" "$domain" "$disk" "$seed"
	launch_domain "$model" "$domain" "$disk" "$seed"
	printf '%s\n' "$domain" >"$acceptance_dir/domain-name.txt"
	printf '%s\n' "$disk" >"$acceptance_dir/disk-path.txt"
	printf 'libvirt domain %s started; manage it through Cockpit\n' "$domain"
}

address() {
	domain=$(model_domain "$1")
	virsh_system domifaddr "$domain" --source lease | awk '$3 == "ipv4" {sub("/.*", "", $4); print $4; exit}'
}

wait_test() {
	model=$1
	[ "$model" = test ] || die "wait is only available for the unattended test model"
	require_dir
	need curl
	need ssh
	need ssh-keyscan
	domain=$(model_domain "$model")
	admin=${SODA_ACCEPTANCE_ADMIN:-soda-test}
	admin_key=$acceptance_dir/admin
	need_file "$admin_key"
	started=$(date +%s)
	deadline=$((started + 1200))
	guest_ip=
	while [ -z "$guest_ip" ]; do
		[ "$(date +%s)" -lt "$deadline" ] || die "libvirt did not report a test guest address within 1200 seconds"
		guest_ip=$(address "$model")
		[ -n "$guest_ip" ] || sleep 2
	done
	known_hosts=$acceptance_dir/known-hosts
	while :; do
		[ "$(date +%s)" -lt "$deadline" ] || die "installed test guest did not accept its SSH key within 1200 seconds"
		ssh-keyscan -T 5 -t ed25519 "$guest_ip" >"$known_hosts" 2>"$acceptance_dir/ssh-keyscan.stderr" || true
		if [ ! -s "$known_hosts" ]; then
			sleep 20
			continue
		fi
		if ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes \
			-o "UserKnownHostsFile=$known_hosts" -i "$admin_key" "$admin@$guest_ip" \
			'id; cat /proc/sys/kernel/random/boot_id' >"$acceptance_dir/ssh-readiness.txt" \
			2>"$acceptance_dir/ssh-readiness.stderr"; then
			break
		fi
		sleep 30
	done
	while ! curl --fail --silent --show-error --insecure "https://$guest_ip:9090/healthz" >/dev/null 2>&1; do
		[ "$(date +%s)" -lt "$deadline" ] || die "test guest Cockpit did not become ready within 1200 seconds"
		sleep 2
	done
	printf '%s\n' "$guest_ip" >"$acceptance_dir/guest-ip.txt"
	cat >"$acceptance_dir/runner.env" <<EOF
SODA_ACCEPTANCE_DIR=$acceptance_dir
SODA_ACCEPTANCE_ARCHITECTURE=x86_64
SODA_ACCEPTANCE_ADMIN=$admin
SODA_ACCEPTANCE_ADMIN_KEY=$admin_key
SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE=$acceptance_dir/admin-password
SODA_ACCEPTANCE_HOST=$guest_ip
SODA_ACCEPTANCE_SSH_PORT=22
SODA_ACCEPTANCE_COCKPIT_PORT=9090
SODA_ACCEPTANCE_LIBVIRT_DOMAIN=$domain
SODA_ACCEPTANCE_LIBVIRT_URI=${SODA_ACCEPTANCE_LIBVIRT_URI:-qemu:///system}
SODA_ACCEPTANCE_IMAGE_DIGEST=$(cat "$acceptance_dir/image-digest.txt")
SODA_ACCEPTANCE_RELEASE_RECORD=$(cat "$acceptance_dir/release-record-path.txt")
SODA_ACCEPTANCE_ISO=$(cat "$acceptance_dir/iso-path.txt")
EOF
	chmod 0600 "$acceptance_dir/runner.env"
	printf 'test guest %s is ready at %s\n' "$domain" "$guest_ip"
}

status() {
	model=$1
	require_dir
	domain=$(model_domain "$model")
	virsh_system dominfo "$domain" | tee "$acceptance_dir/domain-info.txt"
	virsh_system domifaddr "$domain" --source lease | tee "$acceptance_dir/domain-addresses.txt"
	virsh_system domblklist "$domain" --details | tee "$acceptance_dir/domain-blocks.txt"
	virsh_system dumpxml "$domain" >"$acceptance_dir/domain.xml"
}

control() {
	action=$1
	model=$2
	domain=$(model_domain "$model")
	virsh_system "$action" "$domain"
}

command=${1:-help}
model=${2:-}
case "$command" in
	help|-h|--help) usage ;;
	launch) [ -n "$model" ] || die "launch requires staging or test"; launch "$model" ;;
	wait) [ -n "$model" ] || die "wait requires test"; wait_test "$model" ;;
	address) [ -n "$model" ] || die "address requires staging or test"; address "$model" ;;
	status) [ -n "$model" ] || die "status requires staging or test"; status "$model" ;;
	start) [ -n "$model" ] || die "start requires staging or test"; control start "$model" ;;
	stop) [ -n "$model" ] || die "stop requires staging or test"; control shutdown "$model" ;;
	*) usage >&2; exit 2 ;;
esac
