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
that workflow coherent. A separate Cockpit Tailscale page composes native Tailscale administration. Linux,
OpenSSH, Git, Forgejo or an external Git host, Cockpit, Tailscale, `mise`, and
bootc own the facts and mechanisms native to their domains.

AArch64 and x86-64 are equal sibling product targets. Architecture-specific
build, artifact, installation, and acceptance claims require matching-native
evidence.

## Installation and welcome

ISO installation uses stock graphical Anaconda for storage, networking, bootc
deployment, Linux user creation, and administrator selection. Root stays locked.
Reboot and log in normally; the welcome message shows connection details. ISO installation creates
`/etc/cloud/cloud-init.disabled`, preventing cloud-init from altering those accounts.

QCOW2 deployments use standard Fedora cloud-init delivered by VM tooling. Supply
the Linux account, personal SSH public key, optional password hash, and optional
network configuration through user-data. No Soda checkout or manually built
credential ISO is required. A key alone enables SSH authentication; it does not
supply a password for console, Cockpit, PAM, or password-based sudo.

Interactive local and SSH shells always show a concise welcome with the native
hostname, local Cockpit and Forgejo URLs, a current-user SSH command, and current
Tailscale status. It has no completion or dismissal state. Administrators can
customize the native `/etc/profile.d/soda-console-welcome.sh` entry point.
Non-interactive commands, SCP, and SFTP keep their ordinary output.

Soda preserves Anaconda/Fedora firewall defaults, with firewalld enabled and
TCP 9090 allowed for Cockpit. Administrators must allow Forgejo TCP 30000/2222
and selected development ports for LAN access through stock Cockpit's
**Networking → Firewall** page. Soda supplies no default-drop override, custom
zone, or connection-selection trust workflow. Enrolling Tailscale does not
change LAN access or an administrator's firewall choices.

Tailscale is preinstalled and tailscaled is enabled, initially unenrolled.
Administrators sign in through native browser authentication on the separate
**Cockpit → Tailscale** page. The page shows connection state, this device's
name and addresses, visible peers, eligible exit nodes, the native LAN-access
setting for exit-node use, exit-node advertisement and approval, and a link to
the official CLI documentation. It stores no authentication key or workflow state.

The page reads native state on opening and while active. Authentication URLs
are shown as soon as the native process emits them, before authentication
completes. Native status owns pending authentication and approval across page
loads. Closing the page closes its processes; it does not log out the machine.

When the page observes a connected machine, it invokes the existing
`/usr/libexec/soda/forgejo-init refresh-tailnet` command. That command compares
DOMAIN, SSH_DOMAIN, and ROOT_URL against the native reachable Tailnet identity.
Matching values cause no writes or restart. Stale values cause the existing
Forgejo service restart; its inactive oneshot initializer applies the address
before the replacement process starts. The native initializer also runs when
Forgejo starts. Enrollment success and refresh failure are reported separately.
There is no watcher or durable recovery state.

## Access and networking

Soda is cloud-first, not cloud-only.

- On a trusted local network, OpenSSH, Cockpit, and Forgejo are directly
  reachable over the LAN. Tailscale must not block this path.
- In cloud environments, those services are reachable through Tailscale and
  are never exposed to the public Internet.
- Loopback access remains available to the owning local services.

Projects listing and workspace setup do not require a Tailscale identity. Once
Cockpit is reachable through an approved LAN or Tailnet path, its browser UI
uses the hostname from that browser location for SSH guidance rather than
asking the host operation to choose a network identity.

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

The owner registers the first Forgejo account through the normal trusted LAN
or Tailnet before teammates sign in. Native first-user signup grants Forgejo
administration. Use independent Forgejo credentials, even with the same username
as the Linux owner. PAM remains active. Later Linux users' first successful PAM
login creates ordinary Forgejo accounts. Linux wheel membership grants no
Forgejo role. The team controls ongoing registration policy; there is no
mandatory registration-closing step or associated restart.

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

A repository is created through bundled Forgejo or the external authoritative
Git host, then added to Projects with its credential-free SSH clone URL.
Projects creates no repository. The project ID and canonical URL are immutable
after addition. An edit changes display information or additional metadata but
does not accept a URL; replacing the URL requires an administrator to remove the
project and all of its local workspaces, then add it again. The authoritative
repository is not removed.

Selecting **Set up for me** prepares one derived workspace account and its
workspace-private outbound Git key, then attempts the clone. If repository
authentication is unavailable, Projects reports the public key for the person
to register through the authoritative Git host and the person retries. The list
reports `workspace_exists` from the derived Linux account's existence, including
while the failed clone remains retryable; it does not translate checkout
completeness into a second status fact. Successful setup leaves a complete clone
under the workspace account's `$HOME/Projects`. Projects accepts no Forgejo
password, registers no workspace key, and retains no credential,
partial-workflow state, job, retry record, or reconciliation record.

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

Soda's administrator-only person removal deletes local workspaces first and the
primary Linux account last. It neither inspects nor deletes a same-named
Forgejo account. Forgejo availability, ownership, and deletion restrictions do
not block Linux-person deletion. Delete Forgejo accounts explicitly inside
Forgejo. Linux preflight checks, partial-failure reporting, and generic
non-cascading Cockpit/Linux deletion remain unchanged.

## Cockpit and privileged operations

Stock Cockpit owns browser authentication, sessions, TLS, account management,
host overview, metrics, services, logs, terminal, storage, and networking.
Soda adds branding, the Projects and Runners pages, and a separate Tailscale page
using native Tailscale interfaces and stock Cockpit privilege elevation.

Soda may retain fixed, one-shot privileged operations only for accepted
catalog, workspace, project-removal, and person-removal transitions
that genuinely require root. They accept only bounded product inputs and expose
no arbitrary command, path, UID, process selector, credential, or general
account/repository API.

Soda ships no separate dashboard, web server, session service, generic daemon,
runtime API, database, RPC contract, control socket, or generic privileged
bridge.

## Development tools

`mise` is the approved owner of development-tool installation, versions, and
project toolchain configuration. Soda ships `mise`; people invoke and configure
it directly in their workspaces. Project configuration is shared through the
project's native repository workflow. Upstream tool managers own their native
cache behavior. Projects exposes no tool selection, installation action, shared
tool storage, status translation, retry, or cleanup lifecycle. Soda owns no
cache format, cache service, downloader, package manager, version manager,
profile system, or toolchain database. Installed dependencies and other mutable
development state remain private to each workspace.

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
accounts, native Tailscale composition, and fixed one-shot
privileged actions, plus the separately accepted local Runners composition
under #47. Runners retains provider-owned registration, workflows, scheduling
and job history.

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
- coordinated Linux and Forgejo account deletion; and
- release CI that rebuilds fallback A and runs VM/Tailscale acceptance.

These are implementation debt, not accepted behavior. Existing tests and old
artifact evidence prove only those historical paths and must be replaced as
their owning issues land. No public release or finished release artifact is
claimed by this record.

## Engineering verification

The following remain engineering questions, not product decisions:

1. Installed interactive-shell welcome behavior and native Tailscale page access.
2. LAN and Tailnet service reachability, including exit-node use on Fedora 44.
3. Native first-owner signup, later PAM account creation under both team
   registration policies, and manual Git-key registration on the shipped Forgejo.
4. Catalog syntax, path, additional fields required by approved UI, and
   concurrency; project identity and canonical-URL immutability are settled.
5. Workspace naming, classification, staging, and process removal.
6. Direct `mise` operation on Fedora 44 with enforcing SELinux on both
   architectures, without a Soda wrapper or parallel tool state.
7. Upstream-native shared cache behavior without Soda-owned cache state.
8. Matching-native ISO/QCOW2 first-boot and volume-growth behavior.
9. Signed acceptance-record schema and Cosign identity verification.
10. Build-once CI artifact handoff and the 30-minute performance target.

If a native hypothesis fails, return the exact constraint for review. Failure
does not authorize a daemon, database, broker, alternate onboarding path,
fallback package manager, retry queue, or reconciliation service.

## Current-release product-level acceptance criteria

The reset is complete when both matching-native architectures demonstrate:

1. Graphical Anaconda creates ISO accounts; normal interactive login shows welcome.
2. Standard cloud-init provisions QCOW2; welcome is mandatory and stateless.
3. LAN exposes SSH, Cockpit, Forgejo, and normal development-server links;
   cloud exposes them only through Tailscale.
4. Linux and `wheel` remain authoritative for people and administrators.
5. Native first-owner signup grants independent Forgejo administration; later
   PAM accounts are ordinary. Cockpit manages personal authorized keys; users
   register workspace Git public keys with their authoritative Git host.
6. Every selected person-project pair receives a separate Linux account, home,
   full clone, dependencies, processes, and mutable state; the interface reports
   Linux account existence honestly while an incomplete clone remains retryable.
7. Public keys are copied once; Tea and gh authenticate manually per workspace;
   no private or CLI credential is copied.
8. Everyone can view and edit display information and additional metadata in
   the shared project list without a closed field schema or membership model;
   project identity and canonical URL remain immutable, and URL replacement is
   the destructive administrator remove-and-re-add path.
9. A person removes only their own workspace; an administrator removes a whole
   project and every local workspace while preserving the canonical Forgejo
   repository.
10. Person deletion removes local workspaces then the Linux account, preserving
    the Forgejo account and reporting partial failures.
11. Normal development-server links work over LAN and Tailscale without Soda
    port or process tracking.
12. People invoke and configure `mise` directly in workspaces without a Soda
    tool selector, installer, shared storage model, or lifecycle.
13. Native manual update and fallback preserve authoritative mutable state.
14. The signed pre-release record covers both architectures and exact source.
15. Release CI builds each B once, structurally verifies and signs the exact
    outputs, and publishes them unchanged without VM acceptance.
16. No forbidden control-plane, credential, toolchain, updater, or recovery
    machinery remains.

## Issue ownership and order

The current dependency order is:

1. [#42: graphical one-ISO proof](https://github.com/LevitateOS/soda-os/issues/42), and [#45: console-onboarded QCOW2](https://github.com/LevitateOS/soda-os/issues/45).
2. [#15: LAN and Tailnet access](https://github.com/LevitateOS/soda-os/issues/15).
3. [#44: Forgejo SSH keys and manual workspace CLI authentication](https://github.com/LevitateOS/soda-os/issues/44) and [#33: ordered person deletion](https://github.com/LevitateOS/soda-os/issues/33).
4. [#35: shared projects and private workspaces](https://github.com/LevitateOS/soda-os/issues/35), [#37: native Git-host ownership](https://github.com/LevitateOS/soda-os/issues/37), and [#32: focused Cockpit workflows](https://github.com/LevitateOS/soda-os/issues/32).
5. [#24: mise-owned development tools](https://github.com/LevitateOS/soda-os/issues/24).
6. [#25: signed matching-native acceptance evidence](https://github.com/LevitateOS/soda-os/issues/25), followed by [#43: Go acceptance orchestration](https://github.com/LevitateOS/soda-os/issues/43).
7. [#22: reviewed dependency locks](https://github.com/LevitateOS/soda-os/issues/22), [#29: release identity](https://github.com/LevitateOS/soda-os/issues/29), and [#48: build-once signed release](https://github.com/LevitateOS/soda-os/issues/48).

[Issue #46](https://github.com/LevitateOS/soda-os/issues/46) is a separate WSL
feasibility investigation. [Issue #47](https://github.com/LevitateOS/soda-os/issues/47)
is a separate local-runner product and must not become Soda workflow authority.

See [native onboarding acceptance](native-onboarding.md#installed-acceptance)
for cloud-init late enrollment, conditional refresh, reboot, LAN-only, and
service-ordering checks required on both matching architectures.
