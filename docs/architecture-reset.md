# Soda OS architectural reset

**Status:** Accepted product direction and governing architectural constraints;
bounded packaging and installation verification remains assigned to the linked
issues.

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

Soda owns the branded, installable composition, the `$HOME/Projects` workspace
convention, a curated image-resident development toolset, and the smallest
installation-time composition needed to make that way of working reliable. It
does not own project provisioning or lifecycle, parallel implementations of
Linux identity, OpenSSH sessions, Git-host collaboration, Cockpit
administration, runtime language-toolchain management, or bootc deployment
management.

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
3. Administer the machine through Cockpit and normal Linux tools.
4. Connect from a lightweight client through OpenSSH with an SSH-capable
   development tool.
5. Create or select a repository and manage its access through bundled Forgejo
   or the external canonical Git host.
6. Let each Linux user clone that repository with ordinary Git into
   `$HOME/Projects/<repository>`.
7. Use the canonical Git host for authorization, branch exchange, review,
   issues, and releases where that host provides them.

The client retains the human interface. Compilation, indexing, tests, agents,
and development processes run on the Soda machine. Each person configures
ordinary Git and OpenSSH credentials within their own Linux and client
environment; Soda neither enables agent forwarding globally nor manages a
parallel server-side Git-key system.

OpenSSH is the primary remote-development transport. Cockpit and normal Linux
tools provide administration. Tailscale provides private reachability but does
not replace Linux login, process attribution, or Git-host authorization.

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
| Human process and filesystem identity | Linux UID and account |
| Administrator status | Linux administrator membership, currently `wheel` |
| Repository creation, lifecycle, access, and collaboration | The canonical Git host |
| Personal repository checkout | Git and the owning Linux user |
| Host state and service control | Linux kernel, systemd, PAM, polkit, and standard system interfaces |
| Host administration interface | Stock Cockpit |
| Cockpit presentation customization | Soda branding only |
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

### Repositories and access remain native

A project is not a Soda domain object. It is a repository on its authoritative
Git host plus independently owned clones in developers' Linux homes. Git
checkouts may have additional remotes, but Soda stores no repository record,
canonical-host designation, membership, collaborator view, permission copy, or
capability status.

For a Forgejo-hosted repository, Forgejo owns creation, rename, deletion,
authorization, users and keys in its application domain, teams or repository
collaborators, review, issues, and releases. Ordinary users and Forgejo
administrators use Forgejo's native policy and interfaces directly.

For a repository hosted by GitHub, GitLab, another Forgejo instance, or another
external Git service, that service owns the same responsibilities. Users manage
access there and run ordinary `git clone`, `git fetch`, and `git push` with
their own credentials. Git reports those command results; Soda does not probe,
classify, cache, or present external capabilities.

Importing an external repository into bundled Forgejo is a Forgejo-native
operation. Once imported, Forgejo owns the resulting repository. Soda does not
coordinate the import or retain an association between the source and result.

### Native lifecycle actions remain separate

Linux account deletion, Forgejo account deletion or purge, external-provider
account deletion, repository deletion, and local checkout deletion remain
separate native actions. Soda does not provide a distributed deletion,
transfer, handoff, rollback, reconciliation, or anti-drift workflow.

Deleting a Linux account may irreversibly remove its home and every checkout
beneath it. Deleting a local checkout destroys that copy, including uncommitted
and unpushed work, but does not revoke repository-host authorization or erase
copies obtained elsewhere. Forgejo-owned repositories and content are removed
only through Forgejo's native deletion or purge behavior; external repositories
remain the external provider's responsibility. Teams perform any desired
handoff before invoking those destructive native operations.

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

### One Linux user per human

Alice works as Linux user `alice`; Bob works as `bob`. Their UID is the process
and filesystem attribution boundary. They do not normally log in as a shared
project account.

Linux administrator status is authoritative. A Linux administrator is a Soda
administrator without registration, import, or a parallel Soda role.

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

Every later account is owned first by Linux. Any ordinary Linux account accepted
by the shipped `soda-forgejo` PAM policy may authenticate to Forgejo with its
Linux username and password. On the first successful PAM login, Forgejo creates
its own ordinary native user record. Later Linux `wheel` membership has no
effect on Forgejo roles; Forgejo administrators promote or demote users through
Forgejo itself.

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

### Independently managed personal checkouts

Each person creates and manages their own clone with ordinary Git beneath their
actual Linux home directory. Soda's workspace convention is:

```text
$HOME/Projects/<repository>
```

Soda and its documentation query the account's real home directory rather than
assuming `/home/<username>`. The directory is owned by that Linux user. There is
no Soda project root, descriptor, project database, project Unix group,
synthetic project account, project service account, shared bare repository,
membership record, checkout-provisioning job, or project-specific session home.

Alice and Bob share repository history through the canonical host, but they do
not share a working directory, Git administrative directory, local refs,
configuration, hooks, uncommitted files, caches, build output, environment
files, credentials, or agent sockets. Git worktrees remain an ordinary
per-person choice within one person's clone.

OpenSSH logs each person into their normal Linux account and home. Users or
their SSH-capable clients select the checkout directory normally; Soda installs
no forced command, project selector, shell gateway, synthetic `HOME`, or SFTP
dispatcher.

### Credentials belong to the human

Alice must not be able to use Bob's Git credentials, and Bob must not be able to
use Alice's. Credential isolation follows the Linux UID and private home. Each
person configures ordinary Git, OpenSSH, agent forwarding, HTTPS credentials,
or ecosystem credential helpers as they choose. Soda does not enable agent
forwarding globally and does not generate, store, rotate, broker, or synchronize
per-person outbound Git credentials.

### Curated tools belong to the image

Soda ships a broad reviewed collection of language runtimes and developer tools
as immutable image package composition on both supported architectures. The
exact package list and matched-architecture availability are resolved in #24.

Soda does not resolve latest language releases at runtime, download or extract
tool archives, create project toolchain profiles, persist readiness or version
state, or reconcile tool installations. Users and projects may install or use
additional ordinary ecosystem tooling in their own homes and repositories.

### Stock Cockpit provides administration

Soda consumes the Cockpit packages maintained by its Fedora base. Cockpit uses
normal Linux PAM sessions and standard sudo or polkit privilege escalation;
Linux administrator membership remains authoritative. Soda supplies branding
through supported Cockpit branding facilities but ships no Soda-specific
Cockpit page, backend, authentication helper, session store, HTTP service, or
privileged bridge.

### bootc owns updates

Linux administrators check, stage, apply, and roll back Soda OS deployments
with native `bootc` commands. The automatic bootc update timer is disabled so
download, activation, and reboot remain explicit administrator actions.

Soda ships no runtime update daemon, database, release-discovery client,
translated deployment model, RPC, CLI wrapper, or custom Cockpit update page.
Image construction and publication may still produce immutable references and
signing metadata, but they do not create a second runtime update authority.

## Trust model

Soda provides ordinary Unix multi-user separation for a private machine used by
a trusted group. It does not provide hostile-tenant isolation.

Ordinary users must not be able to write another user's workspace or access
another user's private home, stored credentials, or active agent socket.
Members of `wheel`, root, and other root-equivalent principals are fully
trusted. Soda does not promise protection from a malicious machine
administrator, kernel compromise, or deliberate exhaustion of shared CPU,
memory, storage, or GPU resources.

Any feature that grants root-equivalent access, including access to a privileged
shared container daemon, must be evaluated against this trust model. Otherwise,
the UID credential boundary would be nominal rather than real.

## Mutable-state persistence invariant

Normal bootc image replacement must preserve authoritative machine-specific
state, including Linux accounts, user homes and personal checkouts, Forgejo
repositories and machine-specific mutable Forgejo state, Tailscale machine
identity, and other retained mutable product data. That state is not owned by
the replaceable image layer.

Pre-release Soda schemas, mirrors, and internal APIs receive no compatibility
machinery unless explicitly required. Removing obsolete Soda metadata must not
delete authoritative Linux or repository-host state, user-owned files,
repositories, or workspaces unless that destructive removal is separately
approved.

The exact persistent paths and mount layout are implementation decisions. The
survival of authoritative user and machine state is a product requirement.

## Normative product workflow

The intended install-to-development path is:

1. Install the architecture-matched Soda bootc image.
2. Establish the first Linux administrator and enroll the appliance in the
   owner's Tailnet.
3. Use stock Cockpit through the Tailnet for normal Linux administration.
4. Create a Linux account or select an existing account through native Linux or
   Cockpit administration.
5. Create or select a repository and manage collaborators through bundled
   Forgejo or the external canonical Git host.
6. Connect as the Linux user through ordinary OpenSSH.
7. Run `git clone <url> "$HOME/Projects/<repository>"` with that user's normal
   Git credentials.
8. Open that directory through Codex, Claude, Zed, VS Code, or normal shell
   tools.
9. Let Git and the canonical host own branch sharing and any review, issue, or
   release facilities the host provides.

Soda does not sit in steps 4 through 9 as an identity, project, collaborator,
credential, provisioning, or capability authority. Its contribution is the
installed composition and documented convention.

## Consequences

### Positive consequences

- Soda owns less durable state and fewer synchronization boundaries.
- Administrators can use standard recovery and inspection paths.
- Users operate authoritative systems directly without a Soda import step.
- Components can be removed or replaced without migrating shadow state.
- Product tests can focus on user outcomes across real authoritative systems.

### Accepted costs

- Soda provides no integrated project-onboarding, collaborator, checkout,
  credential, or project-lifecycle workflow.
- Users move between Forgejo or an external provider, OpenSSH, and ordinary Git
  rather than one synthetic Soda transaction.
- Linux, Forgejo, external-provider, repository, and checkout deletion are
  separate destructive actions with no Soda transfer or recovery workflow.
- Upstream behavior and supported interfaces constrain Soda.
- Custom dashboard pages and workflows disappear; Cockpit provides its stock
  interface with Soda branding only.
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
- One Linux user represents one human.
- Linux administrator status is authoritative for Soda administration.
- Installation creates the only proactive Forgejo user: a same-named
  Forgejo-local site administrator for the first Linux `wheel` user.
- Any later Linux account accepted by the shipped Forgejo PAM policy may log in;
  Forgejo creates its own ordinary native user record on first successful PAM
  authentication.
- Later Linux `wheel` membership does not grant Forgejo administration; Forgejo
  administrators own later promotion and demotion.
- Forgejo PAM email initialization uses native behavior, optionally with the
  fixed `localhost` packaging convention, and no Soda per-user email model.
- Linux and Forgejo account, credential, role, and revocation state remain
  independent after the applicable Forgejo record exists.
- Bundled Forgejo remains available and external Git hosts remain supported;
  repository creation, access, collaboration, rename, deletion, and purge use
  their native interfaces.
- Soda has no project control plane, project domain object, repository record,
  descriptor, project group, project or service account, membership,
  collaborator view, checkout provisioning, capability model, or project
  lifecycle workflow.
- Each Linux user creates and manages an independent clone with ordinary Git at
  `$HOME/Projects/<repository>`.
- Working state is private to its Linux user; repository history is shared
  through the canonical Git host.
- Human Git credentials are isolated per Linux user and configured through
  ordinary Git, OpenSSH, and ecosystem mechanisms without Soda key management.
- Linux, Forgejo, external-provider, repository, and local-checkout deletion are
  separate native actions. Soda performs no transfer, handoff, distributed
  deletion, rollback, or reconciliation.
- Removing a local checkout irreversibly discards its uncommitted and unpushed
  data but is not repository-access revocation and cannot erase other copies.
- Stock Fedora Cockpit with Soda branding is the complete browser
  administration surface; Soda ships no custom page, backend, authentication
  helper, session service, or privileged bridge.
- Soda ships a broad curated collection of language runtimes and developer
  tools in the immutable image on both architectures, with no runtime Soda
  toolchain manager or persistent toolchain state.
- Linux administrators use native `bootc` commands for explicit update checks,
  staging, activation, and rollback. The automatic timer is disabled and Soda
  ships no runtime update component or shadow deployment state.
- The expected retained generic Soda runtime control-plane surface is zero: no
  SQLite authority, long-running `sodad`, protobuf, gRPC, control socket,
  durable job engine, or reconciliation framework.
- Ordinary Unix separation protects users from peers, not from root or trusted
  administrators.
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

These are bounded packaging and implementation verifications. They do not
authorize a runtime toolchain manager, credential subsystem, update service, or
generic control plane.

## Product-level acceptance criteria

The reset is complete only when the resulting product demonstrates the common
criteria and the applicable Git-host scenario without a Soda project control
plane.

### Common criteria

1. A fresh image accepts an administrator username, password, SSH public key,
   and one-use Tailscale key; Anaconda and Kickstart create the ordinary Linux
   administrator and a minimal first-boot oneshot enrolls Tailscale.
2. After installation, the owner can use OpenSSH and authenticate to stock
   Cockpit through the Tailnet, neither service is publicly exposed, and no
   temporary enrollment credential or Soda bootstrap subsystem remains.
3. Stock Cockpit contains Soda branding but no Soda-specific page, backend,
   authentication helper, session service, or privileged bridge.
4. Alice and Bob connect through OpenSSH as their own Linux users.
5. Each user can run ordinary Git to create and manage an independent clone at
   `$HOME/Projects/<repository>` without a Soda project record, descriptor,
   group, service account, provisioning job, or project-selection SSH helper.
6. Neither user can write the other's checkout or access the other's private
   home, credentials, or active agent socket through ordinary peer access.
7. The reviewed image toolset is available on both supported architectures
   without runtime Soda toolchain state or management services.
8. A Linux administrator can inspect, stage, activate, and roll back deployments
   with native `bootc` commands. The automatic update timer is disabled and no
   Soda update service or shadow deployment model exists.
9. A normal bootc image update preserves Linux users, homes, personal
   checkouts, Forgejo state, Tailscale identity, and other retained
   machine-specific state.
10. The same product-level acceptance scenarios pass on x86-64 and AArch64,
   subject only to explicitly documented architecture-specific limitations.
11. For each architecture, architecture-specific release stages execute on
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
   disabled. Any account accepted by `soda-forgejo` PAM can authenticate with
   its Linux username and password, and Forgejo creates an ordinary native user
   on first successful PAM login.
4. When the fixed PAM email-domain convention is enabled, Forgejo initializes
   that user's email as `<username>@localhost`; otherwise Forgejo uses its native
   fallback behavior.
5. Later Linux `wheel` membership has no effect on Forgejo roles. Forgejo
   administrators promote and demote users through Forgejo itself.
6. Linux and Forgejo state is not projected between systems. Disabling a Linux
   account blocks later PAM authentication but does not claim to revoke Forgejo
   sessions, tokens, SSH keys, or repository authorization.
7. Users create, rename, delete, select, and authorize repositories directly
   through Forgejo's native interfaces, without a Soda repository database,
   collaborator model, or project role.
8. Alice and Bob clone the same Forgejo repository into their own
   `$HOME/Projects` directories, push distinct branches, and collaborate through
   Forgejo's native review mechanism.
9. Git reports clone, fetch, and push results directly; Soda does not translate
   them into capability or provisioning state.
10. Forgejo account or repository deletion and Linux account or home deletion
   remain separate native operations with no Soda transfer, reconciliation, or
   distributed-deletion workflow.

### External canonical repository scenario

1. Repository lifecycle, access, collaboration, rename, and deletion remain
   entirely within the external provider's native interfaces.
2. Each user runs ordinary `git clone`, `git fetch`, and `git push` with their
   own credentials and keeps the clone beneath `$HOME/Projects`.
3. Git reports each command's result directly. Soda does not designate a
   canonical repository, inspect provider membership, test or classify
   capabilities, grant or revoke access, or persist provider state.
4. Deleting a local checkout is an ordinary destructive filesystem action that
   neither changes external authorization nor erases another copy.

## Review order and outputs

Resolve the ownership reviews in dependency order:

1. [#40: installer-native initial administrator and Tailnet provisioning](https://github.com/LevitateOS/soda-os/issues/40)
2. [#33: Linux identity and administrator authority](https://github.com/LevitateOS/soda-os/issues/33)
3. [#35: remove Soda project lifecycle and retain personal Git checkouts](https://github.com/LevitateOS/soda-os/issues/35)
4. [#37: preserve native repository-host ownership](https://github.com/LevitateOS/soda-os/issues/37)
5. [#36: retain ordinary OpenSSH and personal workspace navigation](https://github.com/LevitateOS/soda-os/issues/36)
6. [#32: retain stock Cockpit with Soda branding](https://github.com/LevitateOS/soda-os/issues/32), followed by [#34: remove redundant telemetry machinery](https://github.com/LevitateOS/soda-os/issues/34)
7. [#24: ship a curated immutable image toolset](https://github.com/LevitateOS/soda-os/issues/24) and [#38: use administrator-controlled native bootc operations](https://github.com/LevitateOS/soda-os/issues/38)
8. [#39: remove residual runtime infrastructure](https://github.com/LevitateOS/soda-os/issues/39) as the capstone after the preceding deletions

Issue #33 owns native Linux account behavior and Linux account or home deletion.
Issue #35 owns deletion of the Soda project and workspace control plane and the
`$HOME/Projects/<repository>` convention. Issue #36 owns removal of forced SSH
project selection and synthetic sessions. Issue #37 owns deletion of Soda
repository-host mappings, collaborator projections, provider operations, and
capability models while retaining native Forgejo and external-host behavior.

Issue #32 removes the custom Cockpit application and retains only stock Cockpit
branding; #34 removes telemetry machinery that existed for the deleted custom
surface. Issue #24 removes the runtime toolchain manager while selecting the
image package list. Issue #38 removes the runtime updater in favor of native
bootc. Issue #39 owns only residual generic infrastructure not already deleted
by those vertical issues and may close as verification-only when none remains.

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
- Promising hostile-tenant, container, or VM isolation that has not been
  selected as a product requirement.
- Removing established user outcomes merely to reduce a line count.

The objective is a smaller and more legible product because the correct systems
own their natural responsibilities, not because smallness is itself the product.
