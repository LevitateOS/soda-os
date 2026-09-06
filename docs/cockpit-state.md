# Cockpit feature state

## Where to start

Each installed page has one instance-scoped Zustand store. To investigate an
interaction, follow its named action in the feature's `store.ts`, then its native
adapter. State, pending guards, task transitions, and command/readback sequencing
live together; pure predicates remain in the feature's `ui.ts` or `status.ts`.
Native helpers still authorize, revalidate, and perform operations.

| Feature | State/action owner | Useful starting actions |
| --- | --- | --- |
| Runners | `cockpit/src/runners/store.ts` | `register`, `changeListener`, `remove` |
| Updates | `cockpit/src/updates/store.ts` | `refresh`, `check`, `update` |
| Projects | `cockpit/src/projects/store.ts` | `open`, `setupWorkspace`, `checkRemoval`, `remove` |
| Tailscale | `cockpit/src/tailscale/store.ts` | `signIn`, `applyExitNode`, `applyAdvertisement`, `retryForgejo` |

The feature entrypoint supplies native dependencies and creates the store. The
page subscribes with `useStore`, binds lifecycle events, and passes explicit data
and actions to presentational components. There is no application singleton,
Context registry, hook compatibility layer, or generic operation controller.
The former six coordination hooks have been removed.

Each store's adjacent `store.test.ts` exercises actions without React. Page tests
exercise the same real stores with controlled native responses. Start there to
reproduce a failure rather than constructing a component just to inspect a task.
For a test, create a fresh store and call `start()`; its return value is cleanup.
Construction alone starts no reads or commands. Cleanup retires continuations;
it does not establish whether native work completed. Tailscale additionally
closes its native handles and constructs a fresh adapter on reactivation.

## What stays separate

- Native snapshots are observations, not authority or a browser database. A
  failed read cannot establish absence, readiness, or successful activation.
- A command acknowledgement and its readback can disagree or fail independently.
  Refreshing an observation does not erase an unrelated command failure.
- Projects shares keyed workspace observations between catalog and task views.
  Refresh invalidates observations, not removal receipts. Partial/unknown removal
  requires renewed explicit review; exact confirmation is enforced by the action.
- Updates holds native observations, not a reviewed release selection. Check is
  informational; Update follows bootc's source at operation start. Completion or
  disconnection is not boot proof; reload/focus/refresh rereads the booted digest.
- Ordinary form buffers, disclosure, input elements, and focus stay in components.
  Task-significant choices such as the registration provider belong to the store.
- Runner registration tokens never enter observable state. The submit boundary
  passes a transient typed payload, the adapter serializes synchronously, and
  input/payload clearing happens immediately, including failure. Do not put this
  path into deferred mutation execution, persistence, or logging middleware.
- Tailscale drafts are protected from polling. Preference writes invalidate older
  reads and perform fresh post-write readback instead of accepting pre-write preferences.
  Authentication, preference commands, read errors, and Forgejo refresh are
  distinct concerns within that feature, not separate competing stores.

The ownership goal is fewer places to find decisions, not a promised reduction
in lines, bundle size, or render time. Whether this organization is personally
easier to maintain remains an owner review, not something tests can establish.

## Milestone 5: residual read-layer review

**Decision: retain Zustand-only ownership for now; do not add SWR or TanStack
Query in this migration.** This is an implementation decision, not a permanent
restriction on Soda OS.

The residual read bookkeeping is concrete and bounded:

| Observation | Current coordination that a read library would need to preserve |
| --- | --- |
| Runner list | One page consumer; explicit refresh and readback after both successful and failed commands |
| Update host | Initial/focus reads; unavailable host blocks Update; status errors remain independent of check/update outcomes |
| Project catalog and workspace | Explicit refresh and keyed invalidation; one shared readiness observation; read-only checks never imply setup |
| Removal preview | Explicit renewed review, exact current revision, and attempt-local receipts/account identities; never automatic destructive retry |
| Tailscale status/preferences | One non-overlapping observer; draft/write ordering; native-client lifetime and streaming authentication; conditional Forgejo refresh |

There is no second cache to reconcile and no demonstrated multiple-consumer read
problem. SWR could supply ordinary read deduplication/revalidation, but would not
remove the command sequencing, reviewed intent, error separation, or Tailscale
write ordering. Introducing it now would add another owner while the new store
layout is still awaiting human use and review. No speculative integration or
cache abstraction has been added.

Revisit a read library if a concrete read-coordination problem remains after use.
A bounded trial should replace the chosen snapshot and its loading/read-error
ownership, not copy SWR results into Zustand. Task intent, command outcomes,
confirmation, and secrets must not become query/mutation cache entries. Require
the existing failure, ordering, and lifecycle tests to keep passing and show what
actual bookkeeping disappears before expanding the trial.

## Verification and remaining evidence

On x86-64, the migration passed focused headless store and real-page tests and
`just check`, including all four self-contained production bundles, deterministic
rebuilds, source boundaries, and bundled Zustand MIT attribution. Regression
coverage includes the Updates focus/error defect and Tailscale pre-save/during-
save poll ordering. Projects' explicit production-bundle browser journeys also
passed using intercepted native responses, not a live backend.

This does not establish installed behavior, a performance improvement, or owner
acceptance. AArch64 must reproduce frontend checks and browser journeys on
matching hardware with the locked toolchain and browser prerequisites. Installed
native verification on both architectures still requires separately authorized
disposable systems; no provider registration, Tailscale enrollment, account
deletion, or update activation was exercised on a real host for this migration.
See [Cockpit development](cockpit-development.md) for current verification commands
and [Cockpit UX design](cockpit-ux-design.md) for the separate UX work.
