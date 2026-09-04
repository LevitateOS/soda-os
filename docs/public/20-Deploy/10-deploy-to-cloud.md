# Deploy to a cloud

Import the architecture-matched Soda QCOW2, protect its services from public ingress, and finish setup from the VM console.

## Prerequisites

- A cloud or virtualization platform that imports QCOW2 disks.
- A usable graphical or serial VM console.
- An x86-64 or AArch64 instance matching the downloaded artifact.
- A virtual disk large enough for the operating system, workspaces, source,
  dependencies, and project data.
- A firewall or security group that can block public ingress.
- One SSH public key and a Tailscale auth key.

Soda cloud onboarding does not use cloud metadata or public SSH. If the
provider cannot expose a VM console, it cannot complete the supported setup
path.

## Download and verify the image

1. Open [Download](/download) or the [latest GitHub
   Release](https://github.com/LevitateOS/soda-os/releases/latest).
2. Select the `.qcow2.zst`, checksum, release record, and Sigstore bundle for
   the VM architecture.
3. Keep the four files together in one directory.
4. Verify the compressed download:

   ```sh
   sha256sum --check SodaOS-*.qcow2.zst.sha256
   ```

5. Set `RECORD` to the downloaded release-record filename and verify it:

   ```sh
   RECORD='soda-os-VERSION-ARCHITECTURE.release.json'
   cosign verify-blob \
     --bundle "$RECORD.sigstore.json" \
     --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
     --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
     "$RECORD"
   ```

   Replace `VERSION` and `ARCHITECTURE` with the downloaded filename. This pins
   Soda's release workflow identity and GitHub Actions issuer. See [Sigstore's
   verification guide](https://docs.sigstore.dev/cosign/verifying/verify/) for
   the meaning of the identity and bundle checks.
6. Confirm that the record names the expected architecture and that its
   `qcow2_zst_sha256` matches the verified compressed file.

Do not import an artifact after a failed checksum, signature, architecture, or
record check.

## Import and size the disk

1. Decompress the image:

   ```sh
   zstd --decompress SodaOS-*.qcow2.zst
   ```

2. Import the resulting QCOW2 as the VM's boot disk.
3. Enlarge the virtual disk to the capacity the team needs. Soda grows its
   final root partition and filesystem to the supplied volume.
4. Attach a console and configure firmware and machine type supported by the
   provider for the selected architecture.
5. Do not attach account passwords, SSH keys, Tailscale keys, or setup scripts
   as instance metadata.

## Protect network access

Before the first boot, block public Internet ingress to all Soda services,
including ports 22, 9090, and 30000. A cloud Soda machine is administered and
used through Tailscale after enrollment.

The VM still needs outbound access for Tailscale, Forgejo-related Git hosts,
development dependencies, and later OS image retrieval.

## Boot and finish setup

Start the VM and open its console. Soda presents the common interactive
first-boot setup. Complete it as described in [Make the first
connection](30-first-connection.md).

Do not close the console until setup confirms that an administrator is ready
and Tailscale is connected. Cloud deployments do not select LAN-only access.

## Expected result

The QCOW2 boots from the enlarged disk, first-boot setup is visible on the
console, and no Soda service is reachable from the public Internet.

## If something fails

- A checksum or signature failure means the artifact must not be used.
- A boot failure usually indicates an architecture, firmware, or disk-import
  mismatch; compare those settings with the provider's QCOW2 documentation.
- If the filesystem does not reflect the enlarged virtual disk, stop before
  creating project data and retain the console output for diagnosis.
- If Tailscale cannot connect, correct outbound networking or the auth key from
  first-boot setup. Do not open public SSH as a workaround.

## Next step

Continue with [Make the first connection](30-first-connection.md).
