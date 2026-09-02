# Current Soda OS architecture

The product contract and ownership rules live in
[principles.md](principles.md) and [architecture-reset.md](architecture-reset.md).
This document describes the current implementation after the protected
stock-Anaconda installer, native workspace, native image-lifecycle, native
host-administration, and immutable-toolset slices. It is implementation
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

## Installation boundary

The product ISO uses stock graphical Anaconda for storage, networking,
bootloader setup, bootc deployment, and native Linux-account creation. A
protected removable OEMDRV medium supplies the four installation inputs through
a secret-free Kickstart composition. One fixed installer-only `%pre` hook emits
native `user` and `sshkey` directives; one fixed `%post --nochroot` finalizer
performs only the initial Forgejo-administrator handoff and publication of the
one-attempt Tailscale key. The medium must be ejected and removed before
installation continues.

The former Soda Anaconda spoke, GTK/Glade UI, D-Bus service, private task
objects, custom account mutation, deployment-tree discovery, and custom `/var`
mounting have been deleted. The two hooks are absent from the installed runtime.
The installer image carries one exact-version- and source-hash-guarded Fedora
Anaconda correction that exposes SELinuxFS to Anaconda's own final relabel pass;
Anaconda still owns the relabel and the installed image remains enforcing.

## Accounts and workspaces

A primary human is a regular interactive Linux account outside the
`soda-workspaces` group. Membership in `wheel` is the sole Soda administrator
fact. A derived workspace is a regular password-disabled Linux account in
`soda-workspaces`, with a private primary group, private home, Bash shell, and
a validated Linux account marker that records the primary username and project
ID. The account name is deterministically derived from that association.

Associations are enumerated from NSS, group membership, and the validated
marker. There is no person, role, membership, or workspace database. Setup
copies the primary user's private Tea configuration and standard
`~/.ssh/authorized_keys` once, with the public key installed last. A derived
account then receives ordinary OpenSSH behavior and owns its complete clone at
`$HOME/Projects/<project-id>`.

The installer-created same-named Forgejo administrator is a Forgejo-local
account and receives a private Tea login during installation. The
administrator-only Projects **Add person** action creates an ordinary,
non-`wheel` primary Linux account, then uses that human's supplied password
through unprivileged Tea to trigger Forgejo's native PAM user creation and
personal access-token creation. A narrow Forgejo patch leaves PAM-created users
with no local Forgejo password verifier, so Linux/PAM remains password
authority.
The image owns a dedicated `soda-forgejo-shadow` group, keeps the `git` account
out of that group in NSS, and grants the group only to `forgejo.service` through
systemd's `SupplementaryGroups`. Tmpfiles maintains `/etc/shadow` as
`root:soda-forgejo-shadow` mode `0040`. The existing root-owned
`forgejo-init.service` reapplies that one named tmpfiles configuration before
Forgejo starts; this makes the service privilege independent of failures in
unrelated global tmpfiles rules without adding another service or helper. One
image-owned SELinux rule permits `systemd_tmpfiles_t` only `getattr` and
`setattr` on `shadow_t`, which are the exact permissions observed to be needed
for that metadata rule. It grants neither file-content read nor write access.
The PAM policy accepts regular Linux users, rejects `soda-workspaces`, and
applies normal account checks. Tea stores the Forgejo-owned token in the new
human's private home. Soda neither parses nor separately stores it. Workspace
setup verifies that native Tea identity, copies the opaque Tea configuration
once into the derived home, and does not synchronize later changes. Soda copies
no verifier, authentication result, role, token record, or identity record.

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
utilities, archives, editors, and GitHub CLI. Bun is installed from its
architecture-specific checksum-locked upstream artifact. The Forgejo-compatible
Tea CLI is built natively from its exact checksum-locked tagged source with the
narrow secret-input patch. Their local RPMs own only the executable and license;
they have no downloader, updater, configuration, or service.

`/usr/share/soda/toolset-commands.txt` records the approved command-level
contract. Every listed command is available through ordinary system `PATH` to
primary and derived accounts. Language packages, caches, virtual environments,
project-local dependencies, and `gh` or `tea` authentication profiles remain
user-owned state in ordinary homes and workspaces. Soda has no shared forge
login, runtime toolchain profiles, resolver, readiness state, download service,
version reconciliation, toolchain mount, or persistent toolchain directory.

## Native image lifecycle

Linux administrators inspect and select exact Soda image digests through
native `bootc status` and `bootc switch` operations. Automatic updates remain
disabled. Supported fallback creates a new deployment from an earlier exact
Soda reference through the same switch path, preserving current `/etc` and
`/var`; direct `bootc rollback` is unsupported.

Soda ships no runtime release-discovery client, translated deployment state,
update API, CLI wrapper, background updater, retry, or recovery service.

## No Soda runtime control plane

Soda ships no runtime daemon, general administration CLI, local control socket,
protobuf/gRPC API, generated protocol code, or protocol-generation toolchain.
Stock Cockpit, systemd, and ordinary Linux commands expose native host state.
The surviving `soda-runtime` RPM owns only cohesive host composition: Tailnet
enrollment and guidance, OpenSSH and nftables integration, console behavior,
and upstream service enablement.

## Sibling architectures

AArch64 and x86-64 are equal sibling architectures. Shared source owns product
behavior; platform files select only inputs that genuinely differ. Every RPM,
image, ISO, installation, inspection, and acceptance claim must be produced on
matching-native hardware. Evidence from one sibling does not qualify the other.

At implementation checkpoint `3df4431`, a fresh native x86-64 installation
proved the protected installer, initial Linux and Forgejo administrator,
Tailscale enrollment and credential deletion, stock Cockpit, Projects,
multi-user workspace isolation, direct SSH/SCP/SFTP, destructive ordering,
immutable tools, rootless Podman, exact installed image digest, and installed
absence of the deleted runtime control plane. Native B→A→B selection between
two post-control-plane images preserved current mutable state. Matching-native
AArch64 must repeat this final workflow before release-level architecture and
acceptance completion; x86-64 evidence does not qualify the sibling.
