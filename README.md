# Soda OS

Soda OS is a Rocky Linux 10.2 AArch64 development appliance for trusted local
networks. A thin client connects over SSH to project-owned development
environments and uses a small Go/HTMX cockpit for people, project, worktree,
and toolchain management.

This repository is independent from LevitateOS. It borrows the separation
between declarative distro specifications, Rust orchestration, explicit
contracts, and scenario tests, while keeping Rocky Linux responsible for the
kernel, glibc, systemd, GNU Coreutils, RPM/DNF, Anaconda, SELinux, and SSH.

## Repository layout

- `crates/soda-core`: shared domain types and validation
- `crates/sodad`: privileged local daemon and SQLite state owner
- `crates/sodactl`: administrator CLI for the daemon
- `crates/soda-ssh`: forced-command SSH worktree gateway
- `crates/soda-image`: RPM and installer ISO orchestrator
- `cockpit`: standalone Go/HTMX management website and local PAM helper
- `distro`: Soda identity and toolchain profile specifications
- `packaging`: RPM, systemd, Avahi, and Kickstart inputs
- `tests`: repository-level contract and scenario tests

## Development

```sh
just check
just verify-iso
just rpm
just iso
```

Build artifacts are written under `.artifacts/` and are never committed. The
Rocky source ISO remains under `isos/` and is consumed in place.

`just verify-iso` authenticates Rocky's detached checksum signature with the
Rocky 10 release key and then verifies the configured DVD SHA-256. `just rpm`
builds and test-installs the three Soda RPMs in Rocky 10 AArch64 containers.
`just iso` produces `.artifacts/images/SodaOS-0.1.0-aarch64.iso`; `just
iso-test` produces the unattended, disposable-credential test image.

After Anaconda creates the first Linux administrator, register that account
with Soda while preserving its PAM password:

```sh
sudo sodactl people import \
  --username YOUR_USERNAME \
  --display-name "Your Name" \
  --email you@example.test \
  --role admin \
  --ssh-key "$HOME/.ssh/id_ed25519.pub"
```

The cockpit is then available at `https://soda.local:9090`. Its certificate is
self-signed in this scaffold, so the first browser visit requires an explicit
trust exception.

See [architecture](docs/architecture.md) and [UTM installation](docs/utm.md).
