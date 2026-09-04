Installing Soda OS takes one architecture-matched network ISO from a blank
machine to the common interactive first-boot setup.

## Product contract

### Install with graphical Anaconda

Boot the Soda ISO and use stock graphical Anaconda. Anaconda owns storage
selection, networking needed for installation, firmware and bootloader setup,
and deployment of the exact Soda bootc image.

The Soda ISO is the only installation medium the owner prepares. All human
onboarding happens after the installed system boots.

### Complete first boot

After reboot, the common Fedora-native setup appears on the physical or virtual
console. It can also be reopened in Cockpit through the same bounded operations.

Complete these outcomes:

1. Create the first primary Linux administrator.
2. Set the administrator password.
3. Install the administrator's SSH public key.
4. Prepare the same-named Forgejo site administrator and register the public
   key.
5. Connect Tailscale or explicitly select LAN-only operation.

The setup remains available at startup until all five outcomes are complete;
machine-wide dismissal is disabled before then.

If you enter a Tailscale authentication key, Soda uses it once and removes it.
LAN-only requires no key, and Tailscale can be configured later.

### Install a reusable QCOW2

The reusable QCOW2 boots into the same first-boot setup. Supported VM and cloud
environments must provide a usable console. Console access is a prerequisite
for onboarding; public SSH is not an onboarding path.

The QCOW2 grows its final partition and filesystem to the virtual disk size.

### Choose the access mode

On a trusted local network, SSH, Cockpit, and Forgejo are directly available
over the LAN. Tailscale is optional and never blocks LAN access.

In a cloud environment, connect Tailscale before using those services. They are
not exposed to the public Internet.

Continue with [Administration](administration.md) to add people or
[Connect and develop](connect-and-develop.md) to enter a workspace.
