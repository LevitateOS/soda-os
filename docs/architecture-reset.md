# Soda OS architectural reset

**Status:** Accepted product architecture and governing ownership constraints.
Native x86-64 evidence has proved the current account-preserving fallback;
matching-native AArch64 repetition remains required before release completion.

**Recorded:** 2026-08-31

**Initial implementation snapshot reviewed:** `7f2c60b`

**Current implementation checkpoint documented:** `2d2a359`

**Initial architecture record:** `e992e22`

The [base principles](principles.md) state the product purpose and ownership
philosophy in human terms. This record defines the exact accepted architecture,
governing constraints, and issue boundaries.

At the current checkpoint, the protected stock-Anaconda/Kickstart, native
workspace, direct-SSH, stock-Cockpit, immutable-toolset, telemetry-deletion,
and runtime-updater-deletion slices are present. One fresh native x86-64
installation has exercised those slices together, and native x86-64
A→B→A→B image selection has preserved current mutable state. These are
implementation and test facts, not additional product authority. The current
protected installer and fallback still require matching-native AArch64
installed-system evidence. Later-primary Forgejo PAM authentication remains
stopped at an explicit password-verifier privilege decision. The custom
publication client, final acceptance-runner migration, and residual Health-only
gRPC shell remain later deletion or replacement work.

## Product contract

Soda OS is an opinionated Fedora bootc appliance for private remote
development. A lightweight client connects over Tailscale and OpenSSH to a more
powerful Soda machine. Codex, Claude, Zed, VS Code, shells, compilers, tests,
and other development workloads execute on that machine.

Soda provides a coherent path from installation to collaborative development:

1. Install the architecture-matched Soda image.
2. Establish the first primary Linux administrator and enroll the machine in
   the owner's Tailnet.
3. Manage primary human accounts with Linux and stock Cockpit.
4. Add an existing canonical Git URL to the appliance project catalog, or
   create a native empty repository in the initiating human's bundled Forgejo
   namespace.
5. Select **Set up for me** to obtain a derived Linux workspace account and a
   complete clone at `$HOME/Projects/<repository>`.
6. Connect directly to that workspace account through ordinary OpenSSH.
7. Collaborate through the canonical Git host's native authorization, branches,
   review, issues, and releases.

Soda owns the installable composition, conventions, focused presentation,
minimal catalog, irreducible human-project association, and narrow synchronous
transitions required to make that workflow coherent. Linux, OpenSSH, Git,
Forgejo or the external canonical host, Tailscale, Cockpit, and bootc continue
to own the facts and mechanisms they naturally implement.

Soda does not retain a general daemon, database, RPC API, workflow engine,
credential broker, copied authority, job state, or reconciliation loop.

AArch64 and x86-64 are equal sibling product targets. Every released feature
and product-level acceptance scenario supports both unless an architecture-
specific limitation is explicitly documented and reviewed.

As a current release-integrity policy, architecture-specific input preparation,
dependency resolution, builds, artifact generation, inspection, signing,
publication, installation, and validation execute on matching native hardware.
That policy may change only through explicit review; it is not implied by
product parity alone.

## Why the reset is necessary

The pre-reset implementation repeatedly turned a small discrepancy with an
upstream system into copied state and then into a Soda control plane:

1. Soda added a helper around an upstream behavior.
2. The helper persisted a parallel representation.
3. The parallel representation became authoritative.
4. Drift then required preflights, jobs, retries, compensation, and
   reconciliation.
5. A custom web application, daemon, database, protobuf, and gRPC surface grew
   around that state.
6. Tests preserved the machinery as though it were a product requirement.

This occurred across Linux people and roles, project membership and worktrees,
Forgejo projections, host telemetry, runtime toolchains, and translated bootc
updates.

The reset rejects two symmetric errors:

- A discrepancy does not transfer ownership of an upstream subsystem to Soda.
- The presence of upstream primitives does not justify deleting Soda's accepted
  catalog, discovery, and workspace-onboarding product.

Custom code survives only at the smallest boundary required by an accepted
Soda outcome.

## Authoritative ownership

| Responsibility | Authoritative owner |
| --- | --- |
| Private reachability and Tailnet policy | Tailscale and Tailnet ACLs |
| Remote session transport | OpenSSH |
| Primary human and derived workspace accounts | Linux accounts and UIDs |
| Passwords, groups, homes, permissions, and processes | Linux |
| Linux administrator status | `wheel` membership |
| Repository lifecycle, access, and collaboration | Canonical Git host |
| Checkout contents and repository state | Git and the workspace account |
| Host services and state | Linux, systemd, PAM, polkit, and standard interfaces |
| Host administration interface | Stock Cockpit |
| Project discovery | Soda's minimal declarative catalog |
| Workspace composition and destructive local lifecycle | Soda's narrow synchronous operation |
| Browser product composition | Soda branding and one Cockpit Projects package |
| OS deployment state and native operations | bootc |
| Broad reviewed development tool collection | Soda image composition |
| Additional tools and versions | Users, repositories, and ecosystem tooling |
| Appliance image and conventions | Soda OS |

Transient presentation models are allowed. A persistent second authoritative
copy is not. Each retained adapter must identify its exact accepted outcome,
the upstream owner, the precise remaining gap, and the irreducible Soda fact—if
any—that must survive.

## Primary human and derived workspace accounts

Each person has one primary Linux account. It represents that human for stock
Cockpit, Forgejo PAM, catalog access, and human administrator status. A primary
account in `wheel` is an administrator without a Soda role or registration
record.

For each primary-human and catalog-project pair selected through **Set up for
me**, Soda creates one derived Linux workspace account. That account owns:

- a real private home;
- a complete independently writable clone at
  `$HOME/Projects/<repository>`;
- user-local dependencies and caches;
- project-local data; and
- development processes.

A derived account is not a human, Forgejo user, administrator, project-shared
login, service account, or repository identity. Alice and Bob receive separate
UIDs, homes, clones, Git metadata, credentials, caches, and processes for the
same project.

Linux remains authoritative for every account and home. Soda owns only this
association:

```text
primary human account + immutable catalog project id -> derived workspace account
```

The primary-human/workspace distinction must be represented through ordinary
Linux-native account state or a deterministic account convention with concrete
authorization meaning. It must not require a Soda person database. The current
implementation's group and account marker are verified implementation choices,
not permanent product requirements. Any mechanism must allow the Projects
package and helper to recognize primary humans, allow Forgejo PAM to reject
workspace accounts, and validate derived-account operations.

Primary Linux usernames are stable identifiers for the initial Soda release.
The supported Soda workflow does not rename a primary username while derived
workspaces exist. An out-of-band rename is unsupported and Soda does not repair,
rename, or reconcile affected workspace accounts. The administrator must first
remove or manually repair affected workspaces.

## Direct workspace OpenSSH

Humans connect directly to derived workspace usernames through ordinary
OpenSSH. Interactive shells, direct commands, automation, SCP, and SFTP run as
the authenticated workspace UID in its real home. Soda installs no forced
command, selector, account-transition gateway, synthetic `HOME`, or custom
SFTP dispatcher.

Before setup mutates anything, the primary account must have at least one
public key in the supported standard source:

```text
$HOME/.ssh/authorized_keys
```

If no key exists there, **Set up for me** fails visibly and instructs the human
to install one before retrying. Soda does not create a workspace password,
temporary login, key registry, or alternate authentication path.

Setup copies those currently authorized public keys once into the new
workspace account. Later changes are not synchronized. Soda does not attempt to
enumerate external `AuthorizedKeysCommand` sources unless a future explicit
requirement adds one.

Inbound Soda SSH and outbound Git-host authentication are separate. Copying a
public key into `authorized_keys` does not give the workspace a private Git-host
key. After setup, each human selects ordinary Git authentication such as
per-connection agent forwarding or privately configured workspace credentials.
Soda selects neither method, enables no global forwarding policy, and stores no
credential.

## Project catalog

The catalog is the minimal appliance-wide list of projects offered as Soda
workspaces. Each entry contains exactly:

```text
id
display_name
canonical_url
```

The fields have these rules:

- `id` is immutable and drives stable association and derived-account naming.
- `display_name` is mutable presentation text.
- `canonical_url` is mutable, affects future setup only, and contains no
  password, access token, or other embedded secret. A transport username such
  as `git@host` is not a secret.

The catalog contains no creator, owner, collaborators, membership, permissions,
capabilities, credentials, workspace instances, clone status, process state,
ports, containers, timestamps, job state, retry state, or deletion state.

Every primary human may discover, add, edit, or destructively remove catalog
entries. Soda adds no creator ownership, approval, handoff, or project
membership policy. Adding or editing an entry affects future discovery and
setup only; it never renames existing accounts, moves homes, or rewrites Git
remotes.

The stable project ID remains unchanged when the display name or URL changes.
The exact declarative syntax and persistent path remain implementation
verification in #35; they do not justify a database.

## Workspace setup

Successful **Set up for me** leaves:

- one derived Linux account and private home;
- the primary account's supported public keys copied once; and
- a complete clone owned by that account at
  `$HOME/Projects/<repository>`.

Authentication uses a native user-authenticated Git or repository-host
operation. Soda does not retain credentials. Failure is synchronous and creates
no durable provisioning state, partial derived account, job, retry record, or
reconciliation record.

The architecture intentionally does not claim that Cockpit supplies a
particular Git prompt, that one command completes every host flow, or that every
external provider is supported. The current implementation keeps Git
unprivileged and publishes a completed checkout through the fixed local
workspace boundary; its exact credential transport and staging path remain
implementation choices.

When no URL is supplied, Soda creates a native empty repository in the
initiating human's Forgejo namespace. Soda creates no README, artificial first
commit, environment file, or local branch merely to make creation work. A
credential-free canonical URL is then stored in the catalog.

The current implementation uses the pinned Forgejo user's native repository
endpoint and verifies that the repository is empty. That exact endpoint is an
implementation choice. The operation must not use a Soda-global administrator
token, credential broker, repository projection, or push-to-create assumption.

## Destructive local lifecycle

Project removal is a synchronous, safely repeatable operation:

1. Exclude a concurrent setup for the target project while removal runs.
2. Terminate sessions and processes for every derived workspace account for the
   project.
3. Delete those accounts, homes, complete clones, and explicitly Soda-created
   workspace paths.
4. Remove the catalog entry last.

If deletion fails, the catalog entry remains and the operation reports the
native failure. A later invocation inspects authoritative Linux state and can
continue. There is no transaction log, rollback engine, queue, background
worker, cleanup daemon, or reconciliation loop.

Removal does not delete the canonical Forgejo or external repository, change
Git-host authorization, scan arbitrary filesystem locations, archive data, or
transfer ownership. The trusted team preserves anything valuable before
invoking removal.

Supported primary-human deletion is an administrator-only action in the Soda
Projects package using the same narrow synchronous helper:

1. Identify the human's derived workspace accounts from the deterministic
   Linux/catalog association.
2. Terminate their sessions and processes.
3. Delete those accounts, homes, clones, and explicitly Soda-created paths.
4. Delete the primary Linux account and home last.

Failure leaves the primary account present and reports the native error. The
operation never deletes the corresponding Forgejo account, Forgejo repository,
external account, or external repository.

Generic stock-Cockpit account deletion or direct `userdel` changes only the
explicitly selected Linux account. It is out-of-band and non-cascading. Soda
does not watch for or reconcile such deletion. Normal primary-account creation,
password changes, and `wheel` administration remain stock Cockpit/Linux
behavior.

## Forgejo and canonical Git-host boundaries

Bundled Forgejo owns repository authorization, collaboration, review, issues,
releases, application users, roles, sessions, tokens, and keys. External hosts
own the equivalent facts for externally hosted repositories. Soda stores none
of those facts in the catalog and does not probe or cache provider capabilities.

Installation creates the only proactive Forgejo user: a same-named local site
administrator for the first primary Linux administrator, through Forgejo's
native administrative interface. The selected initial password may be handed
off only through a bounded installer path that leaves no Soda credential state.
If that exact path is unavailable, the password outcome returns to review
rather than justifying credential machinery.

Later primary human accounts authenticate through the shipped Forgejo PAM
source. Forgejo creates its native user on first successful login. Derived
workspace accounts must be rejected using the Linux-native classification.
Later `wheel` changes do not affect Forgejo roles; Forgejo administrators manage
Forgejo roles in Forgejo.

The initial Linux and Forgejo accounts become independent after installation.
Linux disablement blocks later PAM authentication but does not claim to revoke
existing Forgejo sessions, tokens, SSH keys, or repository authorization.
Forgejo revocation and deletion remain Forgejo-native operations.

Soda may configure the Forgejo PAM source with the fixed packaging convention
`localhost` for initial email addresses. This is optional upstream
configuration, not an installer input or Soda-owned email model.

## Cockpit and the narrow privilege boundary

Soda uses stock Fedora Cockpit for browser authentication, TLS, sessions, host
administration, and standard privilege escalation. Soda adds supported branding
and exactly one focused Projects package.

The Projects package provides:

- catalog discovery and exact catalog-entry mutation;
- native empty Forgejo repository creation;
- **Set up for me**;
- destructive project removal; and
- administrator-only cascading primary-human deletion.

It reuses the authenticated Cockpit session and may invoke one narrowly
authorized synchronous operation for only the root-required parts of those
accepted transitions. The retained boundary may mutate the catalog and native
Linux state, install public keys, place a successfully authenticated clone, and
perform the defined destructive ordering.

It exposes no generic `useradd`, `userdel`, arbitrary deletion, command
execution, Forgejo administration, container management, or account-management
API. It retains no credentials, operation status, retry lifecycle, or private
workflow state.

Soda ships no separate dashboard server, TLS stack, authentication helper,
session service, long-running daemon, database, RPC client, job monitor,
credential store, or generic privileged service.

## Installation, mutable state, and OS fallback

The first supported installation path requires four values:

```text
administrator username
administrator password
administrator SSH public key
one-use Tailscale auth key
```

Anaconda and Kickstart create the primary Linux administrator, add it to
`wheel`, set its Linux password, and install its SSH public key. Installation
creates the same-named Forgejo-local site administrator through Forgejo's native
interface.

A small first-boot systemd oneshot gives the one-use credential to Tailscale
through a temporary file, removes that file after the attempt, and disables
itself. After installation, no Soda bootstrap account, credential store, API,
public onboarding endpoint, durable workflow, retry state, or long-running
bootstrap service remains.

Normal bootc image replacement must preserve primary and derived accounts,
password and group state, homes, workspace clones, the catalog, Forgejo mutable
state, Tailscale identity, and other authoritative machine-specific state. That
state is not owned by the replaceable image layer.

Supported fallback to an earlier Soda image must preserve the current primary
accounts, derived accounts, passwords, groups, and administrator membership.
Direct `bootc rollback` is not a supported fallback path. Upstream documents
that direct rollback restores the previous deployment's `/etc`, while creating
a new deployment through native switch/upgrade preserves current `/etc` and
`/var`:
[bootc rollback](https://bootc.dev/bootc/man/bootc-rollback.8.html) and
[bootc upgrades](https://bootc.dev/bootc/upgrades.html).

The current native x86-64 implementation has proved the accepted invariant by
selecting an earlier exact Soda reference through `bootc switch
--download-only`, `bootc switch --from-downloaded`, and a controlled reboot,
then recovering forward the same way. The command sequence remains an
implementation result rather than a new Soda subsystem; matching-native
AArch64 evidence is still required. If a supported architecture cannot satisfy
the invariant through an upstream-native path, the product fallback decision
returns to architectural review. Soda must not respond with an account
database or reconciliation service.

Linux administrators control update checks, staging, activation, and supported
fallback through native bootc operations. The automatic update timer is
disabled. Soda ships no runtime update daemon, release-discovery client,
translated deployment state, API, CLI wrapper, or custom update page.

## Development tools and conflict isolation

Soda ships a broad reviewed collection of language runtimes and developer tools
as immutable image composition on both architectures. The approved command
contract is recorded in `distro/toolset-commands.txt`; exact architecture-owned
package closures remain implementation evidence. Soda does not resolve or
install latest toolchains at runtime, persist toolchain profiles or readiness
state, or reconcile versions. Users and projects may use additional ecosystem
tooling in their own homes and repositories.

Derived UIDs and homes isolate ordinary development conflicts in files,
dependencies, caches, project-local data, and process ownership. They do not
create separate port, network, mount, kernel, or virtual-machine namespaces.
Projects choose non-conflicting host ports and may use rootless Podman or other
ordinary tools when useful. Podman is optional software, not Soda's isolation
mechanism or control plane.

Soda assumes a trusted team on a private Tailnet. Workspace peers do not
normally write one another's homes or credentials. Root, `wheel`, and other
root-equivalent principals remain trusted Linux administrators. Soda does not
promise hostile-tenant isolation.

## Compact governing invariants

- One primary Linux account represents each human.
- One derived Linux account represents each selected human-project pair.
- Linux-native state distinguishes primary and workspace accounts.
- Primary usernames and catalog project IDs are stable identifiers.
- Each catalog entry contains exactly `id`, `display_name`, and a credential-
  free `canonical_url`.
- Every primary human may add, edit, or destructively remove catalogued
  projects.
- Successful setup leaves a complete clone at
  `$HOME/Projects/<repository>` and stores no credential or workflow state.
- Setup requires a public key in the primary account's standard
  `~/.ssh/authorized_keys` before mutation.
- No-URL projects begin as native empty Forgejo repositories.
- Humans connect directly to derived accounts with ordinary OpenSSH.
- Project and human removal delete only defined Linux-local state and preserve
  canonical repositories.
- Supported human deletion is Soda-aware and administrator-only; generic Linux
  deletion is non-cascading and out-of-band.
- Catalog and account lifecycle operations are synchronous and safely
  re-runnable from authoritative state.
- Stock Cockpit plus one Projects package is the browser surface.
- One narrow synchronous privilege boundary survives; no general control plane
  does.
- Projects manage host ports; Podman remains optional.
- The image contains a broad reviewed development tool collection without a
  runtime Soda toolchain manager.
- Native bootc owns deployments, but supported fallback must preserve current
  Linux account state.
- AArch64 and x86-64 receive equal product behavior and matching-native release
  verification.

## Explicit non-goals

- Rebuilding Linux, OpenSSH, Git, Forgejo, Tailscale, Cockpit, bootc, or a
  toolchain manager behind Soda APIs.
- A Soda person database, role mirror, project membership model, provider
  capability model, token broker, or credential vault.
- Project Unix groups, shared project logins, shared worktrees, shared bare
  repositories, project service accounts, or forced-command SSH gateways.
- Soda-managed ports, network namespaces, shared Podman state, containers, or
  VM isolation.
- Durable jobs, retries, compensation, rollback engines, cleanup watchers,
  reconciliation, or anti-drift machinery.
- Public-Internet service exposure, enterprise identity, hostile-project threat
  models, or generic orchestration.
- Compatibility machinery for pre-release Soda schemas or APIs whose product
  behavior has been removed.
- Deleting accepted Soda catalog or workspace outcomes merely because upstream
  tools expose their individual primitives.

## Implementation-specific boundaries

The following are engineering verification, not new product domains. Several
have a proved current implementation, but remain subject to exact-version and
matching-native re-verification rather than becoming permanent mechanisms:

1. Exact declarative catalog syntax and persistent path.
2. Stable project-ID and derived-username encoding.
3. Linux-native primary/workspace classification and Forgejo PAM exclusion.
4. Narrow helper authorization and synchronous concurrency exclusion.
5. Exact user-authenticated Forgejo operation for a truly empty repository.
6. Supported private-repository authentication paths through Cockpit/Git.
7. Safe placement of the authenticated clone under the derived UID.
8. Initial installer-to-Forgejo password handoff.
9. Exact broad development-tool packages on both architectures.
10. Exact installed Soda image reference tracked by bootc.
11. Account preservation across update and supported fallback to an earlier
    image.
12. Tailnet-only service exposure.
13. Process termination and native account-deletion behavior.

If the smallest native implementation hypothesis fails, the owning issue must
return to the accepted product outcome. Failure does not authorize a daemon,
database, generic API, token service, retry queue, provider abstraction, or
reconciliation process.

## Product-level acceptance criteria

The reset is complete only when the resulting product demonstrates:

1. Installation creates the first primary Linux administrator, same-named
   Forgejo-local administrator, SSH access, and one-use Tailnet enrollment with
   no surviving Soda bootstrap state.
2. OpenSSH, stock Cockpit, Forgejo, and product interfaces are reachable through
   the Tailnet and are not directly exposed to the public Internet.
3. Primary and derived accounts are distinguishable through Linux-native state;
   derived accounts cannot become Forgejo PAM users or administrators.
4. The catalog persists only `id`, `display_name`, and credential-free
   `canonical_url` values.
5. Adding or editing a catalog entry does not rename or reconcile existing
   accounts, homes, clones, or remotes.
6. Setup without a primary `~/.ssh/authorized_keys` key fails before creating an
   account or clone.
7. A no-URL project becomes a native empty repository in the initiating human's
   Forgejo namespace without a Soda-global credential or generated first
   commit.
8. Alice and Bob can each set up the same project and receive separate derived
   UIDs, homes, complete writable clones, dependencies, local data, and
   processes.
9. Setup authentication is native and user-authenticated; Soda retains no
   credential, partial workspace, job, or retry state after failure.
10. Direct OpenSSH, commands, automation, SCP, and SFTP operate as the derived
    workspace UID without a gateway or selector.
11. Project removal follows the defined order, removes every Soda-managed local
    workspace, leaves the catalog entry on partial failure, and never deletes
    the canonical repository.
12. Administrator-only supported human deletion removes derived local state and
    deletes the primary account last without invoking Forgejo or an external
    provider.
13. Generic Cockpit or command-line account deletion is documented and observed
    as non-cascading; Soda adds no watcher or reconciliation behavior.
14. Projects manage their own host ports and may use optional rootless Podman
    without Soda container or network state.
15. The broad reviewed image toolset is available on both architectures without
    runtime Soda toolchain state.
16. Native bootc update operations preserve authoritative mutable state.
17. Supported fallback to an earlier Soda image preserves current accounts,
    passwords, groups, administrator membership, homes, workspaces, catalog,
    Forgejo state, and Tailscale identity. Direct `bootc rollback` is not claimed
    unless separately proven.
18. The same product scenarios pass on x86-64 and AArch64, and architecture-
    specific release work executes on matching native hardware.
19. No obsolete Soda daemon, SQLite authority, protobuf, gRPC, control socket,
    workflow engine, credential service, runtime updater, or runtime toolchain
    manager remains.

## Issue ownership and implementation order

The accepted dependency order is:

1. [#40: installer: provision the first administrator and Tailnet enrollment](https://github.com/LevitateOS/soda-os/issues/40), with [#41](https://github.com/LevitateOS/soda-os/issues/41) retaining Anaconda unless the bounded next-generation comparison triggers replacement
2. [#33: architecture(identity): make Linux identity and administrator status authoritative](https://github.com/LevitateOS/soda-os/issues/33) together with [#38: architecture(updates): verify account-preserving native bootc fallback](https://github.com/LevitateOS/soda-os/issues/38)
3. [#37: architecture(git-host): preserve native repository-host ownership](https://github.com/LevitateOS/soda-os/issues/37)
4. [#35: architecture(workspaces): provide catalogued derived workspace accounts](https://github.com/LevitateOS/soda-os/issues/35)
5. [#36: architecture(ssh): use ordinary OpenSSH for direct workspace accounts](https://github.com/LevitateOS/soda-os/issues/36)
6. [#32: architecture(cockpit): retain stock Cockpit with branding and Projects page](https://github.com/LevitateOS/soda-os/issues/32), followed by [#34: architecture(telemetry): remove redundant host telemetry machinery](https://github.com/LevitateOS/soda-os/issues/34)
7. [#24: architecture(toolchains): ship a broad curated immutable toolset](https://github.com/LevitateOS/soda-os/issues/24)
8. [#23: release: replace the custom GitHub publication client](https://github.com/LevitateOS/soda-os/issues/23) after #38 determines which release metadata remains
9. [#25: test(acceptance): collapse raw-QEMU test orchestration](https://github.com/LevitateOS/soda-os/issues/25) around the resulting outcomes
10. [#39: architecture(control-plane): remove residual runtime infrastructure](https://github.com/LevitateOS/soda-os/issues/39) as the capstone

At the current implementation checkpoint, #33, #37's initial-admin repository
path, #35, #36, #32, #34, and the source-side #24 replacement are locally
implemented. The protected #40 path, #24 installed toolset, and #38 fallback
have native x86-64 evidence but still need current matching-native AArch64
repetition. The later-primary portion of #37 is stopped at its explicit
Forgejo password-verifier privilege decision. Issues #23, #25, and #39 remain
substantive later milestones. These status facts do not let issue text redefine
the outcomes above.

Issue #33 owns stable primary identity, Linux-native account classification,
and supported cascading human deletion. Issue #38 owns account behavior across
native update and fallback. Issue #35 owns catalog fields and persistence,
derived naming, setup outcome, checkout placement, and project-removal order.
Issue #36 owns direct SSH, the supported key source, and missing-key failure.
Issue #37 owns native empty Forgejo repository creation and Git-host authority.
Issue #32 owns Projects-page presentation and administrator-only human deletion.
Issue #24 preserves the broad image toolset while removing runtime management.
Issue #39 verifies that only the narrow synchronous operation survives the old
generic runtime shell.

Each issue must identify the accepted outcome, authoritative owner, irreducible
Soda fact, smallest retained boundary, deletion ownership, native failure
behavior, and product-level acceptance evidence. Engineering verification must
remain in its owning issue rather than becoming a new architecture domain.
