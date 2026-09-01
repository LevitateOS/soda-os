# Base system

Soda OS 0.4.0 is built from the Fedora 44 bootc base image pinned by digest in
the Soda image contract. Fedora supplies the kernel, userspace, package
manager, systemd, SELinux policy, SSH server, bootc runtime, and their RPM
provenance. Soda OS is an independent remote-development appliance and is not
endorsed by the Fedora Project.

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
