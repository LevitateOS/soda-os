# Cockpit frontend development

Soda's Projects, Runners, Tailscale, and Updates pages share one Vite+ project in
`cockpit/`. React owns transient presentation. Each page has its own typed native
adapter; the installed Cockpit browser API and existing native executables own
sessions, privileges, operations, and authoritative state.

## Build-host setup

The pinned toolchain is Vite+ 0.3.0, Node 24.20.0, pnpm 11.25.0, React 18.3.1,
and PatternFly 6.6.1. Install the CLI with `scripts/install-cockpit-toolchain.sh`
on each build host; this explicitly installs Vite+ and configures its shell
integration. Reopen the shell or add Vite+'s reported bin directory to PATH.
The installer itself is fetched from a fixed upstream commit. CI uses a pinned
`setup-vp` action with explicit Vite+ and Node versions.

From the canonical checkout:

```sh
vp -C cockpit install --frozen-lockfile
vp -C cockpit check
vp -C cockpit build
vp -C cockpit test
just check
```

`just cockpit-check` runs the four frontend commands. `just check` includes them
before Go tests that inspect built assets. Direct `soda-image rpm` and `oci`
commands also install locked frontend dependencies and build before RPM staging.
That frontend invocation has a ten-minute deadline, including dependency installation.
The release executor uses these same entry points; provision its matching-native
hosts with the pinned toolchain before running a release.

Vite+ owns Oxfmt, Oxlint, strict type checking, and Vitest. No separate formatter,
linter, or test runner configuration is needed. Pure tests use Node; component
tests use jsdom and React Testing Library. Frontend tests also rebuild production
assets and compare file hashes. Generated output and dependencies are untracked.

## Projects browser journeys

The approved Projects prototype is integrated and retired. See the
[UX design and evidence](cockpit-ux-design.md) for current behavior and remaining
milestones. Render the production bundle with simulated native responses using:

```sh
vp -C cockpit build
SODA_PROJECTS_BROWSER_EVIDENCE_DIRECTORY="$PWD/.artifacts/projects-removal-ux" \
  vp -C cockpit test tests/projects-browser.test.ts
```

This explicit Playwright test uses local intercepted assets and response fixtures,
not a native backend or installed acceptance. It saves screenshots and request
observations; no credentials or disposable guest are needed.

## Source and package ownership

See [Cockpit feature state](cockpit-state.md) for action lookup, lifecycle rules,
and the decision to defer an additional read-cache library.

The source uses atomic layers first. PatternFly supplies most primitives and
larger compositions: import its buttons, inputs, forms, modals, tables, alerts,
and layouts directly instead of wrapping them merely to populate an atomic layer.

| Source owner | Responsibility |
| --- | --- |
| `src/atoms/` | Soda eyebrow, wrapping code values, and external links |
| `src/molecules/` | Shared heading, diagnostic and confirmation compositions; feature-specific field groups and summaries |
| `src/organisms/{projects,runners,tailscale,updates}/` | Complete sections and dialogs with explicit data and callbacks |
| `src/templates/` | Sidebar-free Cockpit page layout with content, actions, feedback, and dialog slots |
| `src/pages/` | Store subscription, browser lifecycle, input boundaries, and component composition |
| `src/{projects,runners,tailscale,updates}/` | Stable entrypoints, scoped Zustand stores, native adapters, protocols, types, and pure presentation helpers |
| `src/cockpit/` | Shared Cockpit API types and minimal Soda CSS |

`ProjectsPage`, `RunnersPage`, and `TailscalePage` replace the former `App.tsx`
components. `UpdatesPage` follows the same composition. Interaction tests live
beside the pages. Native protocol, status, and stream tests remain with their
feature owners.

Updates organisms own the installed-image, tracked-source, and pending-deployment
sections, with one explicit Update and restart action. Its feature molecules
own operation feedback and native output presentation. The page connects them
to `updates/store.ts`, which owns requests, bounded streaming output, lifecycle
retirement, and native-state refresh without release/approval state. Status-read
errors are independent of command/check outcomes; focus refresh never erases an
unrelated failure. Long image identities reuse `CodeValue`; notices reuse
`DiagnosticAlert`, while errors retain a separate summary and detailed diagnostic.

Passive organisms and molecules never invoke Cockpit or import native adapters.
Feature organisms and molecules may use their own types and pure presentation helpers;
shared components have no feature dependencies. Pages connect their own feature
state owner to the template and organisms. Each page uses its feature's
`store.ts` for transient observations, named actions, dialogs, and outcomes.
Its entrypoint constructs the store with the native adapter; the page subscribes
and binds its lifetime. There is no cross-page application store or generic
operation controller. Layers may skip levels and use PatternFly directly. For example,
`ProjectActions` composes PatternFly `Button` and `Flex`; `CatalogProjectDialog`
uses PatternFly `Modal` and `Form` with Soda's `CatalogFields`.

Use direct imports, keep feature-specific components under their feature within
an atomic layer, and do not import sibling features or higher layers. The
source-boundary test parses the actual TypeScript/TSX imports (including type
imports and re-exports) through the locked TypeScript API and resolves relative
paths. It checks these boundaries, store creation/subscription ownership, and
each entrypoint's own page/native/store wiring.

Browser state remains transient. Create a fresh feature store for each page
instance and test. Store actions enforce their own pending/confirmation guards;
disposal invalidates continuations, not native outcomes. No persistence or
middleware is used. DOM input/focus stays in the view; registration provider
selection and command outcomes belong to the Runners store. Runner registration
secrets stay in the native input, are cleared immediately after synchronous
request serialization, and are cleared on teardown.

Tailscale's store owns one observer and obtains a fresh adapter for each activation.
The page binds Cockpit visibility/pagehide; hiding closes HTTP/subprocess handles
and clears transient authentication state, while reopening reloads native state.
Closing handles does not prove that native work was undone. Reads do not overlap.
A preference write invalidates reads begun before or during it;
post-write readback retires those reads before releasing draft protection.
Command, read, and Forgejo errors remain independent. Forgejo refresh is attempted
once per connected identity, with explicit retry after failure. No completion
state, daemon, or privileged bridge is added.

PatternFly owns spacing, typography, responsive layouts, modal focus, and theme
behavior. Soda CSS is limited to identity, long values/diagnostics, and the page
surface. Keep sections, native guidance, and product actions in their existing
order. Removed onboarding Setup code stays removed; the workspace **Set up for
me** action remains in Projects.

One build configuration runs four isolated browser environments. Vite's emitted
HTML is relocated from its source subdirectory to each installed package root;
its generated asset references are adjusted correspondingly. Every package is
checked for package-local HTML, JavaScript, CSS, and font references. The only
external runtime asset is Cockpit's installed `../base1/cockpit.js`.

`dist/soda-projects` and `dist/soda-runners` are staged as complete directories
into the Projects and Runners RPMs. `dist/soda-tailscale` and `dist/soda-updates`
are both staged into the Runtime RPM.
No JavaScript tooling is installed on the appliance. React and PatternFly are
compiled assets. Cockpit theme inputs and their licenses are pinned under
`cockpit/vendor`; fonts come from the locked PatternFly dependency. Every package
includes the bundled dependencies' license notices.

After a native RPM build, compare extracted RPM payloads with the actual generated
assets through the same test lifecycle. Use the builder image produced by that
build and absolute paths for its RPM and evidence directories:

```sh
SODA_COCKPIT_RPM_DIRECTORY=/absolute/path/.artifacts/rpms \
SODA_COCKPIT_RPM_BUILDER=soda-os-rpm-builder:0.6.3-aarch64 \
SODA_COCKPIT_EVIDENCE_DIRECTORY=/absolute/path/evidence \
  vp -C cockpit test tests/rpm.test.ts
```

Use the x86-64 builder on the x86-64 host. The test verifies the executing
architecture and RPM headers before extraction, then compares every runtime file
and hash, and records source, lock, builder, and RPM identities. The normal source
suite skips this test until the native RPM directory is explicitly supplied.

Projects has one state owner in `projects/store.ts`: catalog observations,
workspace inspections, and the current task. Setup starts from an explicit action,
not a dialog mount effect. Both workspace views consume the same keyed inspection.
Removal eligibility lives in `projects/ui.ts` and is enforced by the store as well
as displayed by the view. Prior attempted identities remain with the receipt;
fresh previews never rewrite that evidence. Task dialogs in `organisms/projects`
are passive. No parent/dialog refresh callbacks or separate task stores remain.

## Branding verification

The canonical palette lives in `assets/branding/theme/palette.css`, shared with
Forgejo. Cockpit's mappings live in `cockpit/src/cockpit/theme.css`. The native
branding stylesheet imports installed copies of both; each Soda package bundles
those same sources. Do not duplicate color literals in frontend adapters or copy
`preview.css` into a package. Production asset tests verify the symbol and tokens
in all four bundles. Go tests check the RPM asset allowlist and native-login
boundary; `tools/render-cockpit-branding` verifies icon freshness and contrast.

For reproducible login-DOM evidence, obtain the **installed**, compiled Cockpit
366 `login.html`, `login.css`, and `login.js` from
`/usr/share/cockpit/static/` on a matching-native reference host. Copy only those
public static files, not its configuration or credentials, into a local ignored
reference directory. Upstream Git's `login.js` is a build input, not the installed
script. Keep Cockpit package-version evidence with the reference. Then run:

```sh
SODA_COCKPIT_LOGIN_REFERENCE="$PWD/.artifacts/cockpit-login-reference" \
SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY="$PWD/.artifacts/cockpit-branding-implementation" \
  vp -C cockpit test tests/branding-browser.test.ts
```

This opt-in test serves the unmodified installed login files through Playwright
routing, injects the normal environment/base placeholders, and serves local Soda
branding. HTTP authentication responses are simulated: this is **not** installed
login or PAM evidence. It tests explicit and automatic light/dark themes, live
system-theme changes, desktop/narrow layouts, a prefixed URL root, image loading,
computed button colors, password visibility, keyboard focus, failed-login text,
and authentication conversations. Reference-file hashes accompany screenshots.
No new server, authentication implementation, or runtime JavaScript is shipped.

For real integration, provision a disposable matching-native guest as described
below and run the additional suite:

```sh
SODA_COCKPIT_TARGET_FILE=/absolute/path/target.json \
  vp -C cockpit test tests/branding-installed.test.ts
```

It verifies public branding payloads, real login/logout, RPM ownership, the
native guest architecture, shell accents, all four Soda page identities/themes,
narrow layouts, and read-only visits to stock pages. It does not deliberately
submit incorrect passwords, change accounts, check/download updates, or restart.
The other installed/acceptance suites still own privilege escalation and product
operation evidence. Run them as well; this test does not replace them.

Keep source checks, reference-DOM simulations, native RPM inspection, and
installed-session evidence distinct. Integration remains unverified on any
architecture without a matching-native guest run. The image build adds the native
branding link to the five stock entry points that omit it upstream. Guest checks
verify those links; the palette covers surfaces and links as well as accents.
Administrator `/etc/cockpit/branding/` precedence should also be checked in a
disposable guest before shipping.

For read-only signed-in color checks without a password, an operator can run
native `cockpit-ws --local-session=/usr/bin/cockpit-bridge` as their own user,
bound strictly to loopback, and relay it through their SSH connection. Never
expose this passwordless test endpoint to the LAN or Tailnet. Stop the temporary
server and relay afterwards; do not change Cockpit authentication or SELinux
policy for this test. With that endpoint available on the driver's loopback:

```sh
SODA_COCKPIT_COLOR_URL=http://127.0.0.1:19091 \
SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY="$PWD/.artifacts/cockpit-stock-colors" \
  vp -C cockpit test tests/stock-colors-browser.test.ts
```

This suite uses real native pages, no CSS/HTML interception or synthetic backend.
It verifies rendered shell text/background, cards, primary actions, and links in
both themes, including Logs, Services, Terminal, hardware, firewall and Metrics.
It does not prove PAM login or privilege escalation; the credential-based suite
above owns those checks. Terminal ANSI output colors remain terminal semantics.

### Shared-palette CSS reference proofs

For `tests/shared-theme-browser.test.ts`, place public, installed native CSS in a
local reference directory: `cockpit.css` (Cockpit's expanded `overview.css`,
including cards and controls), `forgejo.css` (Forgejo's `index.css`),
`theme-forgejo-light.css`, and
`theme-forgejo-dark.css`. Record the native package versions and file hashes.
Do not copy server configuration or credentials. Then run:

```sh
SODA_THEME_REFERENCE="$PWD/.artifacts/shared-palette/reference" \
SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY="$PWD/.artifacts/shared-palette/proofs" \
  vp -C cockpit test tests/shared-theme-browser.test.ts
```

This opt-in suite renders real native CSS with source adapters on a component
sheet. It checks both applications' canvas, links, filled buttons, hover/pressed
states and narrow layouts, explicit modes against the opposite system preference,
and automatic mode. It uses fallback fonts and a fixture-controlled Cockpit theme
class: it is color evidence, not installed layout, native theme-switching JS, login,
or backend evidence. The installed suites above still own those boundaries.

## Installed browser acceptance

Run this separately on disposable matching-native Soda installations with the
built RPMs installed. Source tests do not establish installed behavior.
Install the locked Playwright Chromium browser on the test driver with
`vp -C cockpit exec playwright install chromium`.

Create an operator-owned JSON target file with `url`, `username`, `passwordFile`,
`architecture` (`x86_64` or `aarch64`), and an absolute `evidenceDirectory`.
The password stays in the protected file named by `passwordFile`; never put it
in command arguments or evidence. Run:

```sh
SODA_COCKPIT_TARGET_FILE=/absolute/path/target.json \
  vp -C cockpit test tests/installed.test.ts
```

The suite logs in through Cockpit, opens Projects, Runners, and Tailscale, checks
native API availability, RPM ownership, actual installed asset hashes, themes,
responsive layouts, dialog focus, refresh, and Tailscale reopening. It captures
screenshots only after login and without entering provider registration secrets.
It writes a source/architecture/package record only after passing.

Updates has a separate, read-only installed smoke test with narrower prerequisites
and evidence; see [Soda Updates](cockpit-updates.md). It reads native status only;
setting its existing target variable cannot update or reboot a guest. It does
not substitute for RPM, theme, responsive-layout, or full update acceptance.
The separately opted-in mutating update/reconnect journey remains pending #61.

Complete mutating acceptance on disposable fixtures as well: project addition,
metadata editing, failed and successful setup with native Git key registration,
workspace/project/person deletion; both runner providers' registration and
lifecycle; and native Tailscale enrollment, exit-node settings, advertisement,
Forgejo refresh, and cancellation. Use approved provider credentials and existing
acceptance secret boundaries. For installation independence, repeat each page's
load with the other Soda frontend directories absent in the disposable guest;
retain native service dependencies. Restore only those test directories afterward.
These scenarios must be recorded as unverified when their prerequisites are absent.

Record the exact source revision and asset/RPM digests for each architecture.
Neither a local browser preview nor a successful RPM file-list check substitutes
for real Cockpit authentication and native integration evidence.
