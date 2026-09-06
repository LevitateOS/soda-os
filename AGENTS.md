# Soda OS agent guidance

## Product and documentation authority

Read [the product contract](docs/product-contract.md) before changing behavior.
It owns the approved release-day outcomes; do not reproduce that specification
here. [The documentation map](docs/README.md) separates public instructions,
implementation references, operations, evidence, and research.

**Public documentation ships with Soda OS and describes the approved finished
release, not the present checkout.** Implementation gaps, temporary restrictions,
unverified mechanisms, and qualification blockers belong in internal docs and
issues. Do not weaken launch documentation to match missing code, or invent
features to fill a gap. Explicit future-only decisions, including x86-64 WSL2,
remain future-only. Screenshots must match the release interface.

A later explicit user decision overrides stale plans, documentation, and tests.
Classify constraints before using them:

1. Product aspiration or explicit requirement.
2. Established user-facing behavior.
3. External protocol/platform requirement.
4. Current implementation choice.
5. Temporary development constraint.
6. Unresolved decision or unverified hypothesis.

Only the first three normally constrain a new design. Fedora versions, paths,
locks, package sets, schemas, available machines, and historical evidence are
not permanent product rules. Tests prove behavior; they do not create authority.
When authorized behavior changes, update implementation, tests, and docs together.

## Human ownership

Prefer a small, coherent system a person can understand, modify, and delete:

1. Delete dead, duplicated, or unnecessary decisions.
2. Replace multiple representations with one direct representation.
3. Separate genuinely independent responsibilities through explicit inputs and
   outputs.

Do not move branches into arbitrary helpers, assertion utilities, parameter
bags, or vague packages to satisfy structural metrics. Avoid speculative
compatibility, migrations, policy frameworks, generic subsystems, fallback
paths, and future-proofing. Add machinery only for an explicit requirement,
established contract, external protocol, reproduced failure, or concrete
correctness/data-loss concern.

Guard against both subsystem takeover and inverse overcollapse: preserve Soda's
accepted coherent user workflow, while leaving native identity, repositories,
permissions, processes, tools, and deployments with their upstream owners.
A narrow adapter may mutate authoritative native state or the accepted catalog;
it does not justify durable jobs, retries, copied authority, or reconciliation.
Examples such as Podman are not selected architectures. Trusted teams do not
imply hostile multitenancy or an enterprise policy system.

Before proposing a bridge, inspect the exact shipped upstream version and
configuration. Distinguish required behavior, defaults, optional features,
packaging conventions, and hypotheses. If a native mechanism fails, report the
exact constraint; do not silently change product behavior or construct a new
subsystem. An engineering verification is not a new product decision.

## Authorization and working method

Before edits, confirm checkout, branch, Git state, and requested scope; inspect
callers, tests, relevant contracts, and source owners. Preserve unrelated work.
State unknowns rather than manufacturing requirements. A plan, suggestion,
review, or discussion is not authorization to implement.

When direction changes, replace the abandoned implementation directly. Add no
compatibility for abandoned local state unless preservation is explicitly
required. Move code when its responsibility changes; update imports, tests, and
links without forwarding packages, aliases, duplicate files, or compatibility
directories. Avoid vague buckets such as common, utils, or services unless a
concrete cohesive owner is demonstrated.

Continue through ordinary authorized engineering failures. Stop at genuine
product-decision, privilege, persistence, data-safety, credential, matching-
hardware, or uncontrolled-cost boundaries—not arbitrary attempt counts.

### Standing commit authorization

This records the user's operational instruction; repository text does not create
authority. For completed, verified work directly authorized within its exact
scope, create clean logical commits by default without asking separately.
Commit focused completed milestones as they are reached; do not leave completed
work only in a disposable worktree. Inspect the full diff first and preserve
unrelated user work.

This authorizes commits only, not push, pull requests, merges, publication,
deployment, releases, registry mutation, destructive cleanup, or history rewriting.
Those operations require their own explicit instructions. Convenience scripts
and documentation do not authorize their side effects.

## Source navigation and commands

[Architecture](docs/architecture.md) owns the code map. `cmd/AGENTS.md` governs
Go code under `cmd`. Inspect `scripts` and `justfile` before assembling manual
artifact commands. In particular:

- `scripts/check-native.sh <architecture>` runs the existing Linux source gate;
  on Darwin it creates a disposable matching-native Linux check environment.
- `scripts/prepare-native-iso-candidate.sh <architecture>` prepares and
  **publishes** a native OCI candidate, then builds its installer ISO.
- `scripts/place-libvirt-iso.sh` separately places an existing ISO for Linux/libvirt.

Read prerequisites and side effects before execution. See
[development](docs/development.md) and [build operations](docs/build-and-release.md).

## Quality and evidence

Scripts and `justfile` own repository verification. Do not duplicate their numeric
limits here or weaken, suppress, or bypass a gate to finish work.

- Run focused tests while changing a responsibility.
- Run `just check` before completion through the appropriate Linux environment.
- Run relevant race tests for concurrent runtime or persistence changes.
- Run relevant artifact/acceptance checks when those areas change and separately
  authorized prerequisites are available.
- Report source checks, builds, artifact inspection, installed behavior, and
  release qualification separately. A command exit is not proof of its intended
  side effect; inspect the actual native state.

The gates may evolve through an explicit tooling decision, not incidental
metric pressure. Do not remove established capabilities to simplify a count.
For documentation renewal, reduce duplication and obsolete prose, not necessary
procedures, warnings, evidence, or license notices; do not game line wrapping.

## Architecture-specific work

AArch64 and x86-64 are equal siblings. Keep each architecture's preparation,
dependency resolution, builds, artifact generation, inspection, signing,
publication, installation, and validation on matching hardware. Coordination
from a sibling computer is allowed only by executing that work remotely on the
matching target; do not substitute emulation or claim sibling artifact proof.
Architecture-static source checks are not artifact execution or platform support.

Record the affected architecture, performed work, sibling repetition required,
and prerequisites/blockers. Keep machine details as handoff context, not product
requirements. Start platform changes from the product aspiration, identify the
truly platform-specific inputs, share genuinely common behavior, and use explicit
platform-owned locks/artifacts. Do not build a generic platform framework before
real platforms demonstrate the boundary. Compilation alone proves no support.

## Acceptance credentials

Use only the inputs the current runner actually consumes. A fixture that needs
a Tailscale auth key may use an operator-owned protected reusable ephemeral key;
do not demand a fresh non-reusable key for each disposable guest without a user
or concrete external security requirement. Pass it only through established
secret-file and anonymous-descriptor boundaries. Never print or retain it; log
the guest out during cleanup. Describe a reusable key truthfully, not as one-use.
Do not expose secrets through argv, tracing, terminal echo, logs, or evidence.
Clean up only exact run-owned resources; preserve operator inputs and evidence.

## htmx skills

Use installed htmx 4 skills only for their applicable workflow:

- `htmx-guidance`: writing/reviewing Cockpit htmx markup and interactions.
- `htmx-debugging`: failed requests, swaps, events, or runtime behavior.
- `htmx-extension-authoring`: creating, modifying, or debugging extensions.

## Handoff

Report changed ownership/behavior, preserved capabilities, restrictions removed
or revised, verification actually exercised, remaining unverified work, and exact
Git operations. Never present a temporary implementation restriction as a Soda
product principle.
