# Install on premises

Install Soda from the architecture-matched network ISO with graphical Anaconda and then complete common first-boot setup.

## Prerequisites

- An x86-64 or AArch64 machine matching the installer architecture.
- A target disk whose existing contents may be permanently erased.
- A display and keyboard, remote console, or equivalent installation console.
- Wired or otherwise Anaconda-supported network access during installation.
- Boot media large enough for the ISO.
- One SSH public key and either a trusted LAN or a Tailscale auth key.

The installer retrieves the exact signed Soda OCI image recorded by the
release, so the machine must have working network and DNS access during
installation.

## Download and verify the installer

1. Open [Download](/download) or the [latest GitHub
   Release](https://github.com/LevitateOS/soda-os/releases/latest).
2. Select the ISO, checksum, release record, and Sigstore bundle for the
   machine architecture.
3. Verify the ISO:

   ```sh
   sha256sum --check SodaOS-*.iso.sha256
   ```

4. Set `RECORD` to the downloaded release-record filename and verify it:

   ```sh
   RECORD='soda-os-VERSION-ARCHITECTURE.release.json'
   cosign verify-blob \
     --bundle "$RECORD.sigstore.json" \
     --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
     --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
     "$RECORD"
   ```

   Replace `VERSION` and `ARCHITECTURE` with the downloaded filename. See
   [Sigstore's verification guide](https://docs.sigstore.dev/cosign/verifying/verify/)
   for details.
5. Confirm that the release record names the expected architecture, the ISO
   checksum matches, and the exact `soda_image_reference` is an immutable
   digest rather than a moving tag.

Stop if any verification fails.

## Prepare boot media

Write the verified ISO to removable media with a tool that performs a raw disk
image write. Select the whole removable device, not one of its partitions.
Eject it cleanly after the write completes.

Writing an ISO destroys the previous contents of the selected removable
device. Double-check the target before starting.

## Install with Anaconda

1. Boot the Soda installer in the machine's native architecture.
2. Wait for graphical Anaconda to open.
3. Configure the installation language and keyboard if offered.
4. Select the target disk and storage layout. Confirm only after checking which
   disks Anaconda will erase or reformat.
5. Configure networking and the hostname. The network must be usable before
   installation begins.
6. Start installation. Anaconda retrieves and deploys the exact Soda OCI
   digest named by the release.
7. Wait for successful completion, remove the installer media, and reboot into
   the installed system.

Administrator credentials and the Tailscale choice are completed after reboot,
not through separate installation media.

## Expected result

The machine boots the installed Soda image from its target disk and presents
the common interactive first-boot setup on the console.

## If something fails

- If the installer cannot retrieve the image, verify network, DNS, system time,
  and anonymous access to the exact digest shown in the release record.
- If storage is wrong, stop before beginning installation and return to
  Anaconda's storage screen.
- If installation fails after disk mutation, retain the Anaconda logs and
  reinstall after correcting the cause. Do not assume the partially installed
  system is usable.

## Next step

Continue with [Make the first connection](30-first-connection.md).
