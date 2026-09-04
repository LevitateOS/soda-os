# Soda OS agent guidance

## Purpose

Soda OS aims to be an understandable, human-owned operating system for remote
development. It should provide a direct path from installing a machine to using
one primary Linux account per human, one derived Linux workspace account per
human-project pair, bundled Forgejo or an external Git host, ordinary Git,
OpenSSH, and stock Cockpit with one focused Soda Projects page.

Prefer a small, coherent product over accumulated flexibility, speculative
machinery, or architecture that only its authors can understand.

## Durable product contract

- Soda OS is cloud-first, not cloud-only. Trusted local-network installations
  expose OpenSSH, Cockpit, Forgejo, and project-selected development servers
  directly over the LAN. Cloud installations expose those services through
  Tailscale and never to the public Internet. Tailscale must not block LAN
  access. Initial setup requires a physical, VM, or supported cloud console.
- The business or technical owner selecting the team's development
  infrastructure is the primary product decision-maker. Daily users are
  developers connecting from lightweight clients to a powerful shared Soda OS
  development server.
- Remote development and administration are SSH-first. A human connects
  directly through ordinary OpenSSH to their derived workspace account;
  non-interactive SSH, automation, SCP, and SFTP retain normal OpenSSH behavior.
- The browser administration surface is stock Cockpit with Soda branding and
  one Soda Projects package. The package reuses Cockpit authentication and
  sessions and may invoke one narrow synchronous catalog-and-workspace operation.
  It also provides the administrator-only Soda-aware human deletion path;
  generic Cockpit or command-line account deletion is non-cascading. The
  package does not add another web server, authentication layer, daemon,
  database, generic backend, or generic privileged bridge.
- A separate administrator-only Cockpit Runners package operates local Forgejo
  and GitHub runner accounts, clients and systemd listeners through the existing
  narrow helper. Providers own registration, workflows, scheduling and job
  history; Soda adds no CI control plane.
- Launch requires bundled Forgejo alongside support for external Git hosts.
  Repository lifecycle, access, and collaboration stay native to the
  authoritative Git host. Soda retains only a minimal appliance-wide project
  catalog without an independently assumed closed field list, plus the
  human-project-to-workspace-account convention. A successful **Set up for me**
  operation leaves a complete clone
  beneath the derived workspace account's `$HOME/Projects/<repository>` without
  retaining Git credentials or workflow state. Repositories are created through
  the authoritative Git host, then added to the catalog by SSH clone URL. The
  project ID and canonical URL are immutable after addition; replacing the URL
  requires administrator removal of the project and its local workspaces,
  followed by re-adding it. Listing projects and setting up a workspace never
  depend on Tailscale identity. Browser SSH guidance uses the host through which
  the person opened Cockpit.
- Workspace isolation means separate Linux homes, checkouts, user-local
  dependencies, process ownership, and project-local data. Projects select
  non-conflicting host ports themselves. Podman is an optional installed tool,
  not Soda's isolation mechanism or a Soda-managed subsystem.
- `mise` owns development-tool installation, versions, and project toolchain
  configuration. Developers invoke it directly through its native commands and
  repository configuration. Soda owns no tool picker, toolchain installer,
  version manager, downloader, cache format, profile system, shared tool store,
  or parallel state model. Tea and GitHub CLI are available in every workspace;
  each workspace authenticates them manually and separately.
- Linux administrators use native `bootc` commands for explicit update checks,
  staging, activation, and supported fallback. Fallback to an earlier image
  must preserve current Linux account, password, group, and administrator
  state; direct `bootc rollback` is not supported unless verified against that
  invariant. The automatic update timer is disabled, and Soda has no runtime
  update service or shadow deployment state.
- AArch64 and x86-64 are equal sibling architectures. Neither is a default,
  fallback, experimental, or second-class target.
- Human installation uses one completed network ISO, stock graphical Anaconda
  for installation-owned responsibilities, and one common interactive
  Soda Setup shared with QCOW2. There is no human-facing OEMDRV, second
  credential image, cloud-init provisioning path, or public-SSH bootstrap.

### Standing commit authorization

This records the user's operational instruction; repository text does not create
authority. For completed, verified work directly authorized within its exact
scope, create a clean logical Git commit by default without asking separately.
Throughout an implementation, commit focused completed milestones as they are
reached; do not leave completed work only in a disposable worktree.
Inspect the full diff first and preserve unrelated user work. This covers
commits only, not push, pull requests, merges, publication, deployment,
releases, registry mutation, destructive cleanup, or history rewriting.

## Direction and current implementation

Treat the repository as a snapshot of the product’s current implementation,
not as a permanent definition of what Soda OS is allowed to become.

The current implementation history may reflect the development and validation
hardware that was available at the time. Do not interpret AArch64 checks, locks,
artifact names, or release code as a product decision against x86-64.

Likewise, the current Fedora version, bootc base, registry, state schema,
toolchain profiles, package set, filesystem paths, and release flow are current
implementation facts. They may change when the product direction or supported
hardware changes.

Before relying on a constraint, classify it as one of:

1. Product aspiration
2. Established user-facing behavior
3. External protocol or platform requirement
4. Current implementation choice
5. Temporary development constraint
6. Unresolved product decision

Only the first three should normally constrain a new design. Do not promote a
temporary limitation into a permanent rule merely because it appears in code,
tests, documentation, or a build lock.

Tests and documentation are evidence of current behavior. They are not
independent product authority. When an authorized product change makes them
outdated, update them with the implementation.

## Human ownership

Optimize for code that a person can understand, modify, and delete.

Prefer, in order:

1. Delete dead, duplicated, or unnecessary decisions.
2. Replace multiple representations with one direct representation.
3. Separate genuinely independent responsibilities with explicit inputs and
   outputs.

Do not satisfy structural gates by moving branches into arbitrary helpers,
creating parameter bags, hiding conditions behind assertion utilities, or
introducing vague abstraction packages.

Avoid speculative compatibility, migrations, policy frameworks, workflow
engines, generic subsystems, fallback paths, and future-proofing. Add machinery
only for an explicit requirement, an established contract, an external
protocol, a reproduced failure, or a concrete correctness or data-loss concern.

Do not remove established product capabilities merely to simplify a metric.
Contracts may change when explicitly required, but the change should simplify
the product rather than replace one form of complexity with another.

## Product behavior

The architecture reset explicitly replaces pre-reset project, workspace,
dashboard, toolchain, and update control-plane behavior while retaining the
smallest Soda-specific project workflow. The target behavior is:

- one primary Linux account per human and one derived Linux workspace account
  per human-project pair, with Linux authoritative for every account and home;
- stable primary usernames and a Linux-native distinction between primary and
  workspace accounts, without a Soda identity database or rename
  reconciliation;
- Soda Setup creation of the initial Linux/Forgejo administrator and its key;
  later primary humans created through stock Cockpit or Linux, first normal
  Forgejo PAM login, native manual key registration, and no workspace-account
  Forgejo identities or ongoing role synchronization;
- stock Cockpit with Soda branding and one focused Soda Projects page;
- a minimal shared declarative project catalog editable by every primary
  human, without an unapproved closed metadata field list, repository-
  membership model, or capability state; display information and additional
  metadata are editable, while project identity and canonical URL are not;
- synchronous workspace setup whose accepted outcome is a derived account and
  complete clone produced through native user-authenticated Git or repository-
  host behavior without retained credentials; setup requires a key in the
  primary account's standard `~/.ssh/authorized_keys` before mutation and
  copies those public keys once; each workspace's outbound public key is
  registered manually through the authoritative Git host before retrying setup;
  `workspace_exists` reports the derived Linux account even while a failed
  clone remains retryable, and listing or setup does not require Tailscale;
- native repository creation through Forgejo or the external authoritative Git
  host, followed by adding its SSH clone URL to the Soda catalog; changing that
  immutable URL requires administrator removal and re-addition;
- direct ordinary OpenSSH login, commands, SFTP, and process attribution as the
  derived workspace UID, without forced commands or synthetic homes;
- repository lifecycle and access through bundled Forgejo or the external
  authoritative Git host;
- user-owned workspace removal and administrator-only project removal that
  deletes the shared entry and every local workspace, including uncommitted
  work, while always preserving the canonical Forgejo repository;
- administrator-only Soda-aware human deletion that removes derived local
  workspaces first, the Forgejo account second, and the primary Linux account
  last, without a watcher for out-of-band Linux deletion;
- direct `mise`-owned development tools and repository configuration, with
  workspace-private installed dependencies and assistants;
- administrator-controlled native `bootc` operations and an account-preserving
  supported fallback, without claiming direct `bootc rollback` before it is
  verified and without a Soda update service; and
- immutable-image construction, installation, inspection, and signed releases.

Pre-reset databases, copied people and repository records, memberships, shared
project accounts, shared worktrees, device-key projection, standalone dashboard
services, jobs, retries, rollback, reconciliation, toolchain profiles, broad
immutable tool manifests, copied Tea credentials, alternate onboarding paths,
and translated update state are implementation evidence and deletion targets,
not preservation contracts. The catalog, derived workspace-account convention,
Projects page, Soda Setup composition, and narrow synchronous operations are
retained outcomes; they must not become a generic control plane.

Do not add Internet-scale, enterprise, attacker-first, or multi-path machinery
without a concrete requirement. The team is trusted. Initial setup uses the
console; LAN and Tailnet access are both first-class within their approved
deployment contexts.

## Current source ownership

Use the present tree as a navigation aid, not an immutable architecture:

- `cmd`: executable-specific construction and command behavior
- `cockpit`: the static stock-Cockpit Projects package and its presentation assets
- `internal/build`: image, installer, and release production
- `internal`: native Projects behavior, host integration, process execution,
  and artifact construction
- `distro`: current distribution specification, locks, and base inputs; current
  immutable-toolset files are implementation debt, not product authority
- `packaging`: files grouped by the artifact or package that ships them
- `tests/acceptance`: system-level installation and boot evidence
- `scripts` and `tools`: repository verification and developer tooling

Move code when responsibility genuinely changes. Update imports, tests, and
documentation directly. Do not preserve an obsolete layout through forwarding
packages, aliases, duplicate files, or compatibility directories.

Avoid vague buckets such as `common`, `utils`, or `services` unless a concrete,
cohesive owner has first been demonstrated.

## htmx skills

Use the installed htmx 4 skills only when their specific workflow applies:

- `htmx-guidance` when writing or reviewing Cockpit htmx markup, attributes,
  events, swaps, or interaction patterns;
- `htmx-debugging` when htmx requests, swaps, events, or other runtime behavior
  do not work as expected;
- `htmx-extension-authoring` when creating, modifying, or debugging an htmx 4
  extension.

## Working method

Before changing the repository:

- confirm the exact checkout, branch, Git state, and requested scope;
- inspect the relevant callers, tests, contracts, and current source of truth;
- distinguish user requirements from assumptions and repository accidents;
- preserve unrelated or uncommitted work;
- state material unknowns instead of manufacturing requirements.

A plan, suggestion, review, or discussion is not authorization to edit files.
Commit, push, release, deployment, and external operations require their own
explicit instructions.

When the user changes direction, prefer changing the implementation directly.
Do not add compatibility for abandoned local state unless preservation is an
explicit current requirement.

## Quality gates

The scripts and `justfile` are the source of truth for current repository
verification. Do not copy their numeric limits into this document.

For implementation work:

- run focused tests while changing a responsibility;
- run `just check` before completion;
- run the relevant race tests for concurrent runtime or persistence changes;
- run artifact or acceptance checks when those areas change and prerequisites
  are available;
- do not weaken, suppress, or bypass a gate merely to finish a change.

The gates themselves may evolve through an explicit tooling decision. Their
current values are not permanent product constraints.

Distinguish source checks, artifact builds, live installation, and observed
user behavior. Report only the level of evidence actually exercised.

## Architecture and platform changes

Development actively uses both x86-64 and AArch64 computers. Keep every
architecture-specific operation on matching hardware: perform x86-64 input
preparation, dependency resolution, builds, artifact generation, inspection,
signing, publication, installation, and validation on an x86-64 computer, and
perform the corresponding AArch64 operations on an AArch64 computer. An agent
may coordinate from a computer of the other architecture only by executing the
work remotely on the matching target; do not build, inspect, sign, publish,
install, or validate one architecture's artifacts on the sibling architecture.
Whenever a change, dependency, artifact, test procedure, limitation, or
follow-up is architecture-specific, record the affected architecture, what was
done, what the sibling architecture must reproduce or verify, and any known
prerequisites or blockers. Keep temporary machine details as handoff context;
do not turn them into product requirements.

When adding x86-64 or another supported platform:

- start from the product aspiration, not from duplicated AArch64 conditionals;
- identify which inputs are truly platform-specific;
- keep product behavior shared where it is genuinely shared;
- use explicit platform-owned locks and artifacts where they differ;
- avoid constructing a generic multi-platform framework before two real
  platforms demonstrate the required boundary;
- validate the produced image and installation path on the target hardware.

Do not claim platform support from successful compilation alone.

## Handoff

At completion, report:

- what ownership or behavior changed;
- which established capabilities were preserved;
- which current restrictions were removed or revised;
- what was verified;
- what remains unverified;
- the exact Git operations performed.

Never describe a temporary implementation restriction as a permanent Soda OS
principle.
