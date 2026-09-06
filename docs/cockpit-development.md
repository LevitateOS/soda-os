# Cockpit frontend development

Projects, Runners, Tailscale, and Soda Updates share the Vite+ project in
`cockpit/`. React owns transient presentation; installed Cockpit and each
feature's typed native adapter own sessions, privileges, and native operations.
[Architecture](architecture.md) owns the native boundaries and implementation gaps.

## Build-host setup

Versions live in `cockpit/package.json`, `.node-version`, and the lockfile.
Install `scripts/install-cockpit-toolchain.sh` after reviewing its host changes;
restart the shell or use Vite+'s reported bin directory. CI pins the same tools.

```sh
vp -C cockpit install --frozen-lockfile
vp -C cockpit check
vp -C cockpit build
vp -C cockpit test
```

`just cockpit-check` runs those commands; `just check` includes them before Go
packaging tests. Native RPM/OCI construction also installs locked dependencies
and builds frontend assets, with a ten-minute deadline including installation.
The release executor uses these entry points. See [development](development.md)
for the full Linux source gate and Darwin's native check container.

Vite+ owns formatting, linting, strict type checks, and Vitest. Pure tests use
Node; component tests use jsdom/React Testing Library. Production tests compare
rebuild hashes. Dependencies and `dist` are untracked.

## Source and package ownership

| Layer | Responsibility |
| --- | --- |
| `src/atoms/` | Soda eyebrow, long code values, external links |
| `src/molecules/` | Shared diagnostics/confirmation and feature fields/summaries |
| `src/organisms/{projects,runners,tailscale,updates}/` | Passive sections and dialogs |
| `src/templates/` | Sidebar-free Cockpit layout and explicit slots |
| `src/pages/` | Store subscription, lifecycle, input boundaries, composition |
| `src/{projects,runners,tailscale,updates}/` | Entrypoint, store, adapter, protocol, types, pure presentation helpers |
| `src/cockpit/` | Shared Cockpit API types and Soda CSS |

Use PatternFly components directly rather than wrappers that merely fill an
atomic layer. Keep direct imports and feature-specific components under their
feature; do not import sibling features or higher layers. Passive components
never invoke native adapters. Pages connect their feature's state to components;
entrypoints create fresh stores with native dependencies. Source-boundary tests
parse TypeScript imports/re-exports and check ownership and actual wiring.

Four isolated browser environments compile to self-contained package roots.
Build relocation adjusts HTML asset references; tests check local JavaScript,
CSS, fonts, and notices. Only installed `../base1/cockpit.js` is external.
Projects/Runners RPMs stage their complete corresponding `dist` directories;
Runtime stages Tailscale and Updates. No JavaScript build tooling is installed
on Soda. Pinned vendor themes and all bundled dependency licenses remain included.

## Feature state and lifecycle

| Feature store | Starting actions |
| --- | --- |
| `projects/store.ts` | `open`, `setupWorkspace`, `checkRemoval`, `remove` |
| `runners/store.ts` | `register`, `changeListener`, `remove` |
| `tailscale/store.ts` | `signIn`, `applyExitNode`, `applyAdvertisement`, `retryForgejo` |
| `updates/store.ts` | `check`, `download`, `requestApply`, `apply` |

Start at an action, then adapter and native owner. Each store/test instance is
fresh. `start()` starts observations and returns cleanup; construction alone
runs nothing. Actions enforce pending and confirmation guards themselves.
Cleanup invalidates continuations, not native outcomes. No application singleton,
generic operation controller, persistence, middleware, or cross-page cache exists.

- Snapshots are observations. Failed reads establish neither absence nor readiness.
  A command result and its readback may fail independently; refresh must not erase
  an unrelated command failure or mutation receipt.
- Projects shares keyed inspections across catalog/task views. Setup starts from
  an explicit action, never dialog mount. Refresh invalidates observations, not
  attempted identities or receipts. Removal eligibility is shared in `projects/ui.ts`
  and enforced by the store, not only the displayed button.
- Updates retains reviewed Apply intent independently of later host reads and
  preserves the installed image while downloading. Focus refresh cannot erase
  verification/command failure. Native status must confirm deployment outcomes.
- DOM buffers and focus remain in views; task-significant provider selection is
  store-owned. Runner tokens stay out of observable state. Synchronous request
  serialization is immediately followed by input/payload clearing, including
  failure and teardown. Do not defer this boundary into mutation caching/logging.
- Tailscale owns one non-overlapping observer and a fresh adapter per activation.
  Cockpit visibility/pagehide closes handles and transient authentication state;
  reopening reloads native state. Preference writes invalidate pre/during-write
  reads and perform fresh readback before releasing draft protection. Command,
  read, authentication, and Forgejo errors stay separate. Forgejo refresh is
  attempted once per connected identity, with explicit retry on failure.

Zustand is the current bounded state owner, not a permanent library mandate.
No demonstrated multi-consumer read problem warrants a second cache. If native
use reveals one, a bounded trial must replace existing read/loading/error
ownership rather than copy query results into Zustand. Confirmation, secrets,
intent, and mutation receipts must not become cache entries.

## Interaction rules

Quiet by default, helpful when needed: show name, accurate observation, and
next useful action; put active inputs/progress/recovery with the task, and
technical background on request. Use one primary action and labelled secondary
Actions. Never hide prerequisites, destructive consequences, or partial results.
Field-associated errors, keyboard focus, and native modal behavior matter.
Do not invent percentages, durations, cancellation, or background guarantees.

Projects' accepted journeys are implemented in production, not a parallel preview:

- Empty catalog → one addition action, not an empty table.
- Add/edit → short forms, optional JSON metadata, immutable ID/address.
- Missing personal key → stock Accounts handoff before mutation.
- Retained workspace → inspect actual key/checkout; do not assume clone failure
  means authorization or account existence means ready.
- Ready → inspected username/path and browser-hostname SSH command.
- Unknown setup → inspect before explicit retry.
- Removal → reviewed native scope, exact confirmation, stopped-work/data warnings.
- Partial/unknown removal → preserve receipts, explicitly check/review remaining
  state, require confirmation again, and never infer old home deletion from a
  missing/replaced account. Inspection does not repair orphaned homes.

PatternFly owns spacing, layouts, modal focus, and themes. Soda CSS stays focused
on identity, long values, diagnostics, and the page surface. Removed onboarding
Setup stays removed; Projects' **Set up for me** is a different retained action.
See [PatternFly content design](https://www.patternfly.org/content-design/best-practices)
and its component design guidelines; neither establishes usability evidence.

## Source and simulated browser evidence

With locked Playwright Chromium installed, render the production Projects bundle
with intercepted local native responses:

```sh
vp -C cockpit build
SODA_PROJECTS_BROWSER_EVIDENCE_DIRECTORY="$PWD/.artifacts/projects-removal-ux" \
  vp -C cockpit test tests/projects-browser.test.ts
```

This opt-in suite exercises light/dark, wide/narrow, validation, focus, clipboard,
and removal recovery, saving screenshots and `observations.json`. No provider,
native backend, or credentials are used. Fixture sources do not ship. Headless
store tests and real-store page tests run in the ordinary source lifecycle.

## Branding verification

`assets/branding/theme/palette.css` owns color values; the Cockpit adapter is
`src/cockpit/theme.css`. Native branding and all four pages consume those same
sources. Do not copy preview CSS or color literals into adapters. Source tests
check package allowlists, login boundaries, palette/symbol inclusion, contrast,
and regenerated icons. [Branding](branding.md) owns integration and upgrade rules.

For reference-DOM tests, copy only public installed Cockpit `login.html`,
`login.css`, and compiled `login.js` from `/usr/share/cockpit/static/` into an
ignored reference directory. Record native package version and hashes. Upstream
Git's JavaScript source is not the installed script; never copy credentials.

```sh
SODA_COCKPIT_LOGIN_REFERENCE="$PWD/.artifacts/cockpit-login-reference" \
SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY="$PWD/.artifacts/cockpit-branding-implementation" \
  vp -C cockpit test tests/branding-browser.test.ts
```

The suite serves unmodified native login assets with local branding and simulated
HTTP authentication. It checks explicit/automatic themes, system changes, narrow
layouts, URL prefixes, loading/colors, password visibility, focus, errors, and
authentication conversations. It is not actual PAM or installed-session evidence.

For shared native CSS proofs, retain public installed `cockpit.css` (expanded
`overview.css`), `forgejo.css` (native `index.css`), `theme-forgejo-light.css`, and
`theme-forgejo-dark.css`, with package versions/hashes:

```sh
SODA_THEME_REFERENCE="$PWD/.artifacts/shared-palette/reference" \
SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY="$PWD/.artifacts/shared-palette/proofs" \
  vp -C cockpit test tests/shared-theme-browser.test.ts
```

This component sheet checks theme/color states with fallback fonts and a fixture
Cockpit class. It does not prove native theme switching, layout, login, or backend.

A read-only signed-in color check can use native `cockpit-ws
--local-session=/usr/bin/cockpit-bridge` as the operator, **bound only to loopback**
and relayed through their SSH connection. Never expose this passwordless test
endpoint to LAN/Tailnet or alter authentication/SELinux. Stop it and its relay
afterward. With the endpoint already available on the driver's loopback:

```sh
SODA_COCKPIT_COLOR_URL=http://127.0.0.1:19091 \
SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY="$PWD/.artifacts/cockpit-stock-colors" \
  vp -C cockpit test tests/stock-colors-browser.test.ts
```

That suite observes actual native pages, not intercepted HTML/CSS; it proves
colors, not PAM/elevation. Terminal ANSI retains native semantics.

## Native RPM and installed browser acceptance

After a separately authorized native RPM build, use that builder and absolute
paths to compare actual generated files/hashes with extracted RPM payloads:

```sh
SODA_COCKPIT_RPM_DIRECTORY=/absolute/path/.artifacts/rpms \
SODA_COCKPIT_RPM_BUILDER=ACTUAL_NATIVE_BUILDER_IMAGE \
SODA_COCKPIT_EVIDENCE_DIRECTORY=/absolute/path/evidence \
  vp -C cockpit test tests/rpm.test.ts
```

The suite checks executing architecture/RPM headers before extraction and records
source, lock, builder, and RPM identity. Without explicit prerequisites it skips;
source tests alone are not native packaging proof.

Use disposable matching-native installations for browser integration. Install
locked Chromium on the driver with `vp -C cockpit exec playwright install chromium`.
An operator-owned JSON target contains `url`, `username`, `passwordFile`,
`architecture` (`x86_64` or `aarch64`), and absolute `evidenceDirectory`. The password
stays in its protected file, not argv or evidence. Run separately:

```sh
SODA_COCKPIT_TARGET_FILE=/absolute/path/target.json \
  vp -C cockpit test tests/installed.test.ts
SODA_COCKPIT_TARGET_FILE=/absolute/path/target.json \
  vp -C cockpit test tests/branding-installed.test.ts
```

The first checks real login, native API, RPM ownership/hashes, themes, layouts,
focus, refresh, and Tailscale reopening. The second covers login/logout, branding,
all four page identities/themes, stock pages, and native integration. Neither
substitutes for mutating product scenarios. Administrator `/etc/cockpit/branding/`
precedence and branding links on stock entries need disposable guest observation.

Updates has a narrower read-only checkpoint test. With the existing empty-release
feed fixture and disposable passwordless-sudo account, set
`SODA_UPDATES_BROWSER_TARGET` to operator JSON with `url`, `username`, protected
absolute `passwordFile`, and `evidenceDirectory`, then run:

```sh
vp -C cockpit test tests/updates-installed.test.ts
```

It never downloads/applies. Its no-release expectation must be revised when the
feed changes; it does not establish positive signed-release verification. The
operator owns test-account and credential cleanup.

## Remaining evidence

Record exact source, platform, asset/RPM identity, topology, and performed checks.
Earlier x86-64 store/Projects simulations and an Updates overlay preview prove
only their recorded boundaries, not installed usability or a real upgrade.
AArch64 must reproduce corresponding native checks; neither sibling qualifies
the other. [Acceptance](../tests/acceptance/README.md) owns qualification coverage.

Still required on disposable installations: add/edit/setup with real Git key
registration; workspace/project/person deletion and partial outcomes; real jobs
and listener lifecycle for both runner providers; browser Tailscale authentication,
exit nodes, approval, refresh failure and cancellation; verified update/restart/
preservation. Repeat page loading with other Soda frontend directories absent,
retaining native dependencies and restoring only those fixture directories.
Missing credentials/guests mean unverified, not skipped-as-passed.

Runners, Tailscale, and Updates UX follow-up, comprehensive accessibility, and
representative-user review remain separate work. Public release screenshots must
show the final native interface; simulated evidence images are not substitutes.
