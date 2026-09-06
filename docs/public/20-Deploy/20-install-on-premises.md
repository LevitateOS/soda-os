# Install on premises

Install Soda on a computer or virtual machine with the network ISO and graphical Anaconda.

## Before you begin

Choose the x86-64 or AArch64 ISO matching the target CPU, then
[verify its signature and checksum](05-verify-downloads.md). Have:

- a target disk whose contents may be erased, with capacity for the OS and your
  source, dependencies, databases, containers, and backups awaiting transfer;
- a display/keyboard or VM console, and supported networking with working DNS;
- an administrator username/password and a personal SSH public key;
- boot media large enough for the ISO, or a VM with the ISO attached.

The installer downloads the release's exact OCI image. It needs Internet access
during installation; it is not an offline installer. Choose memory/CPU/storage
for your workloads and concurrent developers, not just the installer.

For a VM, use matching-architecture virtualization, UEFI firmware and storage/
network devices supported by the platform. Retain console access after install.
A bridged trusted LAN or Tailscale gives clients a route to the guest; host-only
networking does not automatically make it reachable from another computer.
For reusable disk-image provisioning, use [the QCOW2 guide](10-deploy-to-cloud.md).

## Prepare and boot the ISO

Write the verified ISO using a raw disk-image writer, selecting the whole
removable device rather than a partition. **Writing destroys that device's
previous contents.** Check its identity and capacity before confirming, then
eject cleanly. In a VM, attach the verified ISO as optical media instead.

Boot it in the target's native architecture and wait for graphical Anaconda.
Follow the [Fedora installation guide](https://docs.fedoraproject.org/en-US/fedora/latest/getting-started/)
for the installer's native storage and device controls.

## Install with Anaconda

1. Choose language and keyboard settings.
2. Select the installation disk and review the storage layout. **Installation
   can permanently erase existing data.** Confirm only the disks you intend to use.
3. Configure networking and a hostname. Check that the network is usable before
   starting the image download.
4. In **User Creation**, create your primary Linux username and password, and
   grant administrator capability. Root remains locked.
5. Start installation. Anaconda retrieves and deploys the exact release digest.
6. Wait for completion, remove/detach the ISO, and reboot from the installed disk.
7. Log in normally with the new administrator account. The welcome message shows
   hostname, connection URLs, SSH guidance, and Tailscale status.

ISO installation disables cloud-init so it does not change the Anaconda-created
accounts. There is no separate Soda Setup wizard or second credential image.

## Connect and add your key

Follow [First connection](30-first-connection.md) to open Cockpit over the
trusted LAN, add your personal public key through Accounts, and verify SSH.
For private access away from the LAN, enroll through
[Tailscale](../30-Use-Soda/40-tailscale.md). Browser authentication does not require
you to prepare a Tailscale auth key.

Firewalld retains Fedora's enabled defaults and SSH allowance; Soda also allows
Cockpit. Open Forgejo and chosen development ports explicitly through
[Networking → Firewall](../50-Operate/20-administration.md#service-endpoints).

## If installation fails

- **No image download:** check network, DNS, system time, and anonymous access
  to the exact digest in the verified release record.
- **Wrong disk or layout:** stop before installation; return to storage selection.
- **Partial installation:** preserve Anaconda's error/logs, correct the cause,
  and install again. Do not assume a partially written disk is usable.
- **VM does not boot:** compare architecture, firmware, boot order, and disk
  attachment against the VM platform's native documentation.

WSL2 on x86-64 Windows is planned for a future release, with no WSL2 download.
This guide installs the full ISO on hardware or a VM.
