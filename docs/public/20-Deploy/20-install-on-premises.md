# Install on premises

Install Soda from the architecture-matched network ISO with graphical Anaconda and log in normally.

WSL2 support for x86-64 Windows gaming PCs is planned for a future release.
No Soda OS WSL2 distribution is currently available; this guide covers the
network ISO installation path.

## Prerequisites

- An x86-64 or AArch64 machine matching the installer architecture.
- A target disk whose existing contents may be permanently erased.
- A display and keyboard, remote console, or equivalent installation console.
- Wired or otherwise Anaconda-supported network access during installation.
- Boot media large enough for the ISO.
- One SSH public key and either a trusted LAN or a Tailscale auth key.

The installer retrieves its embedded exact Soda OCI digest, so the machine must
have working network/DNS access and anonymous registry retrieval during installation.

## Download and verify the installer

Obtain the matching-architecture ISO and `.sha256` sidecar from your trusted
operator. ISO construction is independent of development OCI publication; do
not assume a new GitHub Release download is available. Keep the operator's
expected source and exact OCI digest for installed readback.

Verify the ISO:

   ```sh
   sha256sum --check SodaOS-*.iso.sha256
   ```

Confirm the selected architecture and expected immutable OCI digest with the
operator. Checksums detect changed bytes; self-computed sidecars are not
provenance or a production-authenticity claim. The pre-alpha flow has no Soda
release record or signature bundle. Stop if a checksum or identity check fails.

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
6. Open **User Creation**, create your Linux account and password, and select
   administrator capability. Root remains locked.
7. Start installation. Anaconda retrieves and deploys the exact Soda OCI
   digest embedded in this ISO.
8. Wait for successful completion, remove the installer media, and reboot into
   the installed system.

Anaconda creates the Linux administrator. After reboot, log in normally.
The mandatory welcome message shows connection details.
ISO installation disables cloud-init so it cannot alter the Anaconda accounts.

## Expected result

The machine boots the installed Soda image from its target disk and presents
the normal login prompt. After administrator login, use native networking tools
or Cockpit for network configuration. Compare `sudo bootc status --json` with the
operator's expected booted OCI digest before claiming the installation succeeded.

## If something fails

- If the installer cannot retrieve the image, verify network, DNS, system time,
  and anonymous access to the exact digest embedded in the ISO.
- If storage is wrong, stop before beginning installation and return to
  Anaconda's storage screen.
- If installation fails after disk mutation, retain the Anaconda logs and
  reinstall after correcting the cause. Do not assume the partially installed
  system is usable.

## Next step

Continue with [Make the first connection](30-first-connection.md).
