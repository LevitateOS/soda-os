# Base system

Soda OS 0.4.0 is built from the Fedora 44 bootc base image pinned by digest in
the Soda image contract. Fedora supplies the kernel, userspace, package
manager, systemd, SELinux policy, SSH server, bootc runtime, and their RPM
provenance. Soda OS is an independent remote-development appliance and is not
endorsed by the Fedora Project.

## Native host administration

Stock Cockpit supplies authentication and its Fedora-owned overview, metrics,
services, logs, accounts, terminal, storage, and networking pages. Those pages
read Linux-owned state directly. Soda adds branding and the focused Projects
package but ships no separate telemetry sampler, translated host-status API, or
custom host-administration backend.

## Immutable development tools

The reviewed development-tool collection is installed in this image through
exact architecture-owned Fedora package locks plus checksum-locked Bun and Tea
RPMs. `/usr/share/soda/toolset-commands.txt` lists the command contract, with
one command per line. The same system commands, including `gh` and the
Forgejo-compatible `tea`, are available to primary and derived workspace
accounts through ordinary `PATH`. Supported human onboarding creates a private
Tea login in the primary home; workspace setup copies that opaque configuration
once into the new derived home. Token lifecycle and later changes remain
user/Forgejo-owned. User packages, caches, project dependencies, and CLI
authentication remain in their homes. Soda has no shared forge login, parsed
token store, synchronization, runtime toolchain profiles, downloader, readiness
state, persistent toolchain directory, or toolchain mount.

## Manual image lifecycle

Automatic image updates are disabled. A Linux administrator inspects and
selects an exact Soda image through native `bootc` operations:

```sh
sudo bootc status
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Supported fallback uses the same sequence with an earlier exact Soda image
digest. Direct `bootc rollback` is unsupported because it may restore the
earlier deployment's historical `/etc` instead of preserving current account
state.
