#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Usage: tests/acceptance/unattended.sh prepare

Prepare a raw-QEMU Soda acceptance installation with test-only credentials.
After it succeeds:

  . "$SODA_ACCEPTANCE_DIR/runner.env"
  tests/acceptance/bootc.sh launch install

Required environment:
  SODA_ACCEPTANCE_DIR              Untracked evidence directory
  SODA_ACCEPTANCE_ISO              Exact Soda installer ISO
  SODA_ACCEPTANCE_RELEASE_RECORD   Release record for that ISO
  SODA_ACCEPTANCE_TAILSCALE_AUTH_KEY_FILE
                                   Protected file containing one disposable key

Optional environment:
  SODA_ACCEPTANCE_ADMIN=soda-test
  SODA_ACCEPTANCE_SSH_PORT=2222
  SODA_ACCEPTANCE_COCKPIT_PORT=19090
EOF
}

die() {
	echo "soda-unattended-acceptance: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command $1 is unavailable"
}

need_file() {
	[ -f "$1" ] || die "required file $1 is unavailable"
}

prepare() {
	case "$(uname -m)" in
		x86_64|amd64) ;;
		*) die "x86-64 unattended acceptance requires matching native hardware" ;;
	esac
	for command in jq openssl ssh-keygen sha256sum xorriso; do
		need "$command"
	done
	acceptance_dir=${SODA_ACCEPTANCE_DIR:-}
	iso=${SODA_ACCEPTANCE_ISO:-}
	record=${SODA_ACCEPTANCE_RELEASE_RECORD:-}
	tailscale_auth_key_file=${SODA_ACCEPTANCE_TAILSCALE_AUTH_KEY_FILE:-}
	[ -n "$acceptance_dir" ] || die "SODA_ACCEPTANCE_DIR is required"
	[ -n "$iso" ] || die "SODA_ACCEPTANCE_ISO is required"
	[ -n "$record" ] || die "SODA_ACCEPTANCE_RELEASE_RECORD is required"
	[ -n "$tailscale_auth_key_file" ] || die "SODA_ACCEPTANCE_TAILSCALE_AUTH_KEY_FILE is required"
	umask 077
	mkdir -p "$acceptance_dir"
	chmod 0700 "$acceptance_dir"
	acceptance_dir=$(CDPATH= cd -- "$acceptance_dir" && pwd)
	need python3
	need_file "$iso"
	need_file "$record"
	need_file "$tailscale_auth_key_file"
	tailscale_auth_key_file=$(python3 - "$tailscale_auth_key_file" "$acceptance_dir" <<'PY'
import os
import stat
import sys

source_name, evidence_name = sys.argv[1:]
source_stat = os.lstat(source_name)
if not stat.S_ISREG(source_stat.st_mode):
    raise SystemExit("Tailscale auth key input must be a regular file, not a symlink")
if source_stat.st_mode & 0o077:
    raise SystemExit("Tailscale auth key input must not be accessible by group or other users")
source = os.path.realpath(source_name)
evidence = os.path.realpath(evidence_name)
if os.path.commonpath((source, evidence)) == evidence:
    raise SystemExit("Tailscale auth key input must remain outside acceptance evidence")
print(source)
PY
	)
	iso=$(CDPATH= cd -- "$(dirname "$iso")" && pwd)/$(basename "$iso")
	record=$(CDPATH= cd -- "$(dirname "$record")" && pwd)/$(basename "$record")

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

	admin=${SODA_ACCEPTANCE_ADMIN:-soda-test}
	admin_key=$acceptance_dir/admin
	password_file=$acceptance_dir/admin-password
	if [ ! -e "$admin_key" ] && [ ! -e "$admin_key.pub" ]; then
		ssh-keygen -q -t ed25519 -N '' -C "$admin@raw-qemu" -f "$admin_key"
	fi
	need_file "$admin_key"
	need_file "$admin_key.pub"
	if [ ! -e "$password_file" ]; then
		openssl rand -base64 24 | tr -d '\n' >"$password_file"
		printf '\n' >>"$password_file"
	fi
	chmod 0600 "$admin_key" "$password_file"
	password=$(tr -d '\r\n' <"$password_file")
	[ -n "$password" ] || die "administrator password file is empty"
	public_key=$(awk 'NF >= 2 {print $1 " " $2; exit}' "$admin_key.pub")
	tailscale_auth_key=$(tr -d '\r\n' <"$tailscale_auth_key_file")
	[ -n "$tailscale_auth_key" ] || die "Tailscale auth key file is empty"

	kickstart=$acceptance_dir/ks.cfg
	kickstart_iso=$acceptance_dir/oemdrv.iso
	prepared=false
	cleanup_prepare() {
		rm -f "$kickstart"
		[ "$prepared" = true ] || rm -f "$kickstart_iso"
	}
	abort_prepare() {
		trap - 1 2 15
		exit 1
	}
	trap cleanup_prepare 0
	trap abort_prepare 1 2 15
	cat >"$kickstart" <<EOF
# Generated test-only Soda OS unattended installation under raw QEMU.
cmdline
lang en_US.UTF-8
keyboard us
timezone UTC --utc
network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda-acceptance
zerombr
clearpart --all --initlabel
autopart --type=plain --fstype=ext4
rootpw --lock
user --name=$admin --groups=wheel --password="$password" --plaintext
sshkey --username=$admin "$public_key"
firstboot --disable
eula --agreed
bootc --source-imgref="containers-storage:$image_reference" --target-imgref="$image_reference"
reboot

%addon org_fedoraproject_soda
tailscale_auth_key=$tailscale_auth_key
%end
EOF
	chmod 0600 "$kickstart"
	xorriso -as mkisofs -quiet -V OEMDRV -o "$kickstart_iso" "$kickstart"
	rm -f "$kickstart"

	image_digest=$(printf '%s\n' "$image_reference" | sed 's/.*@//')
	cat >"$acceptance_dir/runner.env" <<EOF
export SODA_ACCEPTANCE_DIR=$acceptance_dir
export SODA_ACCEPTANCE_ARCHITECTURE=x86_64
export SODA_ACCEPTANCE_ADMIN=$admin
export SODA_ACCEPTANCE_ADMIN_KEY=$admin_key
export SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE=$password_file
export SODA_ACCEPTANCE_HOST=127.0.0.1
export SODA_ACCEPTANCE_SSH_PORT=${SODA_ACCEPTANCE_SSH_PORT:-2222}
export SODA_ACCEPTANCE_COCKPIT_PORT=${SODA_ACCEPTANCE_COCKPIT_PORT:-19090}
export SODA_ACCEPTANCE_GUEST_HOST=${SODA_ACCEPTANCE_GUEST_HOST:-soda-acceptance}
export SODA_ACCEPTANCE_GUEST_SSH_PORT=22
export SODA_ACCEPTANCE_GUEST_COCKPIT_PORT=9090
export SODA_ACCEPTANCE_IMAGE_DIGEST=$image_digest
export SODA_ACCEPTANCE_RELEASE_RECORD=$record
export SODA_ACCEPTANCE_ISO=$iso
export SODA_ACCEPTANCE_KICKSTART_ISO=$kickstart_iso
EOF
	chmod 0600 "$acceptance_dir/runner.env"
	prepared=true
	trap - 0 1 2 15
	printf 'raw-QEMU acceptance inputs prepared in %s\n' "$acceptance_dir"
}

case "${1:-help}" in
	help|-h|--help) usage ;;
	prepare) prepare ;;
	*) usage >&2; exit 2 ;;
esac
