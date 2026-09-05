# Cockpit UX design checkpoint

## Direction

Quiet by default, helpful when needed. A person should recognize the current
state and next useful action without reading an operating-system manual.
The Projects prototype completed the first, review-gated milestone of the UX
overhaul. Its design is approved for implementation; it does not establish a
new product workflow or permission model.

| Level | Content |
| --- | --- |
| At a glance | Item name, accurate status, next useful action |
| Active task | Only the current inputs, prerequisites, progress, or recovery |
| On request | Optional settings, background explanation, technical diagnostics |

Use the installed PatternFly components and stock Cockpit navigation. Establish
one primary call to action in a view; row actions use less prominent styling.
Keep ordinary management actions in a labelled Actions menu. Essential
prerequisites, destructive consequences, and unresolved partial results must
not be hidden in optional details. A quiet screen must not be an ambiguous one.

Use short forms unless a genuinely complex ordered task requires a wizard.
Show field errors at the field, inside the active dialog, with focus and
accessible associations. Keep operational feedback with the affected task.
Do not duplicate the same error in a modal and behind it. Routine success does
not need another modal or a permanent banner. Use descriptive disclosure labels
rather than hiding everything under an unexplained information icon.

Working states name the operation without inventing a percentage, stage,
duration, cancellation promise, or guarantee of background continuation.
A successful mutation and a failed follow-up read are different facts. Retire
obsolete read errors after recovery, but preserve unresolved partial outcomes.
Never infer permissions, readiness, absence, or failure from unavailable data.

## Interactive Projects prototype

From the repository root:

```sh
vp -C cockpit dev --host 127.0.0.1 --port 5173 --strictPort
```

Open <http://127.0.0.1:5173/prototypes/projects/>. This is a local development
preview, not an installed Cockpit package. The top **Design preview** controls
are for reviewers and are not proposed product UI. Choose a scenario or follow
the interactions. The simulated operation takes a short fixed delay solely to
make the working design visible; this is not a proposed runtime estimate.

All accounts, keys, repositories, destinations, and outcomes are examples.
The preview never invokes Cockpit or native operations, registers keys, or
navigates to a real provider. Destination buttons report where the real action
would take the user. Reloading discards the preview's local state.

The prototype lives in `cockpit/prototypes/projects`. Production entrypoints
must not import it, and it must not be included in the four installed bundles.
It reuses the existing React/PatternFly toolchain; it adds no server/backend,
dependency, persistent state, or application framework. Its field checks are
illustrations of validation placement, not authoritative Git URL validation.
Remove this preview when its accepted design is integrated; do not maintain a
second implementation or import prototype state into the product.

### Review journeys

| Scenario | What to inspect |
| --- | --- |
| Empty catalog | One next action; no useless table or filters |
| Not set up | Compact list; setup visible; management secondary |
| Add/edit repository | Short form; optional metadata; immutable identity consequences visible |
| Working | Clear task label, no duplicate submission, no invented completion |
| Personal key needed | Primary-account destination; no confusion with the Git key |
| Git access needed | Retained workspace acknowledged; copyable example key and provider handoff |
| Account exists, clone unconfirmed | Not labelled ready; verify setup before connecting |
| Ready | Connection command and checkout directory on request |
| Unknown outcome | Verify before retrying; no false success or failure |
| Removal | Exact confirmation, affected people/files, preservation and no undo |
| Partial removal | What was deleted and what remains; Close is not cancellation |

Before wider implementation, review these on desktop and a narrow screen,
with keyboard navigation and both themes. Confirm that essential guidance is
reachable without opening technical details, and that normal use is not a
wall of text. Observe representative users without coaching before calling
the design understandable. Screenshots and component tests alone do not
establish that conclusion.

The prototype concentrates on one example project and its workspace lifecycle.
The example viewer, Alice, has administrative access.
Adding a repository replaces that single example; it is not a complete catalog.
Key-registration and connection outcomes remain illustrative GitHub/website
fixtures even when the form's example name or address is edited.
The primary-account deletion flow, runner registration, Tailscale routing, and
OS updates remain subsequent design/implementation work. No shipped capability
is removed or changed by this checkpoint.

### Repeatable browser evidence

With the locked Playwright Chromium already installed, run:

```sh
SODA_PROJECTS_PREVIEW_EVIDENCE_DIRECTORY="$PWD/.artifacts/projects-ux-preview" \
  vp -C cockpit test tests/projects-preview-browser.test.ts
```

The test starts and stops its own loopback-only Vite server. It verifies theme
changes, 1440px and 390px layouts, keyboard focus, the simulated setup journey,
clipboard copying, and visible form validation. It blocks and checks for
external requests, confirms no Cockpit API is present, and saves screenshots
plus `preview.json`. The ordinary suite skips this explicit browser-evidence
run when the evidence directory is not supplied; component tests always run.
No native credentials or disposable VM are required for the preview test.

## Native facts required after design approval

Account existence and complete-clone readiness must remain separate native
facts. The current `workspace_exists` boolean cannot establish readiness. Any
new read/outcome information belongs in the existing narrow native boundary,
not a frontend completion cache. Authorization failures must be distinguished
from unrelated clone failures before presenting a key-registration remedy.

Deletion summaries must use current native facts and be revalidated at
execution. Repository preservation and separate Forgejo account deletion remain
unchanged. A page close cannot undo completed native mutations. Prototype
transitions are deliberate fixtures, not implementations of these guarantees.

## Integration status

The prototype is approved. The first source integration milestone fixes the
interaction foundations across the existing pages:

- Projects validation is inside its dialog and focuses the invalid field.
- Project and runner mutation outcomes are separate from list-read failures;
  failed mutations re-read native facts, and recovered reads retire only read
  errors. A retained runner leaves the creation dialog when the native list
  confirms it, without erasing the failure or retaining the registration token.
- Inactive provider fields are disabled, and runner lifecycle actions identify
  the operation in progress. Closing a removal error is not called cancellation.
- Tailscale read errors recover independently of mutation and Forgejo refresh
  failures. Forgejo repair has an explicit retry, not automatic polling retries.
  Routing forms show scoped saving/saved feedback without claiming approval.
- Updates keeps the installed image visible during downloads, invalidates facts
  after failed status reads, and distinguishes download completion from failed
  readback. Missing data does not diagnose administrative permissions.

These changes preserve native authorization, Git ownership, exact destructive
confirmation, credential clearing, and explicit update activation. They do not
yet integrate the approved Projects composition or supply native clone-readiness
facts. Keep the review prototype separate until that integration replaces it.

## Remaining implementation milestones

1. Implement approved Projects setup, connection, removal, and Accounts handoffs.
2. Apply the design to Runners, Tailscale, and Updates, one focused milestone at
   a time. Keep routing optional, provider jobs provider-owned, and download
   separate from confirmed restart.
3. Align documentation and complete keyboard, theme, responsive, error/recovery,
   native integration, and representative-user validation.

Inspect full diffs and commit completed verified milestones. Run focused tests
throughout, relevant race tests for concurrent changes, and `just check` before
completion. Installed and mutating acceptance requires independently authorized
disposable x86-64 and AArch64 targets, matching-native operations, and approved
provider credentials. The prototype proves neither installation nor native
operations on either architecture. No live system changes or publication are
part of this checkpoint.

## PatternFly references

- [Content design](https://www.patternfly.org/content-design/best-practices)
- [Button hierarchy](https://www.patternfly.org/components/button/design-guidelines)
- [Forms and progressive disclosure](https://www.patternfly.org/components/forms/form/design-guidelines)
- [Expandable sections](https://www.patternfly.org/components/expandable-section/design-guidelines)
- [Contextual alerts](https://www.patternfly.org/components/alert/design-guidelines)
- [Empty states](https://www.patternfly.org/components/empty-state/design-guidelines)
- [Modals](https://www.patternfly.org/components/modal/design-guidelines)
- [Wizard usage](https://www.patternfly.org/components/wizard/design-guidelines)

These inform the design, not claims of usability validation. Existing native
contracts constrain correctness; old wording, section order, and visual
prominence are implementation choices open to revision through this review.
