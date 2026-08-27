# Bootable scaffold scenario

This scenario is the release gate for Soda OS 0.2.0. Generated images, disks,
keys, credentials, logs, databases, and RPMs remain under ignored artifact
paths.

## Build gate

1. Run `just check`.
2. Run `just verify-iso`; require an authenticated Rocky 10 release signature
   and the configured SHA-256 match.
3. Run `just rpm`; require exactly `soda-release`, `soda-runtime`, and
   `soda-cockpit` in the target repository, followed by a successful disposable
   DNF transaction. Require the separate build-only `soda-installer-branding`
   RPM and inspected `product.img` outside that repository.
4. Run `just iso` and `just iso-test`; require AArch64 UEFI boot records, the
   `SodaOS-0-2-0-aarch64` label, Soda metadata, the Soda RPM repository, the
   selected Kickstart, the pinned installer profile, and only the approved Soda
   OS boot-menu entries in each resulting ISO.

## Installed-system gate

Install the automated ISO into a new 64 GB AArch64 disk. It must power off
without operator input. Boot only the installed disk and require:

- `/etc/os-release` identifies `Soda OS 0.2.0` and `ID=sodaos`;
- SELinux is enforcing;
- `sshd`, `sodad`, `soda-authd`, `soda-cockpit`, `avahi-daemon`, and
  `firewalld` are active;
- firewalld exposes SSH and TCP 9090;
- the cockpit health endpoint responds over HTTPS;
- valid PAM credentials sign in and invalid credentials do not;
- the first Anaconda account can be imported as a Soda administrator.

## Collaboration gate

Create Alice and Bob, have each activate their account and register a separate
device key, then create one empty project with both as initial team members.
Require:

- one locked `soda-p-<slug>` account owns the repository and both personal
  workspaces;
- the same `soda-<slug>` client alias uses each person's device key to enter
  that person's personal workspace directly;
- both sessions share the project UID while retaining separate Git attribution,
  HOME, XDG state, and editor state;
- interactive shells, direct commands, SFTP/remote-IDE bootstrap, and port
  forwarding remain available through the forced-command gateway;
- interactive shells show the friendly person/project context while `whoami`
  truthfully returns the shared project account;
- commits in the two workspaces use each person's configured name and email;
- revoking one device blocks a new connection without terminating existing
  sessions or affecting another device;
- Web, Python, Rust, and Go profiles reach `ready` with recorded versions;
- a failed project-setup job is visible and changes state only after an
  explicit retry.

The graphical release-image equivalent is performed in UTM using
`docs/utm.md`; credentials and disk choices remain operator-owned Anaconda
inputs.
