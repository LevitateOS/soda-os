# Cockpit frontend development

Soda's Projects, Runners, and Tailscale pages share one Vite+ project in
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

## Source and package ownership

The source uses atomic layers first. PatternFly supplies most primitives and
larger compositions: import its buttons, inputs, forms, modals, tables, alerts,
and layouts directly instead of wrapping them merely to populate an atomic layer.

| Source owner | Responsibility |
| --- | --- |
| `src/atoms/` | Soda eyebrow, wrapping code values, and external links |
| `src/molecules/` | Shared heading, diagnostic and confirmation compositions; feature-specific field groups and summaries |
| `src/organisms/{projects,runners,tailscale}/` | Complete sections and dialogs with explicit data and callbacks |
| `src/templates/` | Sidebar-free Cockpit page layout with content, actions, feedback, and dialog slots |
| `src/pages/` | Feature hook and component composition for each registered page |
| `src/{projects,runners,tailscale}/` | Stable entrypoints, feature hooks, native adapters, protocols, types, and pure presentation helpers |
| `src/cockpit/` | Shared Cockpit API types and minimal Soda CSS |

`ProjectsPage`, `RunnersPage`, and `TailscalePage` replace the former `App.tsx`
components. Their interaction tests live beside the pages. Native protocol,
status, and stream tests remain with their feature owners.

Visual components never invoke Cockpit or import native adapters. Feature
organisms and molecules may use their own types and pure presentation helpers;
shared components have no feature dependencies. Pages connect their own feature
hook to the template and organisms. Hooks own transient requests, refresh, and
lifecycle handling; there is no shared application store or generic operation
controller. Layers may skip levels and use PatternFly directly. For example,
`ProjectActions` composes PatternFly `Button` and `Flex`; `CatalogProjectDialog`
uses PatternFly `Modal` and `Form` with Soda's `CatalogFields`.

Use direct imports, keep feature-specific components under their feature within
an atomic layer, and do not import sibling features or higher layers. The
source-boundary test parses the actual TypeScript/TSX imports (including type
imports and re-exports) through the locked TypeScript API and resolves relative
paths. It checks these boundaries and each entrypoint's own page/native wiring.

React state remains transient. Runner registration secrets stay in the native
input, are cleared immediately after synchronous request serialization, and
are cleared on teardown. Tailscale's feature hook retains streaming auth URLs,
non-overlapping reads, unsaved form edits, hidden-page cancellation, native state
recovery on reopening, and conditional Forgejo refresh. Component composition
adds no completion state, daemon, or privileged bridge.

PatternFly owns spacing, typography, responsive layouts, modal focus, and theme
behavior. Soda CSS is limited to identity, long values/diagnostics, and the page
surface. Keep sections, native guidance, and product actions in their existing
order. Removed onboarding Setup code stays removed; the workspace **Set up for
me** action remains in Projects.

One build configuration runs three isolated browser environments. Vite's emitted
HTML is relocated from its source subdirectory to each installed package root;
its generated asset references are adjusted correspondingly. Every package is
checked for package-local HTML, JavaScript, CSS, and font references. The only
external runtime asset is Cockpit's installed `../base1/cockpit.js`.

`dist/soda-projects`, `dist/soda-runners`, and `dist/soda-tailscale` are staged as
complete directories into the Projects, Runners, and Runtime RPMs respectively.
No JavaScript tooling is installed on the appliance. React and PatternFly are
compiled assets. Cockpit theme inputs and their licenses are pinned under
`cockpit/vendor`; fonts come from the locked PatternFly dependency. Every package
includes the bundled dependencies' license notices.

After a native RPM build, compare extracted RPM payloads with the actual generated
assets through the same test lifecycle. Use the builder image produced by that
build and absolute paths for its RPM and evidence directories:

```sh
SODA_COCKPIT_RPM_DIRECTORY=/absolute/path/.artifacts/rpms \
SODA_COCKPIT_RPM_BUILDER=soda-os-rpm-builder:0.6.2-aarch64 \
SODA_COCKPIT_EVIDENCE_DIRECTORY=/absolute/path/evidence \
  vp -C cockpit test tests/rpm.test.ts
```

Use the x86-64 builder on the x86-64 host. The test verifies the executing
architecture and RPM headers before extraction, then compares every runtime file
and hash, and records source, lock, builder, and RPM identities. The normal source
suite skips this test until the native RPM directory is explicitly supplied.

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

The suite logs in through Cockpit, opens all three registered packages, checks
native API availability, RPM ownership, actual installed asset hashes, themes,
responsive layouts, dialog focus, refresh, and Tailscale reopening. It captures
screenshots only after login and without entering provider registration secrets.
It writes a source/architecture/package record only after passing.

Complete mutating acceptance on disposable fixtures as well: project addition,
metadata editing, failed and successful setup with native Git key registration,
workspace/project/person deletion; both runner providers' registration and
lifecycle; and native Tailscale enrollment, exit-node settings, advertisement,
Forgejo refresh, and cancellation. Use approved provider credentials and existing
acceptance secret boundaries. For installation independence, repeat each page's
load with the other two Soda frontend directories absent in the disposable guest;
retain native service dependencies. Restore only those test directories afterward.
These scenarios must be recorded as unverified when their prerequisites are absent.

Record the exact source revision and asset/RPM digests for each architecture.
Neither a local browser preview nor a successful RPM file-list check substitutes
for real Cockpit authentication and native integration evidence.
