# Refine Soda ownership and decision discipline

**Status:** Governing interpretation and implementation discipline.

**Supersedes:** `20260831T084503+0200-refine-subsystem-takeover-rule.md`

## Document role and authority

The [base principles](principles.md) explain Soda's product purpose and human
ownership philosophy. The [architectural reset](architecture-reset.md) defines
the accepted product architecture, ownership boundaries, issue order, and
acceptance criteria. [AGENTS.md](../AGENTS.md) governs repository work,
authorization, and evidence. The live issues own bounded implementation,
deletion, and verification work.

This document coordinates how those authorities are interpreted. It does not
create product behavior, select unverified mechanisms, or compete with the
accepted architecture. If a later explicit user decision conflicts with this
document, record the conflict and treat this document's affected direction as
stale.

## Core failure modes

Guard against two symmetric mistakes:

1. **Subsystem takeover:** one discrepancy expands into a Soda-owned remake of
   Linux or an upstream subsystem.
2. **Inverse overcollapse:** accepted Soda product behavior is deleted merely
   because upstream tools provide the individual primitives.

Soda legitimately owns its accepted installable composition, conventions,
focused presentation, minimal project catalog, irreducible associations, and
narrow transitions. It must not turn those into a general daemon, database,
RPC surface, job engine, credential store, copied authority, or reconciliation
loop.

## Two separate gates

### Product-outcome gate

A user-facing postcondition comes only from an explicit user decision or the
accepted product contract. Repository contents, upstream documentation,
protocols, failures, examples, and implementation convenience do not create
new product promises.

### Implementation-necessity gate

Repository evidence, exact-version upstream documentation, protocol
constraints, reproduced failures, and concrete correctness or data-loss
concerns may constrain the implementation or justify a narrow mechanism. They
do not manufacture adjacent product outcomes.

Start with the exact accepted postcondition and identify the authoritative
owner of every resulting fact. Inspect the exact Soda path and exact shipped
upstream versions and configuration before proposing custom behavior. Prefer
configuration, packaging, installer inputs, native commands, and narrow
adapters. A possible discrepancy is not evidence of a gap.

## Decision-state discipline

Classify every material conclusion as one of:

- explicit product requirement;
- accepted architectural decision;
- recommended option awaiting a decision;
- implementation hypothesis awaiting verification;
- engineering verification; or
- rejected or superseded direction.

Do not promote a plausible or upstream-supported mechanism into an accepted
decision. Do not reopen, replace, or weaken an accepted decision without
identifying the conflict and obtaining a new user decision. If a later user
decision conflicts with a document or issue, identify that source as stale
rather than continuing to treat it as authority.

Product decisions materially change user-visible outcomes, ownership,
persistent product data, privilege boundaries, destructive semantics,
supported workflows, or trust promises. Bounded questions about exact upstream
support, PAM rules, paths, commands, or persistence are normally engineering
verification. Keep an unproven mechanism in its owning issue; state only the
accepted outcome normatively.

## Interpretation discipline

Examples, possible tools, analogies, and phrases such as "for example" are not
requirements or selected mechanisms. In particular, do not infer:

- that mentioning Podman selects it as Soda's isolation architecture;
- that "isolation" means hostile-project, hostile-team, or enterprise tenancy;
- that "full setup" requires a credential broker;
- that destructive removal requires archive, transfer, rollback, or recovery;
- that multi-user operation requires enterprise identity; or
- that a convenient workflow must cover every provider or installation path.

Only direct consequences of an accepted requirement are in scope. Each extra
guarantee needs its own evidence and decision.

## Narrow adapters and irreducible Soda state

A narrow adapter has no private durable workflow state. It may mutate an
accepted irreducible Soda-owned fact, such as the project catalog, or state in
an authoritative upstream system, such as a Linux account, home, SSH keys, or
Git checkout. It must not retain job status, retry lifecycle, projections,
reconciliation records, copied authority, or a generalized resource model.

Do not add adjacent machinery merely because a demonstrated bridge exists.
Every lifecycle, identity, persistence, retry, recovery, policy, or
presentation responsibility must be independently required by the accepted
product contract.

A minimal declarative catalog of genuine Soda facts is legitimate. A
generalized configuration schema that mirrors or manages Linux accounts,
Forgejo permissions, clone status, container state, or operation lifecycle is
not. Smallness is a consequence of correct ownership, not a line-count goal.

## Exact upstream verification

When architecture depends on upstream behavior, verify the exact version and
configuration Soda ships. Distinguish explicitly between:

- a mandatory upstream requirement;
- an upstream default;
- an optional upstream feature;
- a Soda packaging convention; and
- an unverified implementation hypothesis.

Never turn an optional setting, recommendation, or fallback into a product
requirement or a claimed upstream limitation.

## Failure and issue discipline

If the smallest implementation hypothesis fails, return to the product
decision. The failure may justify another native path, an accepted visible
limitation, an explicit change to the supported workflow, or a narrowly
evidenced bridge. It does not authorize a subsystem that preserves convenience
at any cost.

Do not create an architecture domain or architecture issue for every technical
discrepancy. A separate architectural decision is warranted only when the
alternatives materially change product behavior, authority, persistent state,
privilege, destructive semantics, or the long-term component model. Otherwise,
keep the work as bounded verification in the issue owning the accepted outcome.

Safe re-execution should normally inspect authoritative state and use
idempotent upstream operations. Partial failure may remain visible and
operator-retriable. It does not independently justify durable jobs, retries,
compensation, reconciliation, callbacks, or monitoring.

Acceptance tests prove behavior. They do not create product authority or
justify runtime state.

## Symmetric stopping tests

### Authority test

After the supported operation succeeds, can upstream systems own the resulting
identities, repositories, permissions, processes, sessions, and deployments
without a Soda mirror, synchronization loop, or workflow record?

The desired answer is yes.

### Product-preservation test

After removing the proposed Soda component, does the accepted Soda user outcome
still exist as a coherent, discoverable, supported, and repeatable appliance
workflow, or has the user been returned to manually assembling the upstream
tools?

If removal destroys an accepted Soda outcome, retain the smallest
product-specific composition that provides it.

## Bootstrap incident

The assistant turned the installation result "create a Linux administrator,
install the required ordinary credentials, and enroll Tailscale" into a
bootstrap contract, generalized bundle, multiple transports, first-boot
operation, retry semantics, runtime verification, documentation set, and
architecture issue before demonstrating an upstream gap.

The correct default hypothesis was native installer composition plus acceptance
testing, with no persistent Soda runtime authority. A bounded installer hook or
one-shot adapter is discussable only after the exact supported path demonstrates
a concrete limitation, and it must leave no private Soda bootstrap state.

## Project-environment incident

The assistant first concluded that Soda needed no project catalog, Projects
page, or project workflow because Linux, Git, Forgejo, and Cockpit supplied the
individual operations. That erased an accepted Soda outcome instead of merely
removing shadow authority.

It then overcorrected by treating Podman, mentioned only as an example, as the
isolation mechanism, adding Dev Containers, and inventing a malicious-project
threat model.

The correct discipline is to preserve the accepted Soda-specific discovery and
workspace-onboarding outcome, use the accepted derived Linux workspace-account
model, and treat alternative tools and formats as candidates until explicitly
accepted and verified.

## Governing principles

Existing systems remain authoritative for the facts they naturally own. Soda
owns only its accepted composition, conventions, presentation, irreducible
associations, and narrow transitions.

Product requirements come from the user and accepted product contract.
Technical evidence constrains implementation but does not manufacture new
product outcomes.

Examples are not selected mechanisms. Candidates are not decisions.
Engineering verifications are not product architecture.

Custom code stays at the edge. Tests prove behavior rather than create
authority. Deletion must preserve accepted user outcomes. The reset rejects
both shadow authority and the inverse removal of Soda's legitimate product.
