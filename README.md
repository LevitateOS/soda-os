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
- `cockpit`: standalone Go/HTMX management website
- `distro`: Soda identity and toolchain profile specifications
- `packaging`: RPM, systemd, Avahi, and Kickstart inputs
- `tests`: repository-level contract and scenario tests

## Development

```sh
just check
```

Build artifacts are written under `.artifacts/` and are never committed. The
Rocky source ISO remains under `isos/` and is consumed in place.

See [architecture](docs/architecture.md) and [UTM installation](docs/utm.md).
