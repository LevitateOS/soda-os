# Cockpit UX design and integration

## Direction

Quiet by default, helpful when needed. A person should recognize the current
state and next useful action without reading an operating-system manual.
The approved Projects prototype is now integrated and retired. Production tests
exercise the accepted journeys; no parallel preview implementation remains.

| Level | Content |
| --- | --- |
| At a glance | Item name, accurate status, next useful action |
| Active task | Current inputs, prerequisites, progress, or recovery |
| On request | Optional settings, background explanation, technical diagnostics |

Use PatternFly and stock Cockpit navigation. Establish one primary action;
keep management actions in a labelled Actions menu. Essential prerequisites,
destructive consequences, and unresolved partial results must not be hidden in
optional details. A quiet screen must not be an ambiguous one.

Use short forms, field-associated errors inside the active dialog, and useful
keyboard focus. Keep feedback with its task rather than duplicating it behind
a modal. Routine success does not need another modal or permanent banner.
Working states name the operation without inventing percentages, duration,
cancellation, or background-continuation guarantees.

A successful mutation and a failed follow-up read are different facts. Retire
obsolete read errors after recovery, but preserve unresolved partial outcomes.
Unavailable data establishes neither readiness nor absence.

## Integrated Projects journeys

| Journey | Production behavior |
| --- | --- |
| Empty catalog | One addition action; no empty table |
| Ordinary list | Compact project/status/actions; contextual setup and secondary management |
| Add/edit | Short form, arbitrary optional JSON metadata, immutable ID and canonical URL |
| Personal key needed | Read-only prerequisite inspection; stock Accounts handoff |
| Git access needed | Actual retained outbound key; no blanket diagnosis of clone failures |
| Existing account | Setup not confirmed until native inspection establishes readiness |
| Ready | Inspected SSH username and checkout path; browser hostname; copyable command |
| Unknown setup outcome | Inspection before retry; no automatic mutation retry |
| Removal | Native people/account/home selection, exact confirmation, stopped-task and data-loss warnings |
| Partial removal | Confirmed, uncertain, and unattempted outcomes; fresh review before another confirmation |

Workspace orchestration belongs to `ProjectsWorkspaceDialog` and `useWorkspace`.
The native `inspect` action checks only the caller's own derived account and
existing keys/checkout, without requiring administrator status. Catalog listing
remains unprivileged and does not inspect private homes. Inspection creates no
accounts, keys, or checkouts and makes no Git-host requests.

Checkout inspection verifies an owned working-tree root and readable HEAD or a
valid unborn branch. It does not assert a clean tree, Git-host access, or complete
historical object integrity; it neither refreshes the index nor runs filters.
Read problems are not interpreted as absent files. Page-local readiness snapshots
are discarded on catalog refresh; there is no persisted completion flag.

**Set up for me** preflights before mutation. **Review setup**, **Connection
details**, and **Check setup** inspect only. Failed setup preserves the actual
retained key and independent diagnostics. A completed command with failed
verification is not presented as verified readiness.

### Removal and recovery

`ProjectsRemovalDialog` and `useRemoval` own transient removal interaction.
The existing narrow helper supplies `removal-inspect` with `{action,target}`;
its preview includes Linux usernames, UIDs, homes, native associations, catalog
presence, and a scope revision. Inspection reuses native preflight checks and
the shared operation lock without stopping processes or changing accounts/files.

Deletion requires that revision as `expected`, alongside `id` for workspace or
project removal, or `username` for person removal. Execution reauthorizes and
rechecks native identities and the selection under the exclusive operation lock;
project removal also holds the catalog lock. Changed scope stops before mutation.
Cosmetic catalog metadata changes do not change repository identity. Revisions
are stateless comparisons, not credentials, saved approvals, or permissions.

Workspace/person selection stays with its native domain owner. The helper owns
execution and catalog ordering; `linuxhost.DeleteAccounts` records sequential
results. Person removal deletes workspaces before the primary account. Own
workspace removal stays caller-scoped; whole-project and person removal remain
administrator-only. Generic Cockpit/Linux deletion remains non-cascading.

A failed deletion may already have stopped processes or removed account/home
data. Structured receipts distinguish confirmed removals, the uncertain failed
account, unattempted accounts, and catalog outcomes. The privileged helper
returns the receipt; the public coordinator emits it before exiting nonzero for
incomplete removal. The browser decodes stdout, never English stderr, to recover
these results. Missing or malformed receipts remain unknown.

Partial results appear before further confirmation fields. **Review remaining
removal** performs a fresh read and requires exact confirmation again. Unknown
outcomes require an explicit **Check current state**. Neither action retries
deletion. An absent or replaced failed identity/home blocks further deletion in
that task: an administrator must inspect prior local data, not assume it gone.
Orphaned homes and missing-primary cascades are not repaired by inspection.

Mutation receipts and catalog-read errors stay independent. Earlier account
labels and homes are retained only within the current task for recovery details;
no account database, deletion history, or durable browser workflow is added.
Canonical repositories and Git-host accounts are never deleted by these actions.
See [Data safety and removal](public/40-Operate-Soda-OS/30-data-safety-and-removal.md).

## Other pages: foundations completed

- Project and runner mutation outcomes are separate from list-read failures.
  Retained runner creation exits when native listing confirms the account,
  without erasing the failure or retaining registration credentials.
- Inactive provider fields are disabled; local runner lifecycle progress is named.
- Tailscale read, mutation, and Forgejo errors remain independent. Forgejo repair
  has an explicit retry; routing feedback does not claim provider approval.
- Updates retains the installed image during download, invalidates failed
  readback, and distinguishes downloaded data from verified deployment status.

These preserve native authorization, Git/provider ownership, credential clearing,
and explicit update activation. Full page redesigns remain separate milestones.

## Repeatable browser evidence

With the locked toolchain and Playwright Chromium already installed:

```sh
vp -C cockpit build
SODA_PROJECTS_BROWSER_EVIDENCE_DIRECTORY="$PWD/.artifacts/projects-removal-ux" \
  vp -C cockpit test tests/projects-browser.test.ts
```

The test renders the production bundle with simulated Cockpit responses. All
page requests are intercepted locally; no native commands, provider requests,
credentials, or installed guest are used. It checks light/dark themes at 1440px
and 390px, modal focus, clipboard contents, validation, and removal recovery,
and writes screenshots and `observations.json`. The ordinary suite skips this
explicit browser run; component/protocol tests always run. Fixture sources are
excluded from installed bundles.

Source checks, relevant Go race tests, and production-browser simulations have
passed on x86-64. Earlier setup evidence has 44 states in
`.artifacts/projects-native-ux/`; current repeatable evidence is in
`.artifacts/projects-removal-ux/`. Neither proves installed behavior.

## Remaining milestones and evidence

1. Redesign Runners, then Tailscale, then Updates in focused increments. Keep
   provider jobs provider-owned, routing optional, and download separate from
   confirmed restart.
2. Complete comprehensive accessibility and representative-user validation.
   Screenshots and component tests alone do not establish usability.
3. Run separately authorized destructive installed acceptance on disposable
   x86-64 and AArch64 targets: actual account/home deletion, process termination,
   repository preservation, and primary-last deletion. Matching-native AArch64
   runtime/browser evidence must also be reproduced; source checks do not replace it.

Inspect complete diffs, commit verified milestones, and run focused tests,
relevant race tests, and `just check`. No deployment, publication, or live
deletion was performed for this integration.

## PatternFly references

- [Content design](https://www.patternfly.org/content-design/best-practices)
- [Button hierarchy](https://www.patternfly.org/components/button/design-guidelines)
- [Forms](https://www.patternfly.org/components/forms/form/design-guidelines)
- [Expandable sections](https://www.patternfly.org/components/expandable-section/design-guidelines)
- [Contextual alerts](https://www.patternfly.org/components/alert/design-guidelines)
- [Empty states](https://www.patternfly.org/components/empty-state/design-guidelines)
- [Modals](https://www.patternfly.org/components/modal/design-guidelines)
- [Wizard usage](https://www.patternfly.org/components/wizard/design-guidelines)

These inform design, not claims of usability validation. Native contracts
constrain correctness; old wording and visual prominence are implementation
choices, not permanent product rules.
