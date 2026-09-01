# Current Soda OS architecture

The product contract and ownership rules live in
[principles.md](principles.md) and [architecture-reset.md](architecture-reset.md).
This document describes the current implementation after the native workspace,
native host-administration, and immutable-toolset slices. It is implementation
evidence, not an independent source of product requirements.

## Native ownership

- Fedora bootc owns the base operating system, packages, systemd, SELinux, PAM,
  Linux accounts, and immutable-image deployment.
- Linux and `wheel` own human identity and administrator status.
- OpenSSH owns interactive shells, direct commands, SCP, SFTP, and key
  authentication.
- Forgejo or an external Git host owns repositories, collaboration, access,
  and repository lifecycle.
- Stock Cockpit owns browser authentication, sessions, and generic host
  administration through its Fedora-owned system, metrics, service, journal,
  account, terminal, storage, and networking packages.
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

## Immutable development tools

The reviewed development-tool collection is installed into the bootc image
from exact architecture-owned package locks. Fedora RPMs supply the language
runtimes, compilers, build systems, Git and SSH clients, container tools,
utilities, archives, and editors. The one tool without a Fedora package, Bun,
is installed from an architecture-specific checksum-locked upstream artifact
through a narrow local RPM. That RPM owns only the executable and its license;
it has no downloader or updater.

`/usr/share/soda/toolset-commands.txt` records the approved command-level
contract. Every listed command is available through ordinary system `PATH` to
primary and derived accounts. Language packages, caches, virtual environments,
and project-local dependencies remain user-owned state in ordinary homes and
workspaces. Soda has no runtime toolchain profiles, resolver, readiness state,
download service, version reconciliation, toolchain mount, or persistent
toolchain directory.

## Native image lifecycle

Linux administrators inspect and select exact Soda image digests through
native `bootc status` and `bootc switch` operations. Automatic updates remain
disabled. Supported fallback creates a new deployment from an earlier exact
Soda reference through the same switch path, preserving current `/etc` and
`/var`; direct `bootc rollback` is unsupported.

Soda ships no runtime release-discovery client, translated deployment state,
update API, CLI wrapper, background updater, retry, or recovery service.

## Temporary residual runtime

The architectural reset is issue-ordered. Until the final control-plane
deletion milestone lands, the reduced `sodad`/`sodactl` surface retains only a
`Health` RPC over the local Unix gRPC socket. It has no Soda identity, project,
repository-projection, provisioning, update, telemetry, toolchain, or SQLite
authority and does not participate in the Projects workflow. The daemon,
socket, generated protobuf code, gRPC transport, and health CLI are temporary
residual infrastructure, not accepted product architecture.

## Sibling architectures

AArch64 and x86-64 are equal sibling architectures. Shared source owns product
behavior; platform files select only inputs that genuinely differ. Every RPM,
image, ISO, installation, inspection, and acceptance claim must be produced on
matching-native hardware. Evidence from one sibling does not qualify the other.
