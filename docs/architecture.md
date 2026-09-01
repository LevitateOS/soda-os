# Current Soda OS architecture

The product contract and ownership rules live in
[principles.md](principles.md) and [architecture-reset.md](architecture-reset.md).
This document describes the current implementation after the native workspace
vertical slice. It is implementation evidence, not an independent source of
product requirements.

## Native ownership

- Fedora bootc owns the base operating system, packages, systemd, SELinux, PAM,
  Linux accounts, and immutable-image deployment.
- Linux and `wheel` own human identity and administrator status.
- OpenSSH owns interactive shells, direct commands, SCP, SFTP, and key
  authentication.
- Forgejo or an external Git host owns repositories, collaboration, access,
  and repository lifecycle.
- Stock Cockpit owns browser authentication, sessions, and host administration.
- Tailscale owns the enrolled node identity and private-network connectivity.

Soda owns only the appliance composition, the minimal project catalog, the
deterministic primary-user/project-to-workspace-account convention, one focused
Cockpit Projects package, and fixed synchronous local lifecycle operations.

## Accounts and workspaces

A primary human is a regular interactive Linux account outside the
`soda-workspaces` group. Membership in `wheel` is the sole Soda administrator
fact. A derived workspace is a regular password-disabled Linux account in
`soda-workspaces`, with a private primary group, private home, Bash shell, and
a validated Linux account marker that records the primary username and project
ID. The account name is deterministically derived from that association.

Associations are enumerated from NSS, group membership, and the validated
marker. There is no person, role, membership, or workspace database. Setup
copies the primary user's standard `~/.ssh/authorized_keys` once. A derived
account then receives ordinary OpenSSH behavior and owns its complete clone at
`$HOME/Projects/<project-id>`.

## Project catalog and lifecycle

The appliance-wide catalog is an atomically replaced, world-readable JSON
array under `/var/lib/soda/catalog`. Each entry contains exactly:

```json
{
  "id": "website",
  "display_name": "Website",
  "canonical_url": "ssh://git@example.test/team/website.git"
}
```

The immutable ID selects the local directory and deterministic account. The
display name and credential-free canonical URL are mutable. Catalog mutation
is serialized by an ephemeral lock; there is no job, retry, reconciliation, or
durable operation state.

`/usr/libexec/soda/soda-projects` runs as the Cockpit-authenticated primary
user. It reads the catalog, performs user-authenticated Forgejo or Git network
operations, and never retains credentials. It invokes
`/usr/libexec/soda/soda-workspace-helper` through an exact-path polkit action
only for validated operations requiring privilege. The helper accepts fixed
JSON requests, derives all accounts and paths internally, binds the caller from
`PKEXEC_UID`, and exposes no arbitrary command, path, UID, process selector, or
credential parameter.

Clone staging stays below the caller's `/run/user/<uid>`. Git runs only as that
caller. The helper validates and publishes the completed tree as the derived
UID. Project removal deletes validated local workspace accounts and homes
before removing the catalog entry. Soda-aware human deletion deletes validated
derived workspaces before deleting the primary account. Neither operation
deletes Forgejo accounts or canonical repositories.

## Browser and network surfaces

Fedora's `cockpit.socket` provides TCP 9090, native PAM sessions, host pages,
and package discovery. Soda ships branding plus the static
`soda-projects` package; there is no Soda web server, session store,
certificate manager, authentication helper, or daemon bridge.

OpenSSH uses TCP 22 and Forgejo uses TCP 30000. Native nftables permits these
managed ports only on loopback and `tailscale0` and rejects other ingress.
Projects select their own non-conflicting ports; Soda has no port allocator or
container/network controller.

## Native image lifecycle

Linux administrators inspect and select exact Soda image digests through
native `bootc status` and `bootc switch` operations. Automatic updates remain
disabled. Supported fallback creates a new deployment from an earlier exact
Soda reference through the same switch path, preserving current `/etc` and
`/var`; direct `bootc rollback` is unsupported.

Soda ships no runtime release-discovery client, translated deployment state,
update API, CLI wrapper, background updater, retry, or recovery service.

## Temporary residual runtime

The architectural reset is issue-ordered. Until its owning later milestone
lands, the reduced `sodad`/`sodactl` surface retains only host telemetry over
the local Unix gRPC socket. It has no Soda identity, project,
repository-projection, provisioning, update, or SQLite authority. Telemetry and
the image-resident toolchain cleanup remain explicit later
deletion/replacement milestones. They do not participate in the Projects
workflow.

## Sibling architectures

AArch64 and x86-64 are equal sibling architectures. Shared source owns product
behavior; platform files select only inputs that genuinely differ. Every RPM,
image, ISO, installation, inspection, and acceptance claim must be produced on
matching-native hardware. Evidence from one sibling does not qualify the other.
