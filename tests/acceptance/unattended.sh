#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Usage:
  tests/acceptance/unattended.sh run \
    --evidence-dir DIR \
    --candidate-iso PATH \
    --candidate-record PATH \
    --candidate-oci PATH \
    --fallback-record PATH \
    --fallback-oci PATH \
    --tailscale-auth-key-file PATH

Install candidate image B once through native raw QEMU, exercise the accepted
product scenarios, select earlier exact image A, and recover forward to B. The
runner owns its disposable VM, OEMDRV media, and loopback registry for the
whole run. It never publishes an artifact or release.

Optional environment:
  SODA_ACCEPTANCE_ADMIN=soda-test
  SODA_ACCEPTANCE_GUEST_HOST=TAILNET_IP_OR_NAME
  SODA_ACCEPTANCE_SSH_PORT=2222
  SODA_ACCEPTANCE_COCKPIT_PORT=19090
  SODA_ACCEPTANCE_REGISTRY_PORT=5001
  SODA_ACCEPTANCE_DISK_SIZE=40G
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
	[ -f "$1" ] && [ ! -L "$1" ] || die "required regular file $1 is unavailable"
}

select_docker() {
	if docker info >/dev/null 2>&1; then
		docker_access=direct
	elif sudo -n docker info >/dev/null 2>&1; then
		docker_access=sudo
	else
		die "Docker is unavailable directly or through passwordless sudo"
	fi
}

select_tailscale() {
	if command -v tailscale >/dev/null 2>&1; then
		tailscale_command=$(command -v tailscale)
	elif [ "$(uname -s)" = Darwin ] && [ -x /Applications/Tailscale.app/Contents/MacOS/Tailscale ]; then
		tailscale_command=/Applications/Tailscale.app/Contents/MacOS/Tailscale
	else
		die "Tailscale is unavailable on PATH and no macOS app CLI was found"
	fi
}

host_tailscale() {
	TAILSCALE_BE_CLI=1 "$tailscale_command" "$@"
}

host_docker() {
	case "$docker_access" in
		direct) docker "$@" ;;
		sudo) sudo -n docker "$@" ;;
		*) die "Docker access was not selected" ;;
	esac
}

absolute_file() {
	need_file "$1"
	printf '%s/%s\n' "$(CDPATH= cd -- "$(dirname "$1")" && pwd)" "$(basename "$1")"
}

protected_secret_file() {
	python3 - "$1" <<'PY'
import os
import stat
import sys

path = sys.argv[1]
value = os.lstat(path)
if not stat.S_ISREG(value.st_mode):
    raise SystemExit(f"{path} must be a regular file, not a symlink")
if value.st_mode & 0o077:
    raise SystemExit(f"{path} must not be accessible by group or other users")
print(os.path.realpath(path))
PY
}

native_architecture() {
	case "$(uname -m)" in
		aarch64|arm64) printf 'aarch64 linux/arm64\n' ;;
		x86_64|amd64) printf 'x86_64 linux/amd64\n' ;;
		*) die "acceptance requires matching-native AArch64 or x86-64 hardware" ;;
	esac
}

record_value() {
	jq -er "$2" "$1" || die "release record $1 is missing $2"
}

validate_artifact_set() {
	label=$1
	record=$2
	oci=$3
	expected_platform=$4
	need_file "$record"
	need_file "$oci"
	[ "$(record_value "$record" '.schema_version')" = 2 ] || die "$label record is not schema 2"
	[ "$(record_value "$record" '.platform')" = "$expected_platform" ] || die "$label record is for the wrong platform"
	reference=$(record_value "$record" '.soda_image_reference')
	case "$reference" in
		*@sha256:????????????????????????????????????????????????????????????????) ;;
		*) die "$label record has no exact image digest" ;;
	esac
}

wait_for_exit() {
	pid=$1
	timeout=$2
	deadline=$(( $(date +%s) + timeout ))
	while kill -0 "$pid" 2>/dev/null; do
		[ "$(date +%s)" -lt "$deadline" ] || die "raw QEMU did not exit within $timeout seconds"
		sleep 1
	done
	wait "$pid" || die "raw QEMU exited unsuccessfully"
}

terminate_process() {
	pid=$1
	kill -TERM "$pid" 2>/dev/null || true
	deadline=$(( $(date +%s) + 10 ))
	while kill -0 "$pid" 2>/dev/null; do
		if [ "$(date +%s)" -ge "$deadline" ]; then
			kill -KILL "$pid" 2>/dev/null || true
			break
		fi
		sleep 1
	done
	wait "$pid" 2>/dev/null || true
}

discover_tailnet_address() {
	before=$1
	deadline=$(( $(date +%s) + 1200 ))
	current=$evidence_dir/.host-tailnet-current.json
	candidates=$evidence_dir/.new-soda-peers.tsv
	while :; do
		host_tailscale status --json >"$current"
		: >"$candidates"
		for id in $(jq -r '.Peer[]? | select(.Online == true and .HostName == "soda") | .ID' "$current"); do
			if ! jq -e --arg id "$id" '.Peer[]? | select(.ID == $id)' "$before" >/dev/null; then
				jq -r --arg id "$id" '.Peer[]? | select(.ID == $id) | [.ID, .TailscaleIPs[0]] | @tsv' "$current" >>"$candidates"
			fi
		done
		case "$(wc -l <"$candidates" | tr -d ' ')" in
			0) ;;
			1)
				cp "$current" "$evidence_dir/host-tailnet-enrolled.json"
				cut -f2 "$candidates"
				rm -f "$current" "$candidates"
				return
				;;
			*) die "multiple newly enrolled Soda peers make the Tailnet endpoint ambiguous" ;;
		esac
		[ "$(date +%s)" -lt "$deadline" ] || die "newly enrolled Soda peer did not become visible within 1200 seconds"
		sleep 5
	done
}

run() {
	evidence_dir=
	candidate_iso=
	candidate_record=
	candidate_oci=
	fallback_record=
	fallback_oci=
	tailscale_key=
	while [ "$#" -gt 0 ]; do
		case "$1" in
			--evidence-dir|--candidate-iso|--candidate-record|--candidate-oci|--fallback-record|--fallback-oci|--tailscale-auth-key-file)
				[ "$#" -ge 2 ] || die "$1 requires a value"
				case "$1" in
					--evidence-dir) evidence_dir=$2 ;;
					--candidate-iso) candidate_iso=$2 ;;
					--candidate-record) candidate_record=$2 ;;
					--candidate-oci) candidate_oci=$2 ;;
					--fallback-record) fallback_record=$2 ;;
					--fallback-oci) fallback_oci=$2 ;;
					--tailscale-auth-key-file) tailscale_key=$2 ;;
				esac
				shift 2
				;;
			-h|--help) usage; return ;;
			*) die "unknown argument $1" ;;
		esac
	done
	[ -n "$evidence_dir" ] || die "--evidence-dir is required"
	[ -n "$candidate_iso" ] || die "--candidate-iso is required"
	[ -n "$candidate_record" ] || die "--candidate-record is required"
	[ -n "$candidate_oci" ] || die "--candidate-oci is required"
	[ -n "$fallback_record" ] || die "--fallback-record is required"
	[ -n "$fallback_oci" ] || die "--fallback-oci is required"
	[ -n "$tailscale_key" ] || die "--tailscale-auth-key-file is required"

	for command in curl docker go jq openssl qemu-img sha256sum ssh ssh-keygen sudo tar xorriso; do need "$command"; done
	select_docker
	select_tailscale
	set -- $(native_architecture)
	architecture=$1
	expected_platform=$2
	repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
	helper=$repo_root/tests/acceptance/internal/bootc.sh
	registry_pin_file=$repo_root/tests/acceptance/registry-image.txt
	skopeo_pin_file=$repo_root/tests/acceptance/skopeo-image.txt
	need_file "$helper"
	need_file "$registry_pin_file"
	need_file "$skopeo_pin_file"
	candidate_iso=$(absolute_file "$candidate_iso")
	candidate_record=$(absolute_file "$candidate_record")
	candidate_oci=$(absolute_file "$candidate_oci")
	fallback_record=$(absolute_file "$fallback_record")
	fallback_oci=$(absolute_file "$fallback_oci")
	tailscale_key=$(protected_secret_file "$tailscale_key")
	validate_artifact_set candidate "$candidate_record" "$candidate_oci" "$expected_platform"
	validate_artifact_set fallback "$fallback_record" "$fallback_oci" "$expected_platform"
	expected_iso=$(record_value "$candidate_record" '.iso_sha256')
	actual_iso=$(sha256sum "$candidate_iso" | awk '{print $1}')
	[ "$expected_iso" = "$actual_iso" ] || die "candidate ISO does not match its release record"

	umask 077
	[ ! -e "$evidence_dir" ] || die "evidence directory already exists: $evidence_dir"
	mkdir -p "$evidence_dir"
	chmod 0700 "$evidence_dir"
	evidence_dir=$(CDPATH= cd -- "$evidence_dir" && pwd)
	admin=${SODA_ACCEPTANCE_ADMIN:-soda-test}
	work_dir=$(mktemp -d "${TMPDIR:-/tmp}/soda-acceptance-run.XXXXXX")
	work_dir=$(CDPATH= cd -- "$work_dir" && pwd)
	chmod 0700 "$work_dir"
	admin_key=$work_dir/admin
	password_file=$work_dir/admin-password
	oemdrv=$work_dir/oemdrv.iso
	disk=$work_dir/soda-system.qcow2
	registry_port=${SODA_ACCEPTANCE_REGISTRY_PORT:-5001}
	printf '%s\n' "$registry_port" | LC_ALL=C grep -Eq '^[0-9]{1,5}$' || die "registry port must be numeric"
	[ "$registry_port" -ge 1 ] && [ "$registry_port" -le 65535 ] || die "registry port is outside the TCP range"
	registry_name=soda-acceptance-registry-$$
	registry_image=$(sed -n '1p' "$registry_pin_file")
	skopeo_image=$(sed -n '1p' "$skopeo_pin_file")
	case "$(uname -s)" in
		Linux) skopeo_container_network='--network host'; skopeo_registry_host=127.0.0.1 ;;
		Darwin) skopeo_container_network=; skopeo_registry_host=host.docker.internal ;;
		*) die "unsupported host operating system for containerized Skopeo" ;;
	esac
	qemu_pid=
	registry_started=false
	completed=false
	inputs_retired=false

	sanitize_evidence() {
		[ -e "$evidence_dir/secret-absence.txt" ] && return
		python3 - "$evidence_dir" "$tailscale_key" "$password_file" "$admin_key" <<'PY'
import pathlib
import sys

evidence = pathlib.Path(sys.argv[1])
labels = ("tailscale-auth-key", "administrator-password", "administrator-private-key")
secrets = []
for label, raw_path in zip(labels, sys.argv[2:]):
    path = pathlib.Path(raw_path)
    if not path.is_file():
        continue
    value = path.read_bytes()
    for candidate in (value, value.rstrip(b"\r\n")):
        if candidate and all(candidate != existing[1] for existing in secrets):
            secrets.append((label, candidate))

redacted = []
for path in evidence.rglob("*"):
    if not path.is_file() or path.is_symlink() or path.name == "secret-absence.txt":
        continue
    contents = path.read_bytes()
    original = contents
    matched = []
    for label, secret in secrets:
        if secret in contents:
            contents = contents.replace(secret, b"[REDACTED]")
            matched.append(label)
    if contents != original:
        path.write_bytes(contents)
        redacted.append((str(path.relative_to(evidence)), sorted(set(matched))))

report = evidence / "secret-absence.txt"
with report.open("w", encoding="utf-8") as output:
    if redacted:
        output.write("result=fail-redacted\n")
        for relative, matched in redacted:
            output.write(f"redacted={relative}:{','.join(matched)}\n")
    else:
        output.write("result=pass\n")
        for label in labels:
            output.write(f"{label}=absent\n")
if redacted:
    raise SystemExit("credential material reached evidence and was redacted")
PY
	}

	retire_run_inputs() {
		[ "$inputs_retired" = true ] && return
		sanitize_evidence
		rm -rf -- "$work_dir"
		inputs_retired=true
	}

	cleanup() {
		status=$?
		trap - 0 1 2 15
		set +e
		if [ -n "$qemu_pid" ]; then
			terminate_process "$qemu_pid"
		fi
		if [ "$registry_started" = true ]; then
			host_docker rm -f "$registry_name" >/dev/null 2>&1 || true
		fi
		if [ "$inputs_retired" != true ]; then
			sanitize_evidence >/dev/null 2>&1 || true
			rm -rf -- "$work_dir"
		fi
		if [ "$completed" != true ]; then
			printf 'acceptance stopped; evidence retained in %s\n' "$evidence_dir" >&2
		fi
		exit "$status"
	}
	trap cleanup 0 1 2 15

	ssh-keygen -q -t ed25519 -N '' -C "$admin@raw-qemu" -f "$admin_key"
	openssl rand -base64 24 >"$password_file"
	chmod 0600 "$admin_key" "$password_file"
	(
		cd "$repo_root"
		go run ./cmd/soda-image --architecture "$architecture" installer-input \
			--unattended \
			--iso "$candidate_iso" \
			--release-record "$candidate_record" \
			--username "$admin" \
			--password-file "$password_file" \
			--ssh-public-key-file "$admin_key.pub" \
			--tailscale-auth-key-file "$tailscale_key" \
			--output "$oemdrv"
	)
	protected_secret_file "$oemdrv" >/dev/null

	host_docker run --detach --name "$registry_name" --publish "127.0.0.1:$registry_port:5000" "$registry_image" \
		>"$evidence_dir/registry-container-id.txt"
	registry_started=true
	registry_deadline=$(( $(date +%s) + 30 ))
	until curl --fail --silent --show-error "http://127.0.0.1:$registry_port/v2/" >/dev/null 2>&1; do
		[ "$(date +%s)" -lt "$registry_deadline" ] || die "disposable registry did not become ready"
		sleep 1
	done
	registry_repository=127.0.0.1:$registry_port/soda-os
	copy_oci() {
		archive=$1
		tag=$2
		if command -v skopeo >/dev/null 2>&1; then
			skopeo copy --preserve-digests --dest-tls-verify=false "oci-archive:$archive" "docker://$registry_repository:$tag"
		else
			# Word splitting is intentional for the fixed host-OS network selection.
			# shellcheck disable=SC2086
			host_docker run --rm $skopeo_container_network \
				--volume "$archive:/input/archive.tar:ro" --entrypoint /usr/bin/skopeo "$skopeo_image" \
				copy --preserve-digests --dest-tls-verify=false oci-archive:/input/archive.tar \
				"docker://$skopeo_registry_host:$registry_port/soda-os:$tag"
		fi
	}
	inspect_digest() {
		tag=$1
		if command -v skopeo >/dev/null 2>&1; then
			skopeo inspect --tls-verify=false --format '{{.Digest}}' "docker://$registry_repository:$tag"
		else
			# Word splitting is intentional for the fixed host-OS network selection.
			# shellcheck disable=SC2086
			host_docker run --rm $skopeo_container_network \
				--entrypoint /usr/bin/skopeo "$skopeo_image" inspect --tls-verify=false --format '{{.Digest}}' \
				"docker://$skopeo_registry_host:$registry_port/soda-os:$tag"
		fi
	}
	copy_oci "$fallback_oci" fallback >"$evidence_dir/registry-fallback-copy.txt"
	copy_oci "$candidate_oci" candidate >"$evidence_dir/registry-candidate-copy.txt"
	a_digest=$(record_value "$fallback_record" '.soda_image_reference' | sed 's/.*@//')
	b_digest=$(record_value "$candidate_record" '.soda_image_reference' | sed 's/.*@//')
	for pair in "fallback:$a_digest" "candidate:$b_digest"; do
		tag=${pair%%:*}
		digest=${pair#*:}
		actual=$(inspect_digest "$tag")
		[ "$actual" = "$digest" ] || die "registry changed the $tag manifest digest"
	done

	export SODA_ACCEPTANCE_DIR=$evidence_dir
	export SODA_ACCEPTANCE_ARCHITECTURE=$architecture
	export SODA_ACCEPTANCE_ADMIN=$admin
	export SODA_ACCEPTANCE_ADMIN_KEY=$admin_key
	export SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE=$password_file
	export SODA_ACCEPTANCE_HOST=127.0.0.1
	export SODA_ACCEPTANCE_SSH_PORT=${SODA_ACCEPTANCE_SSH_PORT:-2222}
	export SODA_ACCEPTANCE_COCKPIT_PORT=${SODA_ACCEPTANCE_COCKPIT_PORT:-19090}
	requested_guest_host=${SODA_ACCEPTANCE_GUEST_HOST:-}
	export SODA_ACCEPTANCE_GUEST_SSH_PORT=22
	export SODA_ACCEPTANCE_GUEST_COCKPIT_PORT=9090
	export SODA_ACCEPTANCE_IMAGE_DIGEST=$b_digest
	export SODA_ACCEPTANCE_IMAGE_A_REFERENCE=10.0.2.2:$registry_port/soda-os@${a_digest}
	export SODA_ACCEPTANCE_IMAGE_B_REFERENCE=10.0.2.2:$registry_port/soda-os@${b_digest}
	export SODA_ACCEPTANCE_RELEASE_RECORD=$candidate_record
	export SODA_ACCEPTANCE_ISO=$candidate_iso
	export SODA_ACCEPTANCE_KICKSTART_ISO=$oemdrv
	export SODA_ACCEPTANCE_DISK=$disk
	export SODA_ACCEPTANCE_DISK_SIZE=${SODA_ACCEPTANCE_DISK_SIZE:-40G}
	host_tailscale status --json >"$evidence_dir/host-tailnet-before.json"

	printf 'installing candidate image B through raw QEMU\n'
	sh "$helper" launch install >"$evidence_dir/qemu.stdout" 2>"$evidence_dir/qemu.stderr" &
	qemu_pid=$!
	if [ -n "$requested_guest_host" ]; then
		tailnet_address=$requested_guest_host
	else
		tailnet_address=$(discover_tailnet_address "$evidence_dir/host-tailnet-before.json")
	fi
	case "$tailnet_address" in
		''|*[!A-Za-z0-9:._-]*) die "invalid Tailnet endpoint: $tailnet_address" ;;
		*) ;;
	esac
	printf '%s\n' "$tailnet_address" >"$evidence_dir/tailnet-address.txt"
	export SODA_ACCEPTANCE_GUEST_HOST=$tailnet_address
	sh "$helper" wait
	sh "$helper" fallback registry-enable
	sh "$helper" fallback seed-b
	sh "$helper" fallback capture b-current

	reboot_to() {
		target=$1
		sh "$helper" fallback stage "$target"
		sh "$helper" fallback unlock
		sh "$helper" stop
		wait_for_exit "$qemu_pid" 120
		qemu_pid=
		sh "$helper" launch installed >>"$evidence_dir/qemu.stdout" 2>>"$evidence_dir/qemu.stderr" &
		qemu_pid=$!
		sh "$helper" wait
	}

	printf 'selecting exact earlier image A\n'
	reboot_to a
	sh "$helper" fallback capture a-selected
	sh "$helper" fallback compare b-current a-selected
	printf 'recovering forward to exact candidate image B\n'
	reboot_to b
	sh "$helper" fallback capture b-restored
	sh "$helper" fallback compare b-current b-restored
	sh "$helper" fallback registry-disable

	workspace_target=$(sh "$helper" project-workspace kept)
	export SODA_ACCEPTANCE_WORKSPACE_TARGET=$workspace_target
	export SODA_ACCEPTANCE_WORKSPACE_KEY=$admin_key
	export SODA_ACCEPTANCE_REQUIRE_WORKSPACE_TOOLSET=1
	sh "$helper" scenario product
	sh "$helper" capture final-pre-capstone
	sh "$helper" stop
	wait_for_exit "$qemu_pid" 120
	qemu_pid=
	retire_run_inputs

	jq -n \
		--arg architecture "$architecture" \
		--arg candidate_source_revision "$(record_value "$candidate_record" '.source_revision')" \
		--arg fallback_source_revision "$(record_value "$fallback_record" '.source_revision')" \
		--arg candidate_oci_sha256 "$(sha256sum "$candidate_oci" | awk '{print $1}')" \
		--arg fallback_oci_sha256 "$(sha256sum "$fallback_oci" | awk '{print $1}')" \
		--arg candidate_record_sha256 "$(sha256sum "$candidate_record" | awk '{print $1}')" \
		--arg candidate_iso_sha256 "$actual_iso" \
		--arg fallback_record_sha256 "$(sha256sum "$fallback_record" | awk '{print $1}')" \
		--arg image_a_digest "$a_digest" \
		--arg image_b_digest "$b_digest" \
		--arg workspace_username "$workspace_target" \
		'{result:"pass",architecture:$architecture,candidate_source_revision:$candidate_source_revision,
		  fallback_source_revision:$fallback_source_revision,candidate_oci_sha256:$candidate_oci_sha256,
		  fallback_oci_sha256:$fallback_oci_sha256,candidate_record_sha256:$candidate_record_sha256,
		  candidate_iso_sha256:$candidate_iso_sha256,fallback_record_sha256:$fallback_record_sha256,
		  image_a_digest:$image_a_digest,image_b_digest:$image_b_digest,
		  workspace_username:$workspace_username}' >"$evidence_dir/summary.json"
	completed=true
	printf 'single-run raw-QEMU acceptance passed; evidence: %s\n' "$evidence_dir"
}

case "${1:-help}" in
	help|-h|--help) usage ;;
	run) shift; run "$@" ;;
	*) usage >&2; exit 2 ;;
esac
