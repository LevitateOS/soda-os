# Deploy to a cloud

Import the architecture-matched Soda QCOW2, protect its services from public ingress, and provision it with standard cloud-init user-data.

## Prerequisites

- A cloud or virtualization platform that imports QCOW2 disks.
- A usable graphical or serial VM console.
- An x86-64 or AArch64 instance matching the downloaded artifact.
- A virtual disk large enough for the operating system, workspaces, source,
  dependencies, and project data.
- A firewall or security group that can block public ingress.
- One SSH public key and a Tailscale auth key.

Supply standard cloud-init user-data through the provider or VM tooling.
Keep console access for native administration; public SSH is not an onboarding
path. If enrollment is omitted, supply a Linux password hash so an administrator
can log in on the console and complete network configuration.

## Download and verify the image

1. Open the [latest GitHub
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
5. Supply the standard cloud-init user-data shown below through the provider's
   supported user-data or instance-metadata facility. Do not attach a separate
   credential disk, use public SSH as a bootstrap, or run a Soda setup script.

## Protect network access

Before the first boot, block public Internet ingress to all Soda services,
including ports 22, 9090, and 30000. A cloud Soda machine is administered and
used through Tailscale after enrollment.

The VM still needs outbound access for Tailscale, Forgejo-related Git hosts,
development dependencies, and later OS image retrieval.

## Deploy on Scaleway

Use Scaleway's snapshot import path for the verified, decompressed QCOW2.
You need permissions for Instances, Object Storage, and security groups in the
chosen Scaleway project, plus an authenticated Scaleway CLI.

1. Choose an Instance type whose architecture matches the Soda image and an
   Availability Zone for the server. Create an Object Storage bucket in the
   same region as that zone.
2. Upload the `.qcow2` file to the bucket. Keep its `.qcow2` extension; do not
   upload the compressed `.zst` file as the disk image.
3. Import the object as an Instance snapshot in the chosen zone. Scaleway's
   [snapshot import guide](https://www.scaleway.com/en/docs/instances/how-to/snapshot-import-export-feature/)
   describes the console path; its
   [CLI guide](https://www.scaleway.com/en/docs/instances/api-cli/managing-instance-snapshot-via-cli/)
   documents Block Storage imports and the required volume size. Wait for the
   import to complete before creating the server.
4. Create a dedicated
   [security group](https://www.scaleway.com/en/docs/instances/how-to/use-security-groups/)
   in the same zone. Enable stateful filtering, set inbound traffic to drop,
   allow outbound traffic, and add no public ingress rules for Soda services.
   Stateful filtering permits replies to connections the server initiates.
5. Use the
   [Instance CLI](https://cli.scaleway.com/instance/#create-server)
   to create the server from that snapshot with `stopped=true`,
   `security-group-id` set to the dedicated group, and local disk boot. Select
   the root-volume option for the snapshot's storage type and size it for the
   team's data. Confirm the attached boot volume and security group before
   starting the Instance. Supply standard cloud-init user-data through the
   platform before first boot, including the Linux account and network enrollment.
6. Start the Instance, then open **Console** from its overview in Scaleway.
   The [serial console](https://www.scaleway.com/en/docs/instances/how-to/use-serial-console/)
   is available independently of public SSH access. Use the cloud-init-created
   Linux login if console administration is needed; provide a password hash
   when console, Cockpit, or PAM login is required.

Keep the security group in place after enrollment. Connect to Soda through its
Tailscale address, not the Instance's public address.

## Provision and boot

Use the VM platform's native user-data facility before first boot. For local
libvirt VMs, virt-install can deliver a user-data file with its native
cloud-init option. No Soda checkout or manually created credential ISO is
needed. Replace the account and public-key values in this standard example:

```yaml
#cloud-config
# Supply through your VM platform's user-data facility. Replace the public key.
users:
  - name: owner
    groups: [wheel]
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-ed25519 REPLACE_WITH_YOUR_PERSONAL_PUBLIC_KEY
    # To enable console, Cockpit, PAM, and password-based sudo, also supply:
    # lock_passwd: false
    # hashed_passwd: '$6$REPLACE_WITH_YOUR_PASSWORD_HASH'
disable_root: true

# Optional Tailnet enrollment: uncomment and supply a protected auth key.
# Cloud-init/provider copies of user-data may retain this key after enrollment.
# write_files:
#   - path: /run/soda-tailscale-auth-key
#     permissions: '0600'
#     owner: root:root
#     content: tskey-auth-REPLACE
# runcmd:
#   - |
#     set -eu
#     trap 'rm -f /run/soda-tailscale-auth-key' EXIT
#     tailscale up --auth-key=file:/run/soda-tailscale-auth-key
#     tailscale wait --timeout=30s
#     printf 'Tailscale enrollment succeeded.\n'
#     if ! /usr/libexec/soda/forgejo-init refresh-tailnet; then
#       printf 'Tailscale enrolled, but Forgejo address refresh failed.\n' >&2
#       exit 1
#     fi

# For LAN-only provisioning instead, explicitly select a trusted NetworkManager
# connection. Never trust a cloud public-facing connection.
# runcmd:
#   - [nmcli, connection, modify, YOUR_TRUSTED_CONNECTION, connection.zone, trusted]
#   - [nmcli, connection, up, YOUR_TRUSTED_CONNECTION]
```

Store user-data with restricted permissions. A personal public key is not a
password: key-only provisioning enables SSH but does not enable password login
to the console, Cockpit, or Forgejo PAM. Supply a password hash when those
logins are required. Use native Linux administration for later password changes.

Cloud-init and the provider may retain user-data, including auth keys and
password hashes. Removing the temporary enrollment file does not erase those
copies. Apply the team's provider and cloud-init retention policy, and never
publish user-data or enrollment credentials.

After enrollment, the native conditional Forgejo refresh applies its Tailnet
address even if Forgejo started first. A refresh failure is a provisioning
failure despite successful enrollment; inspect cloud-init and Forgejo logs,
correct the cause, and explicitly rerun the refresh. It does nothing when the
address already matches.

## Expected result

The QCOW2 boots from the enlarged disk and cloud-init provisions the Linux
account and chosen private network access. Setup still appears after an
administrator console login until they explicitly choose **Don't show Setup
automatically**; it remains available through Cockpit and manual console
launches. No Soda service is reachable from the public Internet. Register the
first Forgejo owner before teammates begin signing in, as described in [Make
the first connection](30-first-connection.md).

## If something fails

- A checksum or signature failure means the artifact must not be used.
- A boot failure usually indicates an architecture, firmware, or disk-import
  mismatch; compare those settings with the provider's QCOW2 documentation.
- If the filesystem does not reflect the enlarged virtual disk, stop before
  creating project data and retain the console output for diagnosis.
- If Tailscale cannot connect, correct outbound networking or the auth key from
  Soda Setup. Do not open public SSH as a workaround.

## Next step

Continue with [Make the first connection](30-first-connection.md).
