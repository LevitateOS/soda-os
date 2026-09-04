# Deploy to a cloud

Provision an architecture-matched Soda OS QCOW2 with NoCloud or ConfigDrive.

Use the reusable Soda OS QCOW2 to create a cloud virtual machine. This is the
primary deployment path for infrastructure owners who operate development
servers in a cloud or private virtualization platform.

The QCOW2 is a preinstalled virtual disk. **NoCloud** and **ConfigDrive** are
the two cloud-init datasource formats Soda accepts for protected first-boot
input. The provider must be able to import a QCOW2 and attach one of those
datasources as local instance media.

## Before you begin

You need:

- a cloud or virtualization account that can import QCOW2 disks;
- the Soda release assets for the virtual machine's architecture;
- the signed Soda provisioning command-line tool distributed with that release;
- a final root-volume size suitable for the team;
- the first administrator's username and password;
- that administrator's SSH public key; and
- a new one-use Tailscale authentication key for the target Tailnet.

Choose `x86_64` for an x86-64 virtual machine or `aarch64` for an AArch64
virtual machine. Do not substitute an artifact built for the other
architecture.

The password and Tailscale key are secrets. Store them in protected files,
keep them out of shell history and source control, and restrict the generated
provisioning media to the infrastructure owner.

## Download and verify the release

Download the architecture-matched cloud disk and its verification material:

```text
SodaOS-<version>-<architecture>.qcow2.zst
SodaOS-<version>-<architecture>.qcow2.zst.sha256
soda-os-<version>-<architecture>.release.json
soda-os-<version>-<architecture>.release.json.sigstore.json
```

Verify the downloaded archive before decompressing it:

```sh
sha256sum --check SodaOS-<version>-<architecture>.qcow2.zst.sha256
```

Verify the signed release record with Cosign:

```sh
cosign verify-blob \
  --bundle soda-os-<version>-<architecture>.release.json.sigstore.json \
  --certificate-identity \
  'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  soda-os-<version>-<architecture>.release.json
```

Then inspect the record and confirm that:

- its architecture matches the destination virtual machine;
- its compressed QCOW2 checksum matches the verified archive; and
- its Soda image reference contains an exact `sha256` digest.

Decompress the reusable disk:

```sh
zstd --decompress SodaOS-<version>-<architecture>.qcow2.zst
```

Keep the signed record with the machine's deployment records. Its exact image
digest is also the authority used for later Soda OS updates.

## Create protected provisioning input

Choose the datasource supported by the destination platform:

- `nocloud` creates media labelled `CIDATA` with NoCloud metadata; or
- `configdrive` creates media labelled `CONFIG-2` with ConfigDrive metadata.

Use `soda-image` from the signed provisioning-tool download for the same
release. Omit `--password-file` to enter the password at its protected prompt:

```sh
soda-image --architecture <architecture> cloud-input \
  --datasource <nocloud-or-configdrive> \
  --username <administrator> \
  --ssh-public-key-file <public-key-file> \
  --tailscale-auth-key-file <tailscale-key-file> \
  --output <new-seed-image>
```

The tool writes a new output image and refuses to overwrite an existing path.

The resulting medium contains:

- the first administrator's Linux username;
- the initial password;
- the SSH public key;
- the one-use Tailscale key; and
- datasource metadata, including the hostname when supplied by the platform.

Protect this output as carefully as the original password and Tailnet key.
Generate a fresh medium for each machine rather than copying one between
instances.

## Create and boot the virtual machine

1. Import the verified, decompressed QCOW2 as a reusable image or boot disk.
2. Create a virtual machine with the same architecture as the downloaded disk.
3. Set the final root-volume size before first boot. Soda expands its root
   partition and filesystem to use the provisioned virtual volume.
4. Attach exactly one protected NoCloud or ConfigDrive medium through the
   provider's local datasource mechanism.
5. Apply the owner's private-network and firewall policy. Soda's managed SSH,
   Cockpit, and Forgejo services are intended to be reached through the
   Tailnet.
6. Boot the machine.

Cloud metadata supplies the machine hostname. When the selected datasource
does not provide one, Soda uses `soda` as the hostname.

## Expected first-boot result

First boot creates the ordinary primary Linux administrator, adds it to
`wheel`, installs the SSH public key in standard `authorized_keys`, creates the
same-named Forgejo site administrator and private Tea login, and offers the
one-use authentication key to Tailscale.

The provisioning attempt consumes its protected guest-local input and removes
the temporary handoff. The reusable QCOW2 itself contains no deployment
credentials.

## Remove every retained copy of the secrets

After the first-boot attempt:

1. Detach the NoCloud or ConfigDrive medium.
2. Delete the generated medium from the operator's computer.
3. Delete any copy retained by the cloud provider, image library, or virtual
   machine configuration.
4. Remove the protected password and Tailnet-key input files when they are no
   longer needed.

Soda can clean only the copy inside the guest. The infrastructure owner is
responsible for provider-held user data, uploaded media, backups, and
snapshots.

If provisioning does not produce the complete expected result, discard the
instance, correct the input, create a fresh protected medium with a new
one-use Tailscale key, and provision a new instance.

Continue with [Make the first connection](30-first-connection.md).
