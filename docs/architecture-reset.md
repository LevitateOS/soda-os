# Soda OS architectural reset

**Status:** Accepted product direction and governing architectural constraints;
bounded workspace, packaging, and installation verification remains assigned
to the linked issues.

**Recorded:** 2026-08-31

**Implementation snapshot reviewed:** `7f2c60b`

**Initial architecture record:** `e992e22`

## Decision

Soda OS needs an architectural reset.

The product is an opinionated Fedora bootc appliance for private remote
development. A lightweight client connects over Tailscale and OpenSSH to a more
powerful Soda machine. Standard clients such as Codex, Claude, Zed, VS Code,
and OpenSSH run their remote workloads there. The canonical Git host provides
repository collaboration, bootc owns deployments, Linux owns people and host
permissions, and Cockpit provides the host administration interface.

Soda owns the branded, installable composition, a curated image-resident
development toolset, a minimal appliance-wide project catalog, and the narrow
human-project workspace workflow that makes that way of working reliable. That
workflow uses ordinary Linux accounts, OpenSSH, Git, Forgejo, and Cockpit; it
does not make Soda authoritative for repository access, collaboration,
credentials, container state, or generic Linux administration.

For each catalogued project a human chooses to set up, Soda creates one derived
Linux workspace account. That account owns a private home, complete Git clone,
user-local dependencies, project-local data, and development processes. Soda
retains no general project control plane, parallel identity registry, daemon,
database, RPC contract, job engine, credential store, or reconciliation loop.

The reset changes the burden of proof: an existing system remains authoritative
unless a concrete Soda product fact cannot be represented there. A discrepancy
justifies bridging that discrepancy. It does not transfer ownership of the
surrounding subsystem to Soda.

## Product contract

Soda OS standardizes this way of working:

1. Install an architecture-matched Fedora bootc development appliance on a
   powerful remote machine.
2. Establish the first Linux administrator and join the machine to a private
   Tailnet without exposing Soda services directly to the public Internet.
3. Administer primary human Linux accounts through stock Cockpit and normal
   Linux tools.
4. Add an existing repository URL to Soda's minimal project catalog, or create
   a repository in the initiating human's bundled Forgejo namespace when no URL
   is supplied.
5. Let any human choose **Set up for me** for a catalogued project. One ordinary
   interactive Git operation authenticates as that human without Soda retaining
   credentials, and a narrow synchronous privileged operation creates the
   derived workspace account and complete clone.
6. Connect from a lightweight client directly through ordinary OpenSSH to that
   derived workspace account with an SSH-capable development tool.
7. Use the canonical Git host for authorization, branch exchange, review,
   issues, and releases where that host provides them.

The client retains the human interface. Compilation, indexing, tests, agents,
and development processes run as the derived workspace UID on the Soda
machine. During setup, Soda copies the primary human account's currently
authorized public SSH keys once into the new workspace account. Later key
changes are not synchronized. Inbound workspace login and outbound Git-host
authentication remain distinct: after setup, each person configures ordinary
Git, OpenSSH, agent forwarding, HTTPS credentials, or ecosystem helpers for the
workspace as they choose. Soda neither enables agent forwarding globally nor
stores, brokers, or synchronizes outbound Git credentials.

OpenSSH is the primary remote-development transport. Stock Cockpit provides
normal Linux administration and hosts one focused Soda Projects page. Tailscale
provides private reachability but does not replace Linux login, process
attribution, or Git-host authorization.

AArch64 and x86-64 are equal sibling product targets: every released feature
and product-level acceptance scenario must support both unless an
architecture-specific limitation is explicitly documented and reviewed.

As a current release-integrity policy, architecture-specific input preparation,
dependency resolution, builds, artifact generation, inspection, signing,
publication, installation, and validation execute on matching native hardware.
This policy may be changed only through an explicit review; it is not implied
by product parity alone.

### Initial installation provisioning

A fresh Soda machine is provisioned by Anaconda and Kickstart with its first
ordinary Linux administrator, that administrator's password and SSH public key,
and a one-use Tailscale enrollment credential. A minimal first-boot systemd
oneshot invokes Tailscale with the credential from a temporary file, removes
that file after the enrollment attempt, and disables itself.

The first supported path requires only an administrator username,
administrator password, administrator SSH public key, and one-use Tailscale
auth key. Soda owns the installer configuration, the small first-boot Tailscale
invocation, and acceptance evidence. Linux owns the account and password,
OpenSSH owns the authorized key and remote session, Cockpit authenticates the
ordinary Linux account through PAM, and Tailscale owns node enrollment and
private reachability.

After installation, no Soda bootstrap account, credential store, API, public
onboarding endpoint, durable workflow state, retry or reconciliation machinery,
or long-running bootstrap service remains. If initial Tailscale enrollment
fails, the first release relies on local recovery or corrected inputs and
reinstallation rather than a Soda recovery subsystem.

## Why the current architecture violates the contract

The repository exhibits a repeatable snowball pattern. This describes the
architectural mechanism visible at the reviewed revision; it does not assign a
single intent or chronology to every change.

1. An existing Linux or upstream behavior did not exactly match one Soda
   workflow.
2. Soda added a custom helper for the discrepancy.
3. The helper needed metadata and Soda persisted a parallel representation.
4. The parallel representation was declared authoritative.
5. Linux, Git, Forgejo, or bootc could now disagree with Soda state.
6. The disagreement required preflights, compensation, rollback, retry, and
   startup reconciliation.
7. The browser needed access to the new state, producing a daemon, protobuf and
   gRPC contracts, client adapters, and presentation models.
8. Tests preserved the internal machinery as if it were an independent product
   requirement.

This is visible across the pre-reset implementation:

- Soda stores people and application roles while Linux already owns accounts,
  UIDs, passwords, groups, and administrator status.
- Soda stores projects, memberships, and worktrees while the filesystem, Git,
  Unix permissions, and repository hosts already persist most of those facts.
- `soda-cockpit` is a standalone web application that owns TLS, PAM bridging,
  sessions, navigation, and host pages instead of being a Cockpit package.
- Soda samples generic Linux host telemetry already available through Cockpit,
  systemd, D-Bus, and ordinary Linux interfaces.
- Soda implements release discovery, downloading, checksums, archive
  extraction, and installation for several language ecosystems.
- Soda translates and validates a parallel representation of bootc update
  state.
- Soda projects people, keys, repositories, and collaborators into Forgejo and
  stores remote mapping records, creating reconciliation work.

The problem is not that Go invokes standard commands or that the code uses a
database, daemon, RPC, validation, retries, adapters, or presentation models.
The problem is using those mechanisms to support a copied authority.

## Ownership principles

### Existing systems remain authoritative

| Responsibility | Authoritative owner |
| --- | --- |
| Private network reachability and Tailnet policy | Tailscale and Tailnet ACLs |
| Remote session transport | OpenSSH |
| Primary human and derived workspace identities | Linux UIDs and accounts |
| Administrator status | Linux administrator membership, currently `wheel` |
| Repository creation, lifecycle, access, and collaboration | The canonical Git host |
| Workspace checkout and repository state | Git and the derived workspace account |
| Host state and service control | Linux kernel, systemd, PAM, polkit, and standard system interfaces |
| Host administration interface | Stock Cockpit |
| Project discovery and workspace composition | Soda's minimal catalog and synchronous workspace operation |
| Cockpit product composition | Soda branding and one focused Projects page |
| Local repository state and history | Git |
| OS update state and administrator-controlled activation | bootc |
| Curated baseline language runtimes and developer tools | Soda image package composition |
| Additional user or repository toolchains | Users, repositories, and ordinary ecosystem tooling |
| Appliance image and conventions | Soda OS |

Only an explicitly retained bounded composition, such as initial installation,
may invoke more than one owner. It must not persist a second authoritative copy
merely to make queries or UI rendering convenient. Transient aggregation,
caches with no authority, and presentation models remain acceptable when a
retained user-facing behavior requires them.

Soda services are reached through the Tailnet unless an explicit future
requirement says otherwise. OpenSSH, Cockpit, Forgejo, and product-specific
interfaces are not directly exposed to the public Internet.

### The catalog is not repository authority

A Soda project is a minimal appliance-wide catalog entry identifying a stable
project and its canonical Git URL. That association is an irreducible
Soda-owned fact because neither Linux nor an arbitrary Git host owns the list of
repositories offered by this appliance as development workspaces.

The catalog is not a repository, membership, permission, capability, clone, or
runtime database. It stores no collaborator list, Forgejo role, provider
account, Git credential, clone status, process state, port assignment,
container state, job, retry, or reconciliation record. Display names and URLs
may change while the stable project identifier remains fixed. Adding or editing
a catalog entry affects future discovery and setup only; it never renames
existing workspace accounts, moves homes, or rewrites Git remotes. Destructive
project removal has the separate immediate local-deletion behavior below.

Every primary human user may add, edit, or destructively remove catalogued
projects. Soda adds no creator ownership, approval, handoff, or policy model.
Repository authorization remains independently owned by the canonical Git
host.

For a Forgejo-hosted repository, Forgejo owns creation, rename, deletion,
authorization, users and keys in its application domain, teams or repository
collaborators, review, issues, and releases. Ordinary users and Forgejo
administrators use Forgejo's native policy and interfaces directly.

For a repository hosted by GitHub, GitLab, another Forgejo instance, or another
external Git service, that service owns the same responsibilities. Users manage
access there and run ordinary `git clone`, `git fetch`, and `git push` with
their own credentials. Git reports those command results; Soda does not probe,
classify, cache, or present external capabilities.

When a human creates a catalogued project without supplying a URL, one ordinary
authenticated Git operation creates the repository in that human's native
Forgejo namespace, using Forgejo's supported repository-creation behavior.
Soda retains only the resulting canonical URL in the catalog. Importing or
mirroring an external repository into bundled Forgejo remains a Forgejo-native
operation.

### Local lifecycle actions are synchronous and destructive

Removing a project from the catalog synchronously terminates its derived
workspace sessions and processes, removes every derived workspace account for
that project, and deletes their homes, complete clones, and other explicitly
Soda-created workspace paths. It does not delete the canonical Git repository,
change Git-host authorization, scan arbitrary mounted filesystems for files a
workspace UID may have created elsewhere, archive data, transfer ownership, or
provide rollback. The trusted team coordinates before invoking removal.

Deleting a primary human Linux account also removes that human's derived
workspace accounts, homes, complete clones, and explicitly Soda-created local
paths. It does not delete or purge the corresponding Forgejo account, a
Forgejo-owned repository, an external-provider account, or an external
repository. Those remain separate native actions in their authoritative
systems.

These retained destructive operations expose native failure synchronously. A
failure does not create a queue, retry record, compensation transaction,
background worker, or reconciliation loop.

### The reset rejects shadow authority, not technologies

This reset is not a blanket prohibition on databases, daemons, RPC, validation,
retries, caches, adapters, or presentation models. Such mechanisms survive only
when a retained Soda-owned behavior concretely requires them. They are not
justified solely by the need to synchronize or present a durable copy of state
already owned upstream.

### Custom code stays at the edge

For every proposed Soda component, answer:

1. What exact user outcome requires it?
2. Which existing system already owns most of that outcome?
3. What precise gap remains?
4. Can configuration, packaging, or a short adapter bridge the gap?
5. Does Soda genuinely own a durable fact that must be persisted?
6. What code and state disappear when the existing owner remains authoritative?

If the retained component cannot answer those questions, it should be deleted
rather than reorganized.

### Tests prove behavior; they do not create product authority

Current tests are evidence of the current implementation. They are not a reason
to preserve obsolete schemas, RPCs, reconciliation, or internal APIs after the
ownership decision changes. Replacement tests prove product-level behavior and
the contracts of retained boundaries.

### Delete vertically

The reset is not a package-shuffling exercise. Implementation removes one
unnecessary ownership slice at a time while leaving an understandable working
product after each step. It does not replace the current control plane with
another generic framework, compatibility layer, or workflow engine.

## Target identity, repository, workspace, and credential model

### Primary human accounts and derived workspace accounts

Alice's primary Linux account represents Alice for Cockpit, Forgejo PAM, and
human administrator status. Bob has an equivalent primary account. Linux
administrator status remains authoritative: a primary human account in `wheel`
is a Soda administrator without registration, import, or a parallel Soda role.

Development does not run as the primary human UID. For every human-project pair
selected through **Set up for me**, Soda creates one derived Linux workspace
account. The derived account owns its real private home, complete checkout,
user-local dependencies, caches, project-local data, and development
processes. It is never a Soda person, Forgejo user, project-shared account,
service account, or administrator.

Linux remains authoritative for every resulting UID, home, permission, and
process. Soda owns only the stable association:

```text
primary human account + catalogued project -> derived workspace account
```

The association is represented by the minimal catalog and a deterministic,
collision-safe account convention rather than a mirrored person, membership,
or workspace database.

### Linux and bundled Forgejo authentication

The first human created during Soda installation is an ordinary Linux account
with explicit `wheel` membership. The installation creates the only proactive
Forgejo user: a same-named Forgejo-local account with site-administrator status
through Forgejo's native administrative interface.

The selected outcome gives that account the same initial password as the Linux
account. The installer may reuse the chosen password only through an existing,
bounded installation-time path that leaves no Soda-owned credential state or
retained plaintext. If the installer cannot provide that direct handoff,
password equality must be reconsidered rather than used to justify custom
credential transport, storage, retry, or recovery machinery.

The initial Forgejo administrator is deliberately independent from the
same-named Linux account after installation. Later password, role, rename,
disable, and deletion operations are not projected between the two accounts.

Every later primary human account is owned first by Linux. Any primary human
account accepted by the shipped `soda-forgejo` PAM policy may authenticate to
Forgejo with its Linux username and password. On the first successful PAM
login, Forgejo creates its own ordinary native user record. Derived workspace
accounts are Linux-only development identities that use installed authorized
public keys for direct OpenSSH access; they must not successfully authenticate
through the Forgejo PAM source or create Forgejo users. Later
Linux `wheel` membership has no effect on Forgejo roles; Forgejo administrators
promote or demote users through Forgejo itself.

Soda may configure Forgejo's PAM source with the fixed appliance-local email
domain `localhost`, producing an initial address such as `alice@localhost`.
This is a packaging convention, not an installer input or product-owned email
model. Without it, Forgejo retains its native email fallback behavior. Soda does
not ask for, persist, derive, or manage a separate per-user Forgejo email.

Soda does not maintain a person database, identity mapping table, role mirror,
watcher, reconciliation process, collision policy, or anti-drift mechanism. If
a same-named Forgejo-local account already exists, Soda does not rename either
account or manufacture another identity; the administrator resolves the
conflict in Forgejo.

Disabling a PAM user's Linux account blocks subsequent PAM authentication but
does not claim to revoke existing Forgejo sessions, access tokens, SSH keys, or
repository authorization. Complete Forgejo revocation remains a Forgejo
administrative operation.

### Independently writable derived workspaces

Each successful **Set up for me** operation leaves a complete ordinary Git
clone beneath the derived workspace account's real Linux home:

```text
$HOME/Projects/<repository>
```

Soda and its documentation query the derived account's real home rather than
assuming `/home/<username>`. There is no shared project account, project Unix
group, service account, shared bare repository, shared checkout, shared
worktree, membership record, provisioning job, synthetic home, or project
session database.

Alice and Bob share repository history through the canonical host, but they do
not share a workspace account, home, working directory, Git administrative
directory, local refs, configuration, hooks, uncommitted files, caches, build
output, environment files, credentials, or agent sockets. Git worktrees remain
an ordinary choice within one derived workspace account's clone.

Workspace isolation addresses ordinary development conflicts. Separate UIDs
and homes isolate checkouts, user-local dependencies, caches, project data, and
process ownership. They do not create separate TCP or UDP port namespaces.
Projects select non-conflicting host ports themselves and may optionally use
Podman or another ordinary tool when useful. Soda provides no port allocator,
proxy, network-namespace manager, shared Podman instance, or container control
plane.

OpenSSH logs a human directly into the derived workspace account and its real
home. During setup, Soda copies the primary account's currently authorized
public SSH keys once into the workspace account. Later changes are not
synchronized. Standard interactive shells, direct commands, automation, SCP,
and SFTP operate as the workspace UID. Soda installs no forced command,
project selector, account-transition gateway, synthetic `HOME`, or SFTP
dispatcher.

### Git authentication remains ordinary and transient

Inbound OpenSSH authentication and outbound Git-host authentication are
separate. Copying public keys into a workspace account authorizes direct login
to the Soda machine; it does not give that account a private Git-host key.

Full setup performs one ordinary Git clone or authenticated push as the logged-
in primary human account. Git uses credentials already available to that
account or prompts interactively through Cockpit. Soda does not inspect or
retain the credential. If Git cannot authenticate, setup fails visibly without
altering an existing catalog entry or creating durable provisioning state.

After setup, the human uses ordinary Git authentication from the direct
workspace SSH session, such as per-connection agent forwarding or credentials
configured privately in that workspace account. Soda does not select a method,
enable forwarding globally, generate private keys, store tokens, broker
credentials, or synchronize credential state.

### Curated tools belong to the image

Soda ships a broad reviewed collection of language runtimes and developer tools
as immutable image package composition on both supported architectures. The
exact package list and matched-architecture availability are resolved in #24.

Soda does not resolve latest language releases at runtime, download or extract
tool archives, create project toolchain profiles, persist readiness or version
state, or reconcile tool installations. Users and projects may install or use
additional ordinary ecosystem tooling in their own homes and repositories.

### Stock Cockpit hosts one focused Soda Projects page

Soda consumes the Cockpit packages maintained by its Fedora base. Cockpit uses
normal Linux PAM sessions and standard sudo or polkit privilege escalation;
Linux administrator membership remains authoritative for normal host
administration. Soda supplies branding through supported Cockpit facilities
and one Projects package for catalog discovery, addition, editing, destructive
removal, built-in repository creation, and **Set up for me**.

The Projects page reuses Cockpit's authentication, session, TLS, and process
interfaces. It may invoke one narrowly authorized synchronous operation for
minimal catalog-entry mutation, derived workspace-account creation and
deletion, public-key installation, and placement of a successfully
authenticated clone. The operation exposes no generic `useradd`, `userdel`,
arbitrary path deletion, command execution, Forgejo administration, container
management, or account-management API.

Soda ships no separate dashboard server, authentication helper, session store,
HTTP service, daemon, database, RPC client, job monitor, credential store, or
generic privileged bridge.

### bootc owns updates

Linux administrators check, stage, apply, and roll back Soda OS deployments
with native `bootc` commands. The automatic bootc update timer is disabled so
download, activation, and reboot remain explicit administrator actions.

Soda ships no runtime update daemon, database, release-discovery client,
translated deployment model, RPC, CLI wrapper, or custom Cockpit update page.
Image construction and publication may still produce immutable references and
signing metadata, but they do not create a second runtime update authority.

## Collaboration and conflict-isolation model

Soda is a private development machine for a trusted team. Derived workspace
accounts prevent ordinary development conflicts by separating homes,
checkouts, user-local dependencies, caches, project data, and process
ownership. They are not a promise of separate host networking, system package
sets, mount namespaces, or virtual machines.

Workspace accounts remain private Linux accounts: peers do not normally write
one another's homes, checkouts, credentials, or agent sockets. Members of
`wheel`, root, and other root-equivalent principals remain normal trusted Linux
administrators. Soda adds no attacker-oriented policy system, container
security model, shared privileged container daemon, or network-isolation
subsystem.

## Mutable-state persistence invariant

Normal bootc image replacement must preserve authoritative machine-specific
state, including primary and derived Linux accounts, their homes and workspace
checkouts, the minimal project catalog, Forgejo repositories and machine-
specific mutable Forgejo state, Tailscale machine identity, and other retained
mutable product data. That state is not owned by the replaceable image layer.

Pre-release Soda schemas, mirrors, and internal APIs receive no compatibility
machinery unless explicitly required. Removing obsolete Soda metadata must not
delete authoritative Linux or repository-host state, user-owned files,
repositories, or workspaces except through the explicitly selected destructive
catalog, workspace, or human-account operations.

The exact persistent paths and mount layout are implementation decisions. The
survival of authoritative user and machine state is a product requirement.

## Normative product workflow

The intended install-to-development path is:

1. Install the architecture-matched Soda bootc image.
2. Establish the first Linux administrator and enroll the appliance in the
   owner's Tailnet.
3. Use stock Cockpit through the Tailnet to create or remove primary human Linux
   accounts and perform normal host administration.
4. Let a primary human account authenticate to bundled Forgejo through PAM when
   that human wants a Forgejo identity. Derived workspace accounts never do so.
5. Through the Soda Projects page, add an external canonical Git URL or create a
   repository in the initiating human's native Forgejo namespace and publish
   the resulting URL in the minimal shared catalog.
6. Let any human select **Set up for me**. Ordinary Git authenticates as that
   human using already configured or interactively supplied credentials. Soda
   stores nothing. After Git succeeds, the narrow synchronous workspace
   operation creates the derived Linux account and home, copies the human's
   currently authorized public SSH keys once, and leaves a complete clone at
   `$HOME/Projects/<repository>` under the workspace UID.
7. Connect directly to the derived workspace account through ordinary OpenSSH
   using Codex, Claude, Zed, VS Code, SCP, SFTP, or a normal shell.
8. Run compilation, indexing, tests, agents, databases, and other project
   processes as the derived workspace UID. Projects choose non-conflicting host
   ports and may use Podman or other ordinary tools when useful.
9. Let Git and the canonical host own authorization, branch sharing, review,
   issues, releases, repository rename, and repository deletion.
10. Treat catalog additions and edits as future-facing only. Removing a
    catalogued project has the separate destructive behavior: it removes its
    derived local workspace accounts and managed paths, but never the canonical
    repository.

Soda composes steps 5, 6, and 10 only to provide the appliance-wide catalog and
workspace-account lifecycle. It does not become an identity, repository,
collaborator, credential, Git-host, container, networking, or capability
authority.

## Consequences

### Positive consequences

- Soda retains the project experience that makes the appliance a coherent
  product without copying Linux, Git, Forgejo, or container state.
- Every human-project pair has a distinct home, checkout, dependency space,
  project data area, and process identity.
- Administrators can use standard recovery and inspection paths.
- Users operate authoritative systems directly without a Soda identity or
  repository import step.
- Components can be removed or replaced without migrating shadow state.
- Product tests can focus on user outcomes across real authoritative systems.

### Accepted costs

- Derived UIDs do not isolate host ports. Projects select non-conflicting ports
  or optionally use an ordinary tool such as Podman.
- A human connects with a derived workspace username rather than using one
  primary SSH username for every project.
- Public SSH keys are copied once during setup; later primary-account key
  changes are not synchronized.
- Full setup fails visibly when ordinary Git cannot authenticate. Soda does not
  preserve one-click behavior by retaining credentials or adding a broker.
- Catalog additions and edits do not rename or reconcile existing workspace
  accounts, homes, checkouts, or remotes.
- Project and human removal irreversibly delete selected local workspace data
  with no archive, transfer, distributed transaction, or rollback.
- Forgejo and external-provider account and repository lifecycle remain native
  and separate from Linux-local deletion.
- Upstream behavior and supported interfaces constrain Soda.
- Obsolete implementation-focused tests, schemas, and APIs may be deleted
  rather than preserved for compatibility when their underlying behavior is no
  longer part of the product contract.

## Decisions already made

- Soda is a remote-development appliance for powerful shared machines and
  lightweight clients.
- Soda services are private through Tailscale and are not directly exposed to
  the public Internet.
- OpenSSH is the primary remote-development transport; Cockpit and normal Linux
  tools provide administration.
- AArch64 and x86-64 carry the same released features and product-level
  acceptance expectations unless a reviewed limitation says otherwise.
- Architecture-specific preparation, dependency resolution, build, artifact
  generation, inspection, signing, publication, installation, and validation
  execute on matching native hardware unless that release-integrity policy is
  explicitly reviewed and changed.
- Initial installation uses Anaconda, Kickstart, ordinary Linux credentials,
  OpenSSH, and a one-use Tailscale key; no Soda bootstrap subsystem survives
  installation.
- One primary Linux account represents each human for Cockpit, Forgejo PAM, and
  human administrator status.
- One derived Linux workspace account represents each human-project pair that
  the human sets up. It owns development processes and local project state but
  is not a human, Forgejo identity, administrator, shared project account, or
  service account.
- Linux administrator status is authoritative for Soda administration.
- Installation creates the only proactive Forgejo user: a same-named
  Forgejo-local site administrator for the first Linux `wheel` user.
- Any later primary human account accepted by the shipped Forgejo PAM policy may
  log in; Forgejo creates its own ordinary native user record on first
  successful PAM authentication. Derived workspace accounts are excluded.
- Later Linux `wheel` membership does not grant Forgejo administration; Forgejo
  administrators own later promotion and demotion.
- Forgejo PAM email initialization uses native behavior, optionally with the
  fixed `localhost` packaging convention, and no Soda per-user email model.
- Linux and Forgejo account, credential, role, and revocation state remain
  independent after the applicable Forgejo record exists.
- Bundled Forgejo remains available and external Git hosts remain supported;
  repository creation, access, collaboration, rename, deletion, and purge use
  their native interfaces.
- Soda owns a minimal declarative catalog containing a stable project identity
  and canonical Git URL. It stores no membership, permission, capability,
  credential, clone, process, port, container, job, retry, or status state.
- Every primary human may add, edit, or destructively remove catalogued
  projects. Soda adds no creator-ownership, approval, handoff, or policy model.
- A project created without a URL becomes a native Forgejo repository in the
  initiating human's Forgejo namespace.
- **Set up for me** performs one ordinary interactive Git operation without
  retaining credentials, then creates a derived workspace account with a
  complete clone at `$HOME/Projects/<repository>`.
- Setup copies the primary human account's currently authorized public SSH keys
  once. Later changes are not synchronized.
- Humans connect directly to derived workspace accounts through ordinary
  OpenSSH. Soda installs no forced command, project selector, synthetic home,
  account-transition gateway, or custom SFTP dispatcher.
- Project removal synchronously deletes the catalog entry, every derived
  workspace account for that project, their homes, clones, and explicitly
  Soda-created local paths. It does not delete the canonical repository.
- Primary human deletion removes that human and all of their derived local
  workspaces, but does not delete their Forgejo account or canonical
  repositories.
- Catalog additions and edits affect future discovery and setup only; existing
  workspace accounts, homes, clones, and remotes are never reconciled.
- Stock Fedora Cockpit with Soda branding and one focused Projects page is the
  browser surface. There is no separate dashboard, authentication helper,
  session service, HTTP service, daemon, database, or generic privileged bridge.
- One narrowly authorized synchronous catalog-and-workspace operation may
  mutate minimal catalog entries, create and delete derived accounts, install
  current public keys, and place a successfully authenticated clone. It
  exposes no generic project, account, or command interface.
- Projects manage host ports themselves. Podman remains optional installed
  software and is not a Soda isolation mechanism or control plane.
- Soda ships a broad curated collection of language runtimes and developer
  tools in the immutable image on both architectures, with no runtime Soda
  toolchain manager or persistent toolchain state.
- Linux administrators use native `bootc` commands for explicit update checks,
  staging, activation, and rollback. The automatic timer is disabled and Soda
  ships no runtime update component or shadow deployment state.
- The expected retained generic Soda runtime control-plane surface is zero: no
  SQLite authority, long-running `sodad`, protobuf, gRPC, control socket,
  durable job engine, or reconciliation framework.
- Ordinary Unix accounts provide the selected development conflict boundary;
  root and `wheel` remain trusted Linux administrators.
- Authoritative mutable user, repository, Tailnet, and machine state survives
  normal bootc image replacement.
- The current implementation is not a compatibility contract. Removing its
  mirrors does not authorize deletion of authoritative user data.

## Decisions still open

- The exact broad curated language-runtime and developer-tool package list that
  is available on both supported architectures.
- Verification of the bounded installer-native password handoff for the initial
  Forgejo-local administrator, and the ordinary installer fallback if that
  direct handoff is unavailable.
- The exact architecture-matched Soda image reference installed systems track
  through native bootc configuration.
- The exact declarative catalog file format and persistent location, stable
  project-ID and derived-account naming convention, and narrow authorization
  mechanism that implement the decided behavior without a database or daemon.
- End-to-end proof that Cockpit can provide ordinary interactive Git
  authentication for clone and Forgejo push-to-create without retaining a
  credential, and that native failure remains synchronous.
- The ordinary Linux/PAM configuration that keeps derived workspace accounts
  out of Forgejo PAM provisioning while retaining direct authorized-key SSH.

These are bounded packaging and implementation verifications, not unresolved
product ownership. They do not authorize a runtime toolchain manager,
credential subsystem, update service, generic helper, or control plane.

## Product-level acceptance criteria

The reset is complete only when the resulting product demonstrates the common
criteria and the applicable Git-host scenario without restoring the deleted
Soda control plane.

### Common criteria

1. A fresh image accepts an administrator username, password, SSH public key,
   and one-use Tailscale key; Anaconda and Kickstart create the ordinary Linux
   administrator and a minimal first-boot oneshot enrolls Tailscale.
2. After installation, the owner can use OpenSSH and authenticate to stock
   Cockpit through the Tailnet, neither service is publicly exposed, and no
   temporary enrollment credential or Soda bootstrap subsystem remains.
3. Stock Cockpit contains Soda branding and exactly one focused Soda Projects
   package. No separate Soda web server, TLS stack, authentication helper,
   session service, daemon, database, RPC client, job monitor, or credential
   store exists.
4. Alice and Bob have primary human Linux accounts. Each may add, edit, or
   remove entries in the minimal shared project catalog.
5. A catalog entry contains only its stable project identity and canonical Git
   URL or equivalently minimal declarative association. It contains no user,
   membership, permission, capability, credential, clone, process, port,
   container, job, retry, or status state.
6. Alice and Bob can each select **Set up for me** for the same project. Ordinary
   Git authenticates interactively as the initiating primary human without
   Soda retaining a credential. Successful setup creates separate derived
   workspace accounts, real private homes, and complete writable clones at each
   workspace account's `$HOME/Projects/<repository>`.
7. Setup copies each primary account's currently authorized public SSH keys once
   into that human's derived workspace account. Later primary-account key
   changes are not synchronized.
8. Alice and Bob connect directly to their respective derived workspace
   usernames through ordinary OpenSSH. Interactive shells, direct commands,
   automation, SCP, and SFTP run as the authenticated workspace UID without a
   forced command, project selector, account-transition gateway, synthetic
   home, or custom dispatcher.
9. Each workspace has separate file ownership, checkout state, user-local
   dependencies, caches, project data, and processes. Projects select
   non-conflicting host ports; Soda provides no port allocator, network
   namespace, shared Podman instance, or container control plane.
10. Catalog additions and edits affect future discovery and setup only. They do
    not rename or reconcile existing accounts, homes, clones, or Git remotes.
11. Removing the project terminates its local workspace processes and deletes
    its catalog entry, derived accounts, homes, complete clones, and explicitly
    Soda-created workspace paths. The canonical repository remains intact.
12. Deleting Alice's primary Linux account removes Alice's derived workspace
    accounts and local managed paths but does not delete Alice's Forgejo account
    or canonical repositories.
13. Native failure during Git authentication, setup, or removal is reported
    synchronously without a durable job, retry, rollback, compensation, or
    reconciliation record.
14. The reviewed image toolset is available on both supported architectures
   without runtime Soda toolchain state or management services.
15. A Linux administrator can inspect, stage, activate, and roll back deployments
   with native `bootc` commands. The automatic update timer is disabled and no
   Soda update service or shadow deployment model exists.
16. A normal bootc image update preserves primary and derived Linux accounts,
   their homes and workspace clones, the project catalog, Forgejo state,
   Tailscale identity, and other retained machine-specific state.
17. The same product-level acceptance scenarios pass on x86-64 and AArch64,
   subject only to explicitly documented architecture-specific limitations.
18. For each architecture, architecture-specific release stages execute on
   matching native hardware. Any cross-architecture release coordination
   consumes already verified native outputs and does not substitute for native
   execution.

### Bundled Forgejo scenario

1. Installation creates a same-named Forgejo-local site administrator for the
   first Linux `wheel` user without a Soda person, role, identity mapping, or
   retained credential record.
2. Where the existing installer path supports a bounded direct handoff, the
   installer-supplied initial password authenticates the first administrator to
   both Linux and Forgejo without leaving Soda-owned credential state.
3. Forgejo's PAM source is active while ordinary local self-registration remains
   disabled. Any accepted primary human account can authenticate with its Linux
   username and password, and Forgejo creates an ordinary native user on first
   successful PAM login. Derived workspace accounts cannot successfully use
   that PAM source and never become Forgejo users.
4. When the fixed PAM email-domain convention is enabled, Forgejo initializes
   that user's email as `<username>@localhost`; otherwise Forgejo uses its native
   fallback behavior.
5. Later Linux `wheel` membership has no effect on Forgejo roles. Forgejo
   administrators promote and demote users through Forgejo itself.
6. Linux and Forgejo state is not projected between systems. Disabling a Linux
   account blocks later PAM authentication but does not claim to revoke Forgejo
   sessions, tokens, SSH keys, or repository authorization.
7. When Alice creates a catalogued project without a URL, an ordinary
   authenticated Forgejo operation creates the repository in Alice's native
   Forgejo namespace. No global Soda token, credential broker, copied password,
   user projection, provider abstraction, or repository database is involved.
8. Forgejo owns repository access and collaboration. Alice grants Bob access
   through Forgejo's native model, and both work from their independent derived
   workspace accounts, push distinct branches, and collaborate through
   Forgejo's native review mechanism.
9. Git reports clone, fetch, push, and authentication results directly; Soda
   does not translate them into capability, membership, or provisioning state.
10. Removing the Soda catalog entry and local workspaces does not delete the
    Forgejo repository. Forgejo account or repository deletion remains a
    separate Forgejo-native action.

### External canonical repository scenario

1. Any human may add an external canonical Git URL to the shared catalog.
   Repository lifecycle, access, collaboration, rename, and deletion remain
   entirely within the external provider's native interfaces.
2. **Set up for me** uses an ordinary interactive Git clone authenticated as the
   initiating human. A failed clone leaves the existing catalog entry and other
   humans' workspaces unchanged and creates no durable failure state.
3. A successful clone is placed beneath the derived workspace account's
   `$HOME/Projects`; later fetch and push use ordinary workspace-controlled Git
   authentication.
4. Git reports each command's result directly. Soda does not inspect provider
   membership, test or classify capabilities, grant or revoke access, or
   persist provider state.
5. Removing the catalog entry deletes Soda-managed local workspaces but neither
   changes external authorization nor erases the external repository.

## Review order and outputs

Resolve the ownership reviews in dependency order:

1. [#40: installer: provision the first administrator and Tailnet enrollment](https://github.com/LevitateOS/soda-os/issues/40)
2. [#33: architecture(identity): make Linux identity and administrator status authoritative](https://github.com/LevitateOS/soda-os/issues/33)
3. [#37: architecture(git-host): preserve native repository-host ownership](https://github.com/LevitateOS/soda-os/issues/37)
4. [#35: architecture(workspaces): provide catalogued derived workspace accounts](https://github.com/LevitateOS/soda-os/issues/35)
5. [#36: architecture(ssh): use ordinary OpenSSH for direct workspace accounts](https://github.com/LevitateOS/soda-os/issues/36)
6. [#32: architecture(cockpit): retain stock Cockpit with branding and Projects page](https://github.com/LevitateOS/soda-os/issues/32), followed by [#34: architecture(telemetry): remove redundant host telemetry machinery](https://github.com/LevitateOS/soda-os/issues/34)
7. [#24: architecture(toolchains): ship a curated immutable image toolset](https://github.com/LevitateOS/soda-os/issues/24) and [#38: architecture(updates): use administrator-controlled native bootc operations](https://github.com/LevitateOS/soda-os/issues/38)
8. [#23: release: replace the custom GitHub publication client](https://github.com/LevitateOS/soda-os/issues/23) after #38 determines which release metadata remains
9. [#25: test(acceptance): collapse raw-QEMU test orchestration](https://github.com/LevitateOS/soda-os/issues/25) around the resulting product outcomes
10. [#39: architecture(control-plane): remove residual runtime infrastructure](https://github.com/LevitateOS/soda-os/issues/39) as the capstone after the preceding deletions

Issue #33 owns primary-human Linux identity, administrator authority, and local
human deletion. Issue #35 owns the minimal catalog, derived workspace-account
association, synchronous setup, and destructive project removal while deleting
the old project control plane. Issue #36 owns direct workspace OpenSSH behavior,
one-time public-key installation, and removal of forced SSH machinery. Issue
#37 owns deletion of Soda repository-host mappings, collaborator projections,
provider operations, and capability models while retaining native Forgejo and
external-host authority and the catalog's canonical URL.

Issue #32 removes the standalone Cockpit application while retaining stock
Cockpit, Soda branding, and one focused Projects package. #34 removes telemetry
machinery that existed for the deleted custom surface. Issue #24 removes the
runtime toolchain manager while selecting the image package list. Issue #38
removes the runtime updater in favor of native bootc. Issue #39 owns only
residual generic infrastructure not already deleted by those vertical issues;
it verifies that the narrow synchronous workspace helper is the only retained
Soda runtime privilege boundary.

Each review produces the same outputs:

- required user outcome;
- authoritative owner;
- irreducible Soda-owned fact, if any;
- retained adapter or privileged boundary;
- components and schemas to delete;
- native failure behavior and destructive effects; and
- product-level acceptance criteria.

Concrete correctness fixes may proceed independently. They must not be used to
polish an ownership boundary that its architecture review has not justified.

## Non-goals

- Rebuilding Linux, Cockpit, OpenSSH, Git, Forgejo, bootc, or an existing
  toolchain manager behind a different Soda API.
- Adding a browser IDE or replacing SSH-capable development clients.
- Managing arbitrary external Git-provider membership through a generic
  provider integration.
- Adding public-Internet exposure, enterprise identity, generic orchestration,
  workflow engines, event buses, or background policy systems.
- Adding Soda-managed host-port allocation, network namespaces, shared Podman
  state, container orchestration, or VM isolation to the selected UID-and-home
  development conflict boundary.
- Removing established user outcomes merely to reduce a line count.

The objective is a smaller and more legible product because the correct systems
own their natural responsibilities, not because smallness is itself the product.
