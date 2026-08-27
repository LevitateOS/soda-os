# Soda OS release and operator runbook

This is the trusted-LAN MVP release path for Fedora 44 bootc AArch64. It uses
one HTTPS registry, `registry.soda.local/soda/os`, with anonymous pulls and
publisher-only pushes. The administrator supplies the registry CA, the Soda
Cosign public key, and a passphrase-protected private key; only the CA and
public key are copied into the image.

## Build and initial install release

Run these commands from a clean Soda checkout with the pinned tools available.
The paths below are operator inputs, not repository files.

```sh
just check
just release-tools
just oci /path/to/registry-ca.crt /path/to/cosign.pub

go run ./cmd/soda-image publish \
  --archive .artifacts/images/soda-os-0.3.1-aarch64.oci.tar \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --defer-current
```

Cosign prompts for the private-key passphrase during publication. Save the
printed exact image reference, then build the ISO from it:

```sh
go run ./cmd/soda-image iso \
  --image registry.soda.local/soda/os@sha256:EXACT_DIGEST \
  --archive .artifacts/images/soda-os-0.3.1-aarch64.oci.tar \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub

go run ./cmd/soda-image publish \
  --archive .artifacts/images/soda-os-0.3.1-aarch64.oci.tar \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --iso .artifacts/images/SodaOS-0.3.1-aarch64.iso
```

The final command re-checks the exact ISO payload, writes a signed release
record under `.artifacts/releases/`, and updates `:current` last. Treat
`current` solely as discovery: installations and updates use the resolved,
signed exact digest. The ISO has matching `.sha256` and `.payload.json` files.

Boot the ISO on AArch64 UEFI and complete stock interactive Anaconda. Use the
installer for a new Soda host only; there is no Rocky in-place conversion. The
default hostname is `soda`, but the installer permits a different hostname.

## Administrator-controlled updates

On an installed Soda host, inspect and prepare an update without interrupting
Soda work:

```sh
sudo sodactl os update status
sudo sodactl os update check
sudo sodactl os update stage
```

`check` resolves `current` once, uses the registry CA to authenticate HTTPS
transport, and uses Soda's embedded Cosign public key to verify the exact image
signature. It accepts only a signed `linux/arm64` exact digest with state schema
2. `stage` downloads that digest using bootc's container signature policy
but leaves the booted deployment and running Soda services untouched.

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

## Local AArch64 acceptance

`scripts/bootc-acceptance.sh` separates repeatable evidence collection from
operator-owned authentication. Set an untracked evidence directory, the
Anaconda administrator's SSH identity, and the exact digest under test before
using `launch`, `wait`, `capture`, `workload`, or `stop`. Run `--help` for the
complete input contract.

The `launch install` operation creates a blank sparse disk and starts the stock
Anaconda ISO with AArch64 UEFI, SSH forwarded to port 2222, Cockpit forwarded
to port 9090, a passive serial log, and a QMP lifecycle socket. The operator
alone completes Anaconda. After installation, enroll the administrator's
public key and record the disposable host key once, then use a normal `ssh -tt`
session for password and `sudo` prompts:

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

Use the graphical console only for Anaconda. Use QMP only for observation and
clean shutdown. The acceptance path does not inject QEMU keys, store
passwords, enable passwordless sudo, take polling screenshots, or repair a
failed final disk.
