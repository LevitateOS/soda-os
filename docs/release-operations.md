# Soda OS release and operator runbook

This is the public-release path for Fedora 44 bootc on the equal AArch64 and
x86-64 sibling architectures. It uses `ghcr.io/levitateos/soda-os` for signed
immutable image digests and one paired GitHub Release in `LevitateOS/soda-os`
for both installer ISOs, signed records, and the signed release index. The
designated human release owner supplies the Soda Cosign public key, a
passphrase-protected private key, and `SODA_GITHUB_TOKEN`; only the public key
and GitHub discovery locations are copied into runtime images.

## Build and initial install release

Run these commands from a clean Soda checkout with the pinned tools available.
The paths below are operator inputs, not repository files.

```sh
just check
just release-tools
ARCH=x86_64 # repeat all steps for aarch64
just oci "$ARCH" /path/to/cosign.pub

go run ./cmd/soda-image --architecture "$ARCH" publish \
  --archive ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --prepare-only
```

Cosign prompts for the private-key passphrase during publication. Save the
printed exact image reference, then build the ISO from it:

```sh
go run ./cmd/soda-image --architecture "$ARCH" iso \
  --image ghcr.io/levitateos/soda-os@sha256:EXACT_DIGEST \
  --archive ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  --public-key /path/to/cosign.pub

go run ./cmd/soda-image --architecture "$ARCH" publish \
  --archive ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --iso ".artifacts/images/SodaOS-0.3.1-${ARCH}.iso"
```

The final command re-checks the exact ISO payload and writes an
architecture-named signed release record under `.artifacts/releases/`. After
both AArch64 and x86-64 artifacts exist, the designated release owner creates
the paired public release:

```sh
export SODA_GITHUB_TOKEN=... # release-owner token; do not pass it on the command line
go run ./cmd/soda-release \
  --public-key /path/to/cosign.pub --signing-key /path/to/cosign.key \
  --aarch64-iso .artifacts/images/SodaOS-0.3.1-aarch64.iso \
  --aarch64-record .artifacts/releases/soda-os-0.3.1-aarch64.release.json \
  --aarch64-bundle .artifacts/releases/soda-os-0.3.1-aarch64.release.json.sigstore.json \
  --x86_64-iso .artifacts/images/SodaOS-0.3.1-x86_64.iso \
  --x86_64-record .artifacts/releases/soda-os-0.3.1-x86_64.release.json \
  --x86_64-bundle .artifacts/releases/soda-os-0.3.1-x86_64.release.json.sigstore.json
```

`soda-release` creates a draft, uploads the two ISOs, two records, two record
bundles, and signed index, verifies the downloaded bytes, then publishes it.
The ISO has a matching `.sha256` file, and the publisher independently
re-inspects its embedded exact payload before creating the signed record.

Boot the matching ISO on AArch64 or x86-64 UEFI and complete stock interactive
Anaconda. Use the installer for a new Soda host only; there is no Rocky in-place
conversion. The default hostname is `soda`, but the installer permits a
different hostname.

## Administrator-controlled updates

On an installed Soda host, inspect and prepare an update without interrupting
Soda work:

```sh
sudo sodactl os update status
sudo sodactl os update check
sudo sodactl os update stage
```

`check` downloads the public signed release index once, verifies it with Soda's
embedded Cosign public key, and selects the installed architecture's exact
digest. It accepts only a paired index containing both sibling architectures,
with signed exact image references matching state schema 3. `stage` downloads
that digest using bootc's container signature policy but leaves the booted
deployment and running Soda services untouched.

During a planned maintenance window, activate the already staged deployment:

```sh
sudo sodactl os update activate --confirm-reboot
```

This is the only Soda update action that requests a reboot. No timer or Soda
process checks, downloads, stages, activates, or reboots automatically. After
the reboot, use `sudo sodactl os update status` and `bootc status` to confirm
the exact booted digest.

Schema 2 is the persistent-state compatibility boundary. Users and PAM
passwords, device and host SSH keys, projects, bare repositories, worktrees,
toolchains, SQLite state, certificates, and logs persist across the image
transition. This MVP does not support database-schema-changing releases or
state rollback across incompatible schemas.

## Local sibling-architecture acceptance

`tests/acceptance/bootc.sh` separates repeatable evidence collection from
operator-owned authentication. Set an untracked evidence directory, the
selected architecture, Anaconda administrator's SSH identity, and exact digest
under test before using `launch`, `wait`, `capture`, `workload`, or `stop`. Run
`--help` for the complete input contract.

The `launch install` operation creates a blank sparse disk and starts the
platform-matched stock Anaconda ISO. AArch64 uses its UEFI/HVF harness; x86-64
uses OVMF/KVM and a serial text console. Both forward SSH and Cockpit only to
the configured host address and expose a QMP lifecycle socket. Complete
Anaconda, then enroll the administrator's public key and record the disposable
host key once before using normal SSH sessions for privileged evidence:

```sh
ssh-copy-id \
  -o StrictHostKeyChecking=accept-new \
  -o "UserKnownHostsFile=$SODA_ACCEPTANCE_DIR/known-hosts" \
  -i "$SODA_ACCEPTANCE_ADMIN_KEY.pub" \
  -p 2222 vince@127.0.0.1
```

Do not use the forced project account as the administrator account.

Before each `capture NAME`, the operator records the corresponding privileged
status without exposing credentials:

```sh
mkdir -p "$HOME/.local/state/soda-acceptance"
sudo sodactl os update status > \
  "$HOME/.local/state/soda-acceptance/NAME-privileged.json"
```

The capture operation copies that status alongside the boot ID, QMP status,
service and mount state, Cockpit health, registry manifests, classic Cosign
signature attachment, release hashes, stdout, stderr, and timestamps. The
workload operation uses a registered Soda device key and the project's forced
SSH account to prove that staging does not interrupt a live workspace.

Use the platform's configured Anaconda console and use QMP only for observation
and clean shutdown. Equal support requires equivalent evidence from both
sibling paths: exact booted and staged deployment identity, UEFI boot entries,
Soda/SSH/Cockpit behavior, persistent state, non-disruptive staging, and manual
reboot activation. A successful run for one architecture does not substitute
for the other's acceptance gate.
