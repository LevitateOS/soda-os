# Soda OS architectural reset

**Status:** Accepted product architecture and governing ownership constraints.

**Recorded:** 2026-08-31. **Reconciled:** 2026-09-04.

The [base principles](principles.md) explain Soda's purpose and ownership
philosophy. This record defines the accepted product behavior and boundaries.
GitHub issues track bounded work; they do not override this document. Current
source and tests are implementation evidence only.

## Product contract

Soda OS is an opinionated Fedora bootc appliance for remote development by a
trusted team. A powerful Soda machine runs builds, tests, editors, agents,
databases, development servers, and project processes. Lightweight clients use
ordinary OpenSSH, Cockpit, Git, and browser links.

Soda owns the installable composition, project catalog, workspace convention,
focused Projects page, and the narrow synchronous operations required to make
that workflow coherent. The current release also retains Soda Setup as a
temporary post-install composition. Linux,
OpenSSH, Git, Forgejo or an external Git host, Cockpit, Tailscale, `mise`, and
bootc own the facts and mechanisms native to their domains.

AArch64 and x86-64 are equal sibling product targets. Architecture-specific
build, artifact, installation, and acceptance claims require matching-native
evidence.

## Installation and Soda Setup

The intended permanent architecture is one complete installation journey with
no separate Soda-owned post-install setup. The exact future division of
responsibilities must be proven against Fedora's native installation
boundaries. This direction does not authorize moving every Soda Setup screen
into a renamed custom installer or inventing another setup system.

For the current release, Soda Setup is the supported temporary workaround. Its
complete user journey remains required until a proven replacement provides the
same accepted outcomes.

Human installation uses one completed architecture-matched network ISO:

1. Boot the ISO.
2. Use stock graphical Anaconda for storage, networking, bootloader, firmware,
   and bootc deployment responsibilities that cannot safely move.
3. Boot the installed system.
4. Complete Soda Setup after the installed system boots.

The same Soda Setup state and operations serve ISO-installed and reusable
QCOW2 systems. Soda Setup appears on the physical, VM, or supported cloud
console and is also available in stock Cockpit. These are two surfaces for one
setup, not two onboarding implementations, and Soda Setup can always be opened
again later.

Soda Setup remains available on startup until machine-wide dismissal.
Dismissal is disabled until all of these are true:

- a primary Linux administrator exists;
- its password is set;
- its SSH public key is installed;
- the same-named Forgejo administrator is ready; and
- Tailscale is connected or the owner explicitly selected **Allow access from
  the local network** for the current connection.

A Tailscale authentication key, when used, is entered after installation,
consumed once, and removed. Trusting the current local connection requires no
Tailscale key. Tailscale remains a separate choice, can be configured later,
and never disables trusted local-network access.

There is no human-facing repository checkout, internal Go or xorriso command,
OEMDRV, second credential image, separate provisioning medium, NoCloud,
ConfigDrive, partial cloud-data merge, `soda-image cloud-input`, or public-SSH
bootstrap. Supported cloud environments must provide a usable VM console.

Automatic LAN/cloud detection and Anaconda enable/disable choices are future
ideas, not current requirements.

## Access and networking

Soda is cloud-first, not cloud-only.

- On a trusted local network, OpenSSH, Cockpit, and Forgejo are directly
  reachable over the LAN. Tailscale must not block this path.
- In cloud environments, those services are reachable through Tailscale and
  are never exposed to the public Internet.
- Loopback access remains available to the owning local services.

Projects choose their own non-conflicting host ports. A normal development-
server URL sent to a teammate works over LAN or Tailscale, including hot
reload. Soda has no Share button, port registry, process tracker, server
discovery list, proxy, network namespace manager, or container controller.

## Trust and human identity

All team members are trusted. Administrator status is a capability boundary,
not a hostile-user model. Soda does not invent approval, transfer, archival,
preservation, recovery, or malicious-administrator policy.

Each person has one ordinary primary Linux account. Linux owns its username,
UID, password, groups, home, permissions, and processes. Membership in `wheel`
is the only administrator fact. Stock Cockpit Accounts and ordinary Linux tools
own account listing and administrator promotion. Soda has no person database or
parallel role system.

Primary accounts exist for identity and administration. Development happens
only in derived workspaces.

For each selected person-project pair, Soda creates one derived Linux workspace
account with its own UID, private home, complete Git clone, dependencies,
caches, mutable files, project data, and processes. Linux-native state or a
deterministic convention distinguishes workspace accounts without a Soda
identity database.

Before workspace creation mutates anything, the primary account must have a
valid public key in its standard `~/.ssh/authorized_keys`. Soda copies the
current public keys once into the new workspace's standard `authorized_keys`.
It copies no private key and performs no later synchronization.

Humans connect directly to the workspace account through ordinary OpenSSH.
Interactive shells, commands, automation, SCP, and SFTP run as the derived UID
in its real home. Soda has no forced command, selector, synthetic home, custom
SFTP dispatcher, or SSH gateway.

## Forgejo, Git, Tea, and GitHub CLI

Bundled Forgejo owns users, public keys, repositories, authorization,
collaboration, issues, pull requests, releases, sessions, tokens, and native
deletion consequences. External Git hosts own the equivalent facts for their
repositories.

The initial first-boot composition creates the same-named Linux and Forgejo
administrator. Each later supported person receives a matching ordinary
Forgejo account. Soda registers that person's SSH public key with Forgejo. Git
uses SSH.

Workspace accounts remain Linux-only development identities and never become
Forgejo users. Linux `wheel` membership does not grant Forgejo administration.
Soda stores no Forgejo identity mirror, role projection, shared token, copied
password verifier, repository mapping, or synchronization state.

Tea and GitHub CLI are available in every workspace. Authentication is manual
and separate in each workspace. Soda never creates or copies Tea tokens,
Tea configuration, gh configuration, private keys, or other Git-host
credentials. There is no credential broker.

## Projects and workspaces

Stock Cockpit provides one Soda Projects page backed by one shared appliance
project catalog. Every primary human can view and edit the shared project list.
The catalog is not a permission, membership, credential, workspace, process,
port, container, job, retry, or runtime-status database.

No closed project metadata field list is approved. In particular, the former
name-and-Git-address-only and exact-three-fields rules are rejected. Any new
user-visible project information requires an explicit product decision.

A project may refer to an existing Git remote or begin as a native empty
repository in bundled Forgejo. Selecting **Set up for me** synchronously leaves
one derived workspace account and a complete clone under that account's
`$HOME/Projects`. Git and the initiating user own authentication. Soda retains
no credential, partial-workflow state, job, retry record, or reconciliation
record.

Each person may remove only their own workspace.

Only an administrator may remove an entire project. That action permanently:

1. terminates the project's local workspace sessions and processes;
2. deletes every derived workspace account, home, clone, dependency, and other
   explicitly Soda-created local workspace path, including uncommitted work;
3. removes the shared Soda project entry last; and
4. leaves the canonical Forgejo repository intact.

The trusted team coordinates before removal. Soda adds no approval, transfer,
archive, preservation, rollback, recovery, or hostile-admin policy. If a step
fails, stop and report exactly what succeeded and remains so an administrator
can retry explicitly.

Supported person deletion is administrator-only and ordered:

1. remove that person's workspaces;
2. delete the Forgejo account and report Forgejo's native consequences; and
3. delete the primary Linux account last.

Failure stops immediately and exposes the partial result. There is no rollback
or hidden completion. Soda does not invent repository transfer, archival, or
preservation. Generic Cockpit or `userdel` account deletion remains a
non-cascading Linux operation and triggers no watcher or reconciliation.

## Cockpit and privileged operations

Stock Cockpit owns browser authentication, sessions, TLS, account management,
host overview, metrics, services, logs, terminal, storage, and networking.
Soda adds branding, the Projects page, and access to the same Soda Setup state
and bounded operations used on the console.

Soda may retain fixed, one-shot privileged operations only for accepted
first-boot, catalog, workspace, Forgejo public-key registration, tool-scope,
project-removal, and person-removal transitions that genuinely require root.
They accept only bounded product inputs and expose no arbitrary command, path,
UID, process selector, credential, or general account/repository API.

Soda ships no separate dashboard, web server, session service, generic daemon,
runtime API, database, RPC contract, control socket, or generic privileged
bridge.

## Development tools

`mise` is the approved owner of development-tool installation, versions, and
project toolchain configuration. Soda may offer multiple convenience choices
when creating a project or workspace. Examples such as Go, Rust, web, and
Python are not a closed list.

Later installation chooses either **my workspace** or **this project**. Any
project user may add shared project tools; there is no approval or membership
subsystem. Shared project tools are stored once and reused by that project's
workspaces. Upstream tool managers own their native shared download caches.
Soda owns no cache format, cache service, downloader, package manager, version
manager, profile system, or toolchain database. Installed dependencies and
other mutable development state remain private to each workspace.

Coding assistants are personal and workspace-specific. A person selects them
per workspace and logs into each separately. Credentials are never copied.

The removed Soda runtime toolchain manager remains absent. The current custom
Soda Bun package and broad immutable image-wide language/compiler toolset are
rejected implementation debt, not product requirements.

## Native image lifecycle

Administrators choose updates manually through native bootc behavior. Fedora's
automatic bootc update timer is disabled. Soda has no update discovery client,
translated deployment state, custom updater, update API, update page, or
automatic reboot behavior.

Supported fallback selects an earlier exact signed Soda OCI digest through a
native bootc path that preserves current identities, groups, homes, catalog,
workspaces, Forgejo state, Tailscale identity, SSH state, and other authoritative
mutable data. Direct `bootc rollback` is unsupported unless independently
proved to preserve that invariant. Soda adds no recovery engine.

## Release and acceptance evidence

A push to protected `production` coordinates one matching-native build for each
architecture and publishes OCI, network ISO, and compressed reusable QCOW2
outputs. Each release image B is built exactly once, checked, and published
unchanged. Release CI never rebuilds a release copy.

Fallback image A is the previous signed published OCI selected by immutable
digest. CI downloads it; it never rebuilds A or unused historical ISO/QCOW2
artifacts.

Expensive boot, graphical installation, first-boot provisioning, product,
update, and fallback tests run beforehand on user-controlled matching-native
machines. They produce one signed strict JSON acceptance record for the exact
source commit. The record identifies its schema, source commit, acceptance-
suite revision or digest, both architectures, required scenarios and results,
previous fallback digest, completion time, and approved signer.

The signed record is an authenticated claim about those earlier tests. It is
not a claim that later CI-built bytes were boot-tested. Cosign/Sigstore owns the
signature boundary; Soda creates no attestation service.

Release CI verifies that record, runs cheap source and unit checks once, builds
x86-64 and AArch64 once in parallel on matching-native builders, performs only
structural artifact, identity, checksum, signature, and remote-publication
checks, then publishes the exact checked outputs unchanged.

Release CI runs no VM, product acceptance suite, fallback suite, or
acceptance-only Tailscale enrollment. The wall-clock target is 30 minutes. At
45 minutes it reports the active slow stage and continues; timing is not a hard
stop.

Strict release records, immutable digests, checksums, Cosign verification,
remote asset verification, anonymous image retrieval, clean checkout, and
production-commit verification remain required.

## Forbidden architecture

Soda must not create a general daemon or runtime API, separate identity or
permission database, repository mirror or membership model, credential broker
or shared-token system, custom SSH gateway, container-controller workspace
model, Soda-owned dependency cache or downloader, custom OS updater, or
workflow/job/retry/reconciliation platform.

A narrow integration is allowed only for the Projects UI/catalog, workspace
accounts, Forgejo public-key registration, `mise` project/workspace scopes,
upstream caches, optional workspace assistants, the current temporary Soda
Setup composition, and fixed one-shot privileged actions.

## Current implementation

At source checkpoint `5cf31df`, the repository has completed much of the old
control-plane deletion, native workspace composition, direct OpenSSH, stock
Cockpit, native bootc lifecycle, and artifact construction. Historical
matching-native reset evidence exists for both architectures.

The source still implements several superseded paths:

- mandatory protected OEMDRV and installer-time account/Forgejo provisioning;
- NoCloud, ConfigDrive, `cloud-input`, and a separate cloud finalizer;
- Tailnet-only nftables exposure for managed services;
- an exact three-field catalog contract;
- Soda-created Tea tokens/configuration and copying into workspaces;
- the custom `soda-bun` build and broad immutable tool manifest;
- person deletion that does not delete the Forgejo account; and
- release CI that rebuilds fallback A and runs VM/Tailscale acceptance.

These are implementation debt, not accepted behavior. Existing tests and old
artifact evidence prove only those historical paths and must be replaced as
their owning issues land. No public release or finished release artifact is
claimed by this record.

## Engineering verification

The following remain engineering questions, not product decisions:

1. The smallest Fedora-native console/Cockpit Soda Setup implementation and
   bounded privilege split for the current release, plus the future path to one
   complete installation journey without a separate Soda-owned post-install
   setup.
2. Exact LAN and cloud firewall/service binding on Fedora 44.
3. Forgejo public-key registration through the exact shipped version.
4. Catalog syntax, path, fields required by approved UI, and concurrency.
5. Workspace naming, classification, staging, and process removal.
6. `mise` operation on Fedora 44 with enforcing SELinux, both architectures,
   shared permissions, concurrency, workspace scope, and project scope.
7. Upstream-native shared cache behavior without Soda-owned cache state.
8. Matching-native ISO/QCOW2 first-boot and volume-growth behavior.
9. Signed acceptance-record schema and Cosign identity verification.
10. Build-once CI artifact handoff and the 30-minute performance target.

If a native hypothesis fails, return the exact constraint for review. Failure
does not authorize a daemon, database, broker, alternate onboarding path,
fallback package manager, retry queue, or reconciliation service.

## Current-release product-level acceptance criteria

The reset is complete when both matching-native architectures demonstrate:

1. One network ISO reaches graphical Anaconda and Soda Setup
   without separate human provisioning media.
2. The same Soda Setup state completes on ISO and QCOW2 through console or
   Cockpit, and cannot be dismissed before all required outcomes or explicit
   trust of the current local connection.
3. LAN exposes SSH, Cockpit, Forgejo, and normal development-server links;
   cloud exposes them only through Tailscale.
4. Linux and `wheel` remain authoritative for people and administrators.
5. Each person's public SSH key is registered in Forgejo, and Git uses SSH.
6. Every selected person-project pair receives a separate Linux account, home,
   full clone, dependencies, processes, and mutable state.
7. Public keys are copied once; Tea and gh authenticate manually per workspace;
   no private or CLI credential is copied.
8. Everyone can view and edit the shared project list without a closed field
   schema or membership model.
9. A person removes only their own workspace; an administrator removes a whole
   project and every local workspace while preserving the canonical Forgejo
   repository.
10. Person deletion removes workspaces, then the Forgejo account, then the
    primary Linux account, exposing partial failure without rollback.
11. Normal development-server links work over LAN and Tailscale without Soda
    port or process tracking.
12. `mise` provides workspace and shared-project tool scopes and upstream cache
    reuse without a Soda toolchain subsystem.
13. Native manual update and fallback preserve authoritative mutable state.
14. The signed pre-release record covers both architectures and exact source.
15. Release CI builds each B once, structurally verifies and signs the exact
    outputs, and publishes them unchanged without VM acceptance.
16. No forbidden control-plane, credential, toolchain, updater, or recovery
    machinery remains.

## Issue ownership and order

The current dependency order is:

1. [#40: current Soda Setup composition](https://github.com/LevitateOS/soda-os/issues/40), [#42: graphical one-ISO proof](https://github.com/LevitateOS/soda-os/issues/42), and [#45: console-onboarded QCOW2](https://github.com/LevitateOS/soda-os/issues/45).
2. [#15: LAN and Tailnet access](https://github.com/LevitateOS/soda-os/issues/15).
3. [#44: Forgejo SSH keys and manual workspace CLI authentication](https://github.com/LevitateOS/soda-os/issues/44) and [#33: ordered person deletion](https://github.com/LevitateOS/soda-os/issues/33).
4. [#35: shared projects and private workspaces](https://github.com/LevitateOS/soda-os/issues/35), [#37: native Git-host ownership](https://github.com/LevitateOS/soda-os/issues/37), and [#32: focused Cockpit workflows](https://github.com/LevitateOS/soda-os/issues/32).
5. [#24: mise-owned development tools](https://github.com/LevitateOS/soda-os/issues/24).
6. [#25: signed matching-native acceptance evidence](https://github.com/LevitateOS/soda-os/issues/25), followed by [#43: Go acceptance orchestration](https://github.com/LevitateOS/soda-os/issues/43).
7. [#22: reviewed dependency locks](https://github.com/LevitateOS/soda-os/issues/22), [#29: release identity](https://github.com/LevitateOS/soda-os/issues/29), and [#48: build-once signed release](https://github.com/LevitateOS/soda-os/issues/48).

[Issue #46](https://github.com/LevitateOS/soda-os/issues/46) is a separate WSL
feasibility investigation. [Issue #47](https://github.com/LevitateOS/soda-os/issues/47)
is a separate local-runner product and must not become Soda workflow authority.
