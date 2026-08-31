# Soda OS architectural reset

**Status:** Accepted product direction and governing architectural constraints;
open product-policy and implementation decisions remain assigned to the linked
reviews.

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

Soda owns the branded, installable composition, project conventions, and the
smallest cross-system onboarding needed to make that way of working reliable.
It does not own parallel implementations of Linux identity, OpenSSH sessions,
Git-host collaboration, Cockpit administration, language-toolchain
distribution, or bootc deployment management.

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
5. Create a project from a new bundled Forgejo repository or an existing
   canonical Git repository.
6. Give each collaborator an independently writable, person-owned checkout.
7. Use the canonical Git host for repository authorization, branch exchange,
   review, issues, and releases where that host provides them.

The client retains the human interface. In the agent-forwarding model, it also
retains private authentication keys. Compilation, indexing, tests, agents, and
development processes run on the Soda machine. A protected server-side
per-person Git key remains an open alternative for clients that cannot forward
an agent reliably.

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
| Host state and service control | Linux kernel, systemd, PAM, polkit, and standard system interfaces |
| Host administration interface | Cockpit |
| Product-specific administration composition | Soda Cockpit package |
| Local project-resource access | Filesystem permissions and an optional Unix group where justified |
| Local repository state and history | Git |
| Repository authorization and collaboration | The canonical Git host |
| OS deployment state and activation | bootc |
| Language runtimes and developer toolchains | Selected upstream manager or ecosystem tooling; selection pending #24 |
| Appliance image, conventions, and irreducible associations | Soda OS |

Soda may coordinate a workflow across these owners. It must not persist a
second authoritative copy merely to make queries or UI rendering convenient.
Transient aggregation, caches with no authority, and presentation models remain
acceptable when a retained user-facing behavior requires them.

Soda services are reached through the Tailnet unless an explicit future
requirement says otherwise. OpenSSH, Cockpit, Forgejo, and product-specific
interfaces are not directly exposed to the public Internet.

### Repository authority follows the canonical host

A Soda project designates one canonical collaboration repository and host. Git
checkouts may have additional remotes, but Soda does not treat those remotes as
authoritative for project authorization or collaboration status.

For a Forgejo-hosted project, Forgejo owns repository authorization, users and
keys in its application domain, teams or repository collaborators, review,
issues, and releases. Soda uses Forgejo's native authorization model without
requiring a one-to-one structural mirror between a Unix group and a Forgejo
team. Through a reviewed Forgejo integration, Soda may inspect native
authorization and coordinate explicitly supported grants and revocations.

For a project whose canonical repository remains on GitHub, GitLab, another
Forgejo instance, or another external Git service, that service owns those
facts. Bundled Forgejo is not inserted as a parallel authority. External
provider administration is outside the initial Soda contract.

Importing an external repository into bundled Forgejo is a separate explicit
operation. Once the import is accepted and verified, bundled Forgejo becomes
the canonical host. Merely cloning an external URL does not change authority.

External repository access is capability-based. Soda normally cannot determine
authoritative membership or grant access on an arbitrary external host. It can
use the person's credentials, attempt the required clone, fetch, or push, report
which operation succeeds, and direct the administrator to manage authorization
through the external provider.

Capability results are operation-specific and depend on the credentials
available to the person's current session. Untested or unavailable capability
information is `unknown`, not denied. Soda does not create undisclosed remote
state to probe capability: push capability is learned from a user-requested
push, while any non-mutating capability check must be explicitly selected and
must report only what it exercised.

### Local and repository access are separate grants

Local filesystem authorization and repository-host authorization protect
different resources. Their overlap does not create a third authoritative Soda
membership.

A project collaborator is a transient, faceted product view. Soda presents
local workspace access, shared-local-resource access, and each observable
repository capability separately. Untested or unavailable capability
information is `unknown`, not denied.

For Forgejo-hosted projects, Soda may inspect native Forgejo authorization. For
externally hosted projects, authorization remains externally administered, and
Soda reports only the Git capabilities verified through operations using the
person's credentials.

Soda does not persist cached collaborator lists, copied repository permissions,
or a durable provisioning status merely to present this view.

### Cross-system workflows are derived and re-runnable

For each authoritative system that Soda is explicitly designed and authorized
to modify, provisioning applies independently inspectable, idempotent
operations. Cross-system changes are not normally atomic. Partial completion
must identify the observed state of each applicable owner and allow the
operation to be safely re-run.

For a bundled Forgejo project, Soda may coordinate supported Linux and Forgejo
operations through reviewed integrations. For an external canonical
repository, Soda modifies only supported local state and observes the result of
credential-backed Git operations; authorization changes remain external.

Soda may mutate authoritative systems only through explicitly supported
boundaries. For unsupported external repository hosts, Soda observes
credential-backed Git capabilities without claiming membership knowledge or
provider-administration authority.

Soda does not add a transaction log, rollback engine, or reconciliation daemon
merely to simulate an atomic transaction across Linux and a repository host.
Revocation must report residual access especially clearly when one owner has
removed access and another has not.

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

## Target identity, project, workspace, and credential model

### One Linux user per human

Alice works as Linux user `alice`; Bob works as `bob`. Their UID is the process
and filesystem attribution boundary. They do not normally log in as a shared
project account.

Linux administrator status is authoritative. A Linux administrator is a Soda
administrator without registration, import, or a parallel Soda role.

Linux and repository-host identities remain distinct. The review must decide
how a Linux account is correlated with a bundled Forgejo account without
creating a parallel Soda person database. An identical-username contract,
Forgejo authentication configuration, explicit account selection, or a minimal
irreducible mapping are candidates, not decisions.

The privilege path is also open. Linux `wheel` membership does not inherently
make that person a Forgejo site administrator. Project onboarding must not
require a parallel Soda administrator role or an unnecessarily powerful,
long-lived Forgejo credential. Until reviewed, no new authority for ordinary
users to provision projects or change local membership is implied.

### Minimal project descriptor

The association between a local workspace hierarchy and its canonical
repository may be an irreducible Soda-owned fact when it cannot be derived
unambiguously. A small declarative project descriptor is permitted for that
association and may contain irreducible association facts such as:

```text
project identifier
workspace root
optional Unix group
canonical repository URL
repository hosting mode
optional Forgejo repository identifier
workspace visibility policy
```

Repository URLs and upstream identifiers in the descriptor are references, not
copied assertions about current repository-host state, and must be validated
when used. Repository URLs stored by Soda must not contain credentials.

It must not contain cached collaborator lists, repository permissions,
provisioning status, copied upstream accounts, or operational state already
owned by Linux, Git, or the repository host.

### Project Unix groups require a concrete local resource

A Unix group is justified only when it governs a concrete local resource, such
as traversal of a project hierarchy, explicitly shared artifacts or services,
or authorization to receive a workspace beneath a protected project root. It is
not a copy of canonical-host repository membership.

If a project has no shared local resource and collaboration occurs entirely
through the canonical Git host, a per-project Unix group may not be necessary.
Whether groups are universal or optional remains under review in #35.

The permission-policy invariants are:

- the project hierarchy is administrator-controlled;
- each person can access their own workspace;
- other ordinary users cannot access that workspace by default; and
- explicitly shared local resources use a reviewed group or ACL boundary.

Issue #35 selects the concrete ownership modes, permissions, and ACLs. The
default protects uncommitted environment files, agent logs, generated
credentials, test data, and other material that never reaches Git. A project
may deliberately enable cross-user reading, but it is not the universal
default.

### One person-owned checkout per person and project

Alice and Bob share a repository and its history, but they do not write to the
same working directory. A target layout may resemble:

```text
/srv/soda/projects/example/
├── alice/
│   └── repository/
└── bob/
    └── repository/
```

Each checkout is independently writable and owned by the corresponding human.
Changes are exchanged through Git and reviewed through the canonical host.

Git worktrees remain useful within one person's clone when that person or their
agents need several concurrent branches. A Git common directory shared for
write access across different Unix users is not the intended security boundary:
it also shares refs, configuration, locks, hooks, and administrative metadata.

### Credentials belong to the human, not the project

Alice must not be able to use Bob's Git credentials, and Bob must not be able to
use Alice's. Project directories contain no shared human credentials.
Credential isolation follows the Linux UID through a private home, a per-user
agent socket, or another reviewed per-user mechanism.

The exact outbound Git credential method remains open. The review compares at
least:

- SSH agent forwarding, which keeps private keys on the client; and
- a protected per-person key on the Soda machine, which is simpler for clients
  that do not support forwarding consistently.

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
state, including Linux accounts, user homes, project workspaces, Forgejo
repositories and configuration, Tailscale machine identity, and other retained
mutable product data. That state is not owned by the replaceable image layer.

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
3. Open standard Cockpit through the Tailnet.
4. Create a Linux account or select an existing account.
5. Select the repository mode: new bundled Forgejo repository, existing
   bundled Forgejo repository, external canonical repository, or explicit
   import into Forgejo.
6. Establish the local project-resource boundary only where a local shared
   resource requires it.
7. For bundled Forgejo, grant repository access through its native
   authorization model. For an external canonical host, authorization remains
   externally administered and Soda reports only Git capabilities observed
   through the person's credential-backed operations.
8. Create one independently writable, person-owned checkout for each local
   collaborator.
9. Show ready-to-use SSH connection guidance for Codex, Claude, Zed, VS Code,
   and normal OpenSSH clients.
10. Let Git and the canonical host own branch sharing, review, issues, and
    releases.

The exact bootstrap path for steps 2 and 3 and the privilege required for steps
5 through 8 remain under review. The workflow defines the user outcome without
manufacturing implementation authority.

## Consequences

### Positive consequences

- Soda owns less durable state and fewer synchronization boundaries.
- Administrators can use standard recovery and inspection paths.
- Direct upstream changes become visible without a manual Soda import step.
- Components can be removed or replaced without migrating shadow state.
- Product tests can focus on user outcomes across real authoritative systems.

### Accepted costs

- Cross-system operations are not atomic.
- Some UI state must be queried or tested live.
- Administrators may see owner-specific partial failures instead of one
  synthetic transaction result.
- Upstream behavior and supported interfaces constrain Soda.
- Some convenient custom pages and workflows will disappear.
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
- Bundled Forgejo remains available, while external canonical repositories
  remain supported without being mirrored into Forgejo automatically.
- Repository authority follows the canonical host.
- External provider administration is outside the initial Soda contract.
- One Linux user represents one human.
- Linux administrator status is authoritative for Soda administration.
- Each local collaborator receives an independently writable, person-owned
  checkout.
- Personal workspaces and private homes default to owner-only.
- Human Git credentials are isolated per Linux user and never shared through a
  project account or project directory.
- Local filesystem access and repository-host access are separate grants.
- A project collaborator is a transient, faceted view, not a stored Soda domain
  entity; untested or unavailable repository capabilities are `unknown`, not
  denied.
- Cross-system operations expose partial completion and support safe
  re-execution rather than simulated distributed transactions.
- Transient aggregation and presentation models are allowed; persistent shadow
  authority is not.
- A minimal Soda project descriptor is allowed only for irreducible
  associations that cannot be derived.
- Ordinary Unix separation protects users from peers, not from root or trusted
  administrators.
- Authoritative mutable user, repository, Tailnet, and machine state survives
  normal bootc image replacement.
- The current implementation is not a compatibility contract. Removing its
  mirrors does not authorize deletion of authoritative user data.

## Decisions still open

- Initial owner creation and Tailnet enrollment before SSH and Cockpit are
  reachable.
- Creation or selection of the initial Forgejo administrator.
- Correlation of Linux and bundled Forgejo identities without a Soda person
  database.
- The privilege path from a Linux administrator to required Forgejo operations.
- Whether ordinary users may create projects or manage collaborators.
- The exact workspace root and descriptor location.
- Whether project Unix groups are universal, optional, or unnecessary.
- Which local project resources, if any, are shared among collaborators.
- SSH agent forwarding versus protected server-side per-person Git keys.
- Whether any project service account remains necessary.
- The smallest project-selection SSH helper, if one remains necessary.
- The exact Cockpit package and privileged-operation boundary.
- The observable Git capabilities required for externally hosted projects.
- Safe revocation order when local and canonical-host changes cannot be atomic.
- Whether a revoked user retains, loses, archives, or transfers their checkout.
- Person deletion while preserving or transferring repositories and workspaces.
- Project deletion and rename behavior.
- Whether the minimal descriptor requires a database or only declarative files.
- Whether a long-running `sodad`, protobuf, or gRPC boundary survives.
- The retained toolchain user outcome and selected upstream boundary.
- The minimum Soda-specific update policy and interface above bootc.

These choices are resolved from product behavior and verified upstream
capabilities, not from the shape of the current code.

## Product-level acceptance criteria

The reset is complete only when the resulting product demonstrates the common
criteria and the criteria for each supported repository mode.

### Common criteria

1. A fresh image can establish its first administrator and join a Tailnet
   without publicly exposing Soda services.
2. Existing Linux accounts can be selected without importing them into a Soda
   user database.
3. Alice and Bob connect through OpenSSH as their own Linux users.
4. Each receives an independently writable, person-owned checkout for the same
   project.
5. Under the selected permission policy, neither can write the other's checkout
   or access the other's private home, credentials, or active agent socket.
6. A supported direct change in authoritative Linux or Forgejo
   state—including a Linux change made through Cockpit—is reflected by Soda
   without import into shadow state.
7. A normal bootc image update preserves Linux users, homes, repositories,
   workspaces, Tailscale identity, and other retained machine-specific state.
8. The same product-level acceptance scenarios pass on x86-64 and AArch64,
   subject only to explicitly documented architecture-specific limitations.

### Bundled Forgejo scenario

1. Existing Forgejo accounts and repositories can be selected without being
   imported into a Soda user or repository database.
2. Soda grants and revokes supported repository access through the reviewed
   Forgejo integration and reports the resulting Forgejo-native authorization
   state.
3. Alice and Bob can push distinct branches and collaborate through Forgejo's
   review mechanism.
4. Re-running a partially completed provisioning or revocation operation is
   safe and reports the observed Linux and Forgejo state separately.

### External canonical repository scenario

1. Soda designates the external repository as canonical and creates or removes
   only the applicable local workspaces and shared-local-resource access.
2. For a requested clone, fetch, or push, Soda uses the person's available
   credentials and reports the exercised capability as successful, denied,
   failed for another reason, or `unknown` when untested or unavailable.
3. Soda does not infer external membership, grant or revoke provider access, or
   promise access to the provider's review mechanism.
4. Soda does not perform an undisclosed push or create other remote state to
   test capability; any non-mutating check is explicit and reports only what it
   exercised.
5. Re-running a partially completed local provisioning or revocation operation
   is safe and reports local results separately from observed Git capabilities;
   provider authorization remains externally administered.

## Review order and outputs

Resolve the ownership reviews in dependency order:

1. [#33: Linux identity and administrator authority](https://github.com/LevitateOS/soda-os/issues/33)
2. [#35: local project and workspace model](https://github.com/LevitateOS/soda-os/issues/35)
3. [#37: canonical repository-host authority and identity correlation](https://github.com/LevitateOS/soda-os/issues/37)
4. [#36: OpenSSH workflow after identity and workspace paths are known](https://github.com/LevitateOS/soda-os/issues/36)
5. [#32: Cockpit composition and privilege path](https://github.com/LevitateOS/soda-os/issues/32), followed by [#34: retained telemetry outcomes](https://github.com/LevitateOS/soda-os/issues/34)
6. [#24: retained toolchain behavior](https://github.com/LevitateOS/soda-os/issues/24) and [#38: retained update policy](https://github.com/LevitateOS/soda-os/issues/38)
7. [#39: retained runtime state and process boundaries](https://github.com/LevitateOS/soda-os/issues/39) as the capstone after the preceding reviews remove or justify its inputs

Issue #35 owns whether a descriptor exists, its location and lifecycle, the
workspace root, optional Unix groups, workspace visibility, filesystem
resources, and checkout lifecycle. Issue #37 owns canonical repository
references, repository-host mode semantics, optional Forgejo repository
identifiers, repository identity validation, Git-host authorization, and
collaboration. Neither issue may create a third Soda membership to simplify its
half of the boundary.

Issue #32 establishes the administration surface before #34 decides which
Soda-specific telemetry outcomes, if any, remain. Issue #39 is not evaluated
against requirements that the preceding reviews are expected to remove.

Each review produces the same outputs:

- required user outcome;
- authoritative owner;
- irreducible Soda-owned fact, if any;
- retained adapter or privileged boundary;
- components and schemas to delete;
- failure and safe re-run behavior; and
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
