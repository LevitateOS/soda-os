# Base system

Soda OS is built from a Fedora bootc base pinned by digest. Fedora supplies the
kernel, userspace, package manager, systemd, SELinux policy, OpenSSH, bootc, and
their RPM provenance. Soda OS is independent and is not endorsed by Fedora.

## Native administration and access

Stock Cockpit owns browser authentication and the overview, metrics, services,
logs, accounts, terminal, storage, and networking pages. Soda adds branding and
one focused Projects package. There is no Soda telemetry service, host-status
API, or separate administration backend.

On a trusted LAN, OpenSSH, Cockpit, and Forgejo are directly reachable. Cloud
deployments use Tailscale and never expose those services to the public
Internet.

## Development tools

`mise` owns development-tool installation, versions, and project configuration.
Tools may be installed for one workspace or shared once by a project. Upstream
tool managers own their shared download caches; Soda has no cache service,
downloader, profile system, or toolchain database.

Tea and GitHub CLI are available in every workspace. Authenticate each one
manually and separately inside that workspace. Soda does not create or copy
their tokens or configuration.

## Manual image lifecycle

Automatic image updates are disabled. A Linux administrator selects an exact
signed Soda image through native bootc operations:

```sh
sudo bootc status
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Supported fallback repeats the sequence with the previous signed Soda image
digest. Direct `bootc rollback` is unsupported because it may restore the
earlier deployment's historical `/etc` instead of preserving current accounts.
