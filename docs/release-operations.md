# Soda OS release and operator runbook

This is the trusted-LAN MVP release path for Fedora 44 bootc on the equal
AArch64 and x86-64 sibling architectures. It uses one HTTPS repository,
`registry.soda.local/soda/os`, with anonymous pulls and publisher-only pushes,
but keeps separate `current-aarch64` and `current-x86_64` discovery tags and
architecture-named release records. It does not publish a multi-platform index.
The administrator supplies the registry CA, the Soda Cosign public key, and a
passphrase-protected private key; only the CA and public key are copied into
the images.

## Build and initial install release

Run these commands from a clean Soda checkout with the pinned tools available.
The paths below are operator inputs, not repository files.

```sh
just check
just release-tools
ARCH=x86_64 # or aarch64
just oci "$ARCH" /path/to/registry-ca.crt /path/to/cosign.pub

go run ./cmd/soda-image --architecture "$ARCH" publish \
  --archive ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --defer-current
```

Cosign prompts for the private-key passphrase during publication. Save the
printed exact image reference, then build the ISO from it:

```sh
go run ./cmd/soda-image --architecture "$ARCH" iso \
  --image registry.soda.local/soda/os@sha256:EXACT_DIGEST \
  --archive ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub

go run ./cmd/soda-image --architecture "$ARCH" publish \
  --archive ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --iso ".artifacts/images/SodaOS-0.3.1-${ARCH}.iso"
```

The final command re-checks the exact ISO payload, writes an
architecture-named signed release record under `.artifacts/releases/`, and
updates only `:current-$ARCH` last. Treat that architecture-specific tag solely
as discovery: installations and updates use the resolved, signed exact digest.
The ISO has a matching `.sha256` file, and the publisher independently
re-inspects its embedded exact payload before releasing it.

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

`check` resolves the installed architecture's `current-aarch64` or
`current-x86_64` tag once, uses the registry CA to authenticate HTTPS transport,
and uses Soda's embedded Cosign public key to verify the exact image signature.
It accepts only a signed exact digest matching the installed platform with
state schema 3. `stage` downloads that digest using bootc's container signature
policy but leaves the booted deployment and running Soda services untouched.

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
