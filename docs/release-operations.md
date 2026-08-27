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
  --archive .artifacts/images/soda-os-0.2.0-aarch64.oci.tar \
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
  --archive .artifacts/images/soda-os-0.2.0-aarch64.oci.tar \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub

go run ./cmd/soda-image publish \
  --archive .artifacts/images/soda-os-0.2.0-aarch64.oci.tar \
  --registry-ca /path/to/registry-ca.crt \
  --public-key /path/to/cosign.pub \
  --signing-key /path/to/cosign.key \
  --iso .artifacts/images/SodaOS-0.2.0-aarch64.iso
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
