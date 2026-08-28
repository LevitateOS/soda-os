# Soda OS agent guidance

## Purpose

Soda OS aims to be an understandable, human-owned operating system for remote
development. It should provide a direct path from installing a machine to
creating projects, admitting collaborators, and entering attributed personal
workspaces through SSH and Cockpit.

Prefer a small, coherent product over accumulated flexibility, speculative
machinery, or architecture that only its authors can understand.

## Direction and current implementation

Treat the repository as a snapshot of the product’s current implementation,
not as a permanent definition of what Soda OS is allowed to become.

The intended primary architecture is x86-64. The current implementation targets
AArch64 because that is the available development and validation hardware.
Do not interpret current AArch64 checks, locks, artifact names, or release code
as a product decision against x86-64.

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

Preserve established authenticated product behavior unless the requested change
explicitly revises it. This currently includes:

- persistent people, projects, memberships, and attributed workspaces;
- authenticated Cockpit account and project management;
- SSH shells, direct commands, SFTP, and per-person session state;
- device-key attribution and project access projection;
- provisioning, retry, rollback, and reconciliation;
- immutable-image construction, installation, inspection, and signed releases;
- explicit administrator-controlled OS update staging and activation.

This list describes existing capabilities, not a prohibition against evolving
their contracts or implementation.

The trusted-LAN, authenticated happy path is the current MVP operating model.
Do not add Internet-scale, enterprise, attacker-first, or multi-path machinery
without a concrete requirement. Do not assume that the current MVP deployment
model must remain permanent either.

## Current source ownership

Use the present tree as a navigation aid, not an immutable architecture:

- `cmd`: executable-specific construction and command behavior
- `cockpit`: Cockpit and PAM executables, daemon client, HTTP views, and assets
- `internal/build`: image, installer, and release production
- `internal`: runtime domain, daemon, host, persistence, SSH, updates, telemetry,
  toolchains, and process execution
- `distro`: current distribution specification, profiles, locks, and base inputs
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
- keep generated protobuf files generated rather than editing them manually;
- do not weaken, suppress, or bypass a gate merely to finish a change.

The gates themselves may evolve through an explicit tooling decision. Their
current values are not permanent product constraints.

Distinguish source checks, artifact builds, live installation, and observed
user behavior. Report only the level of evidence actually exercised.

## Architecture and platform changes

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
