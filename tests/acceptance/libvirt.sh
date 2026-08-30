#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Usage: tests/acceptance/libvirt.sh COMMAND staging

Commands:
  launch staging   Define the persistent, owner-started Cockpit staging VM
  address staging  Print the current libvirt DHCP address
  status staging   Record and print libvirt domain status

Required environment for launch:
  SODA_ACCEPTANCE_DIR   Untracked staging evidence directory
  SODA_ACCEPTANCE_ISO   Exact Soda installer ISO

Optional environment:
  SODA_ACCEPTANCE_ARCHITECTURE=x86_64
  SODA_ACCEPTANCE_DOMAIN_STAGING=soda-staging
  SODA_ACCEPTANCE_DISK_SIZE=40G
  SODA_ACCEPTANCE_LIBVIRT_URI=qemu:///system
  SODA_ACCEPTANCE_STORAGE_DIR=/home/libvirt/images

This helper only defines the owner-controlled staging domain. It leaves the
domain shut off with autostart disabled. Automated test VMs use raw QEMU through
bootc.sh and never appear in Cockpit's libvirt inventory.
EOF
}

die() {
	echo "soda-libvirt-staging: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command $1 is unavailable"
}

need_file() {
	[ -f "$1" ] || die "required file $1 is unavailable"
}

require_staging() {
	[ "${1:-}" = staging ] || die "the libvirt helper only manages staging"
}

require_native_x86() {
	architecture=${SODA_ACCEPTANCE_ARCHITECTURE:-x86_64}
	[ "$architecture" = x86_64 ] || die "the libvirt staging model requires x86_64"
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

domain_name() {
	printf '%s\n' "${SODA_ACCEPTANCE_DOMAIN_STAGING:-soda-staging}"
}

domain_exists() {
	virsh_system dominfo "$1" >/dev/null 2>&1
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

launch() {
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
	domain=$(domain_name)
	domain_exists "$domain" && die "libvirt domain $domain already exists"
	disk=$(prepare_disk "$domain")

	set -- sudo -n virt-install --connect "${SODA_ACCEPTANCE_LIBVIRT_URI:-qemu:///system}" \
		--name "$domain" --metadata "description=Soda OS owner-controlled staging appliance" \
		--memory 4096 --vcpus 4 --cpu host-passthrough --machine q35 \
		--import --boot uefi,hd,cdrom,menu=on \
		--disk "path=$disk,format=qcow2,bus=virtio,cache=none" \
		--disk "path=$iso,device=cdrom,readonly=on,bus=sata" \
		--network network=default,model=virtio \
		--graphics vnc,listen=127.0.0.1 --video virtio --rng /dev/urandom \
		--osinfo detect=on,require=off \
		--events on_reboot=restart,on_poweroff=destroy,on_crash=preserve \
		--noautoconsole
	"$@" --print-xml >"$acceptance_dir/domain.xml"
	virsh_system define "$acceptance_dir/domain.xml" >/dev/null
	virsh_system autostart "$domain" --disable >/dev/null 2>&1 || true
	virsh_system dumpxml "$domain" >"$acceptance_dir/domain.xml"
	virsh_system dominfo "$domain" >"$acceptance_dir/domain-info.txt"
	printf '%s\n' "$iso" >"$acceptance_dir/iso-path.txt"
	printf '%s\n' "$disk" >"$acceptance_dir/disk-path.txt"
	printf '%s\n' "$domain" >"$acceptance_dir/domain-name.txt"
	printf 'libvirt domain %s defined and left shut off; start it through Cockpit\n' "$domain"
}

address() {
	domain=$(domain_name)
	virsh_system domifaddr "$domain" --source lease |
		awk '$3 == "ipv4" {sub("/.*", "", $4); print $4; exit}'
}

status() {
	require_dir
	domain=$(domain_name)
	virsh_system dominfo "$domain" | tee "$acceptance_dir/domain-info.txt"
	virsh_system domifaddr "$domain" --source lease | tee "$acceptance_dir/domain-addresses.txt"
	virsh_system domblklist "$domain" --details | tee "$acceptance_dir/domain-blocks.txt"
	virsh_system dumpxml "$domain" >"$acceptance_dir/domain.xml"
}

command=${1:-help}
model=${2:-}
case "$command" in
	help|-h|--help) usage ;;
	launch) require_staging "$model"; launch ;;
	address) require_staging "$model"; address ;;
	status) require_staging "$model"; status ;;
	*) usage >&2; exit 2 ;;
esac
