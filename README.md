# Soda OS

![soda os](assets/branding/source/soda-logo-horizontal.svg)

Soda OS is a Fedora bootc development appliance for trusted local networks,
with equal AArch64 and x86-64 sibling support. A thin client connects over SSH
to project-owned development environments and uses a small Go/HTMX cockpit for
team, project, personal workspace, and development-environment management.

This repository is independent from LevitateOS. It borrows the separation
between declarative distro specifications, Go orchestration, explicit
contracts, and scenario tests. Fedora supplies the pinned Fedora 44 bootc
base, kernel, userspace, RPM/DNF, systemd, SELinux, and SSH.

## Repository layout

- `cmd`: executable-specific daemon, CLI, SSH gateway, and image-builder code
- `cockpit`: Cockpit and PAM executables, daemon client, and HTTP presentation
- `internal`: runtime control plus the artifact pipeline under `internal/build`
- `distro`: Soda identity, profiles, distribution locks, and Fedora base metadata
- `packaging`: bootc and RPM inputs grouped by shipped package
- `assets`: canonical Soda branding sources and rendered assets
- `docs`: architecture, artifact, branding, and release operations
- `tests/acceptance`: blank-disk bootc acceptance scenario and runner
- `scripts` and `tools`: repository checks and developer tooling

## Development

```sh
just check
ARCH=x86_64 # or aarch64
just rpm "$ARCH"
just oci "$ARCH"
just iso "$ARCH" ".artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar"
```

Build artifacts are written under `.artifacts/` and are never committed.
`just rpm` builds exactly the four locked local Soda RPM inputs. `just oci`
builds those RPMs and emits
`.artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar` without loading or publishing
the image. `just iso` derives the exact image digest from that local archive and
embeds it in a platform-matched installer without a registry, signing key, or
network publication step. Architecture selection is always explicit; neither
sibling is a default or fallback.
The package lock pins every Fedora package added to the immutable base, and the
finished image contains a complete RPM inventory plus its verified SHA-256
checksum.

Local development does not publish or sign images. Optional release metadata
records preserve the exact local archive digest, image labels, RPM inventory,
and ISO checksum. The paired GitHub publisher can distribute completed sibling
artifacts, but it does not participate in local OCI or ISO construction. See the
[release and operator runbook](docs/release-operations.md) for the exact
commands and [runtime image and installer contract](docs/installer.md) for the
artifact boundary.

Initial installation is a fresh platform-matched bootc installation from the
generated AArch64 or x86-64 ISO. The stock interactive Anaconda flow selects
storage, networking, hostname, and the first Linux administrator. A mandatory
Soda Account spoke collects that person's email while the stock User Creation
screen remains authoritative for full name, username, password, and
administrator selection. First boot imports that account into Soda.

Mutable Soda state is preserved outside the image. The database, certificates,
projects, and toolchains physically live below `/var/lib/soda`; systemd bind
mounts retain the SSH-visible `/srv/soda/projects` and
`/opt/soda/toolchains` paths. Linux users and PAM passwords, SSH host keys,
`/etc/soda/authorized_keys`, and Soda logs remain host state.

An administrator controls OS updates explicitly:

```sh
sudo sodactl os update status
sudo sodactl os update check
sudo sodactl os update stage
sudo sodactl os update activate --confirm-reboot
```

Checking resolves the installed sibling's release-index entry once, validates
its exact digest, platform, and image metadata, and records only that digest.
Staging downloads that exact digest without changing the running
deployment. Activation is the separate, explicit maintenance-reboot action.
Soda neither polls, downloads, activates, nor reboots automatically.

Enroll the appliance with `sudo tailscale up`, then run `soda-tailnet` to print
its canonical MagicDNS identity and dashboard URL. Run
`sudo systemctl restart soda-cockpit forgejo` after first enrollment so they
load that Tailnet identity. The certificate is self-signed in this scaffold,
so the first browser visit requires an explicit trust exception. Sign in with
the imported Linux account, open **My Account**, and register each client
device's public SSH key. The same page shows the person's server-generated
public Git key for external Git hosts. Project aliases log in as that person and
send the selected project slug, so commands, Git, and SFTP run under the
person's Linux UID. External repositories accept SSH remotes only and wait for
the creator to add the displayed public key before setup continues.

See [architecture](docs/architecture.md), the [runtime image and installer
contract](docs/installer.md), the [managed listener contract](docs/networking.md),
and the [release and operator runbook](docs/release-operations.md).
