# Bootable scaffold scenario

This scenario is the release gate for Soda OS 0.1.0. Generated images, disks,
keys, credentials, logs, databases, and RPMs remain under ignored artifact
paths.

## Build gate

1. Run `just check`.
2. Run `just verify-iso`; require an authenticated Rocky 10 release signature
   and the configured SHA-256 match.
3. Run `just rpm`; require exactly `soda-release`, `soda-runtime`, and
   `soda-cockpit`, followed by a successful disposable DNF transaction.
4. Run `just iso` and `just iso-test`; require AArch64 UEFI boot records, the
   Soda RPM repository, the selected Kickstart, and the Soda OS boot-menu
   branding in each resulting ISO.

## Installed-system gate

Install the automated ISO into a new 64 GB AArch64 disk. It must power off
without operator input. Boot only the installed disk and require:

- `/etc/os-release` identifies `Soda OS 0.1.0` and `ID=sodaos`;
- SELinux is enforcing;
- `sshd`, `sodad`, `soda-authd`, `soda-cockpit`, `avahi-daemon`, and
  `firewalld` are active;
- firewalld exposes SSH and TCP 9090;
- the cockpit health endpoint responds over HTTPS;
- valid PAM credentials sign in and invalid credentials do not;
- the first Anaconda account can be imported as a Soda administrator.

## Collaboration gate

Create two people and one empty project, then add both collaborators. Require:

- one locked `soda-p-<slug>` account owns the repository and both worktrees;
- each person's SSH key enters only that person's default worktree;
- interactive shells, direct commands, SFTP/remote-IDE bootstrap, and port
  forwarding remain available through the forced-command gateway;
- commits in the two worktrees use each person's configured name and email;
- Web, Python, Rust, and Go profiles reach `ready` with recorded versions;
- a failed provisioning job is visible and changes state only after an
  explicit retry.

The graphical release-image equivalent is performed in UTM using
`docs/utm.md`; credentials and disk choices remain operator-owned Anaconda
inputs.
