# Soda Updates

The administrator-only Cockpit page follows native bootc's configured image
source. It has one explicit **Update and restart** action, with a warning that
SSH sessions and development workloads may be interrupted. Check is informational,
not approval of a selected version or digest.

## Ownership and commands

- `cockpit/src/updates`, `cockpit/src/pages/UpdatesPage.tsx`: native observations,
  transient interaction state, and React/PatternFly presentation.
- `/usr/libexec/soda/soda-updates` (`cmd/soda-updates`): synchronous root-only
  `status`, `check`, and `update`. Cockpit requires administrative access; the
  executable is not setuid and adds no service or privileged generic bridge.
- `internal/updates`: native status projection and fixed bootc operations using
  supplied query and progress runners.
- `soda-runtime`: packages the helper and static page. The runtime updater uses
  only bootc. The old Skopeo/Cosign RPM requirements and custom Cosign image
  pipeline remain pending removal after the release/acceptance consumers are
  replaced under issue #61; they are not update prerequisites.

The helper protocol is:

| Operation | Native commands | Output |
| --- | --- | --- |
| `status` | `bootc status --json` | One JSON host projection |
| `check` | `bootc upgrade --check`, then `bootc status --json` | Native check progress on stderr; one JSON host projection on stdout |
| `update` | Read native status for diagnostics, then `bootc upgrade --apply` | Streamed native progress/errors |

Bootc owns configured `spec.image`, digest resolution, cached-update metadata,
deployments, activation, and any restart. Soda does not compare versions, fetch
GitHub releases, verify signatures, select a browser-approved digest, or invoke
`systemctl reboot`. There is no download/apply protocol or confirmation dialog.

The ephemeral `/run/soda-updates.lock` serializes Soda checks and updates, not
ordinary administrator bootc commands. Status remains readable during an
operation. The page serializes in-flight actions, bounds displayed output, and
retires asynchronous callbacks when its lifecycle ends. Native output is never
parsed to determine eligibility or success.

## Native state and update behavior

The page shows the tracked source, actual booted digest, native cached metadata
for the tracked source, and pending deployments. Cached metadata is an observation,
not a live registry guarantee. Metadata may be attached to a staged or booted
entry. A failed Check is not "up to date"; refresh does not erase its error.

Update follows the native source at operation start even if it advanced after
Check. Same-version images and compatible staged/download-only deployments do
not block it. With an unchanged booted image and no pending update, bootc needs
no extra Soda-triggered reboot. The page reports compatibility, read-only system,
queued-rollback, missing-source, and `/usr` overlay diagnostics without replacing
administrator source choices.

Request completion, staging, or disconnection is not boot proof. The page rereads
native status after an operation when reachable and on reload/focus/explicit
refresh after reconnect. Command and readback failures remain distinct. Verify
the actual booted digest before concluding that the update took effect; do not
automatically retry an ambiguous outcome.

## Selected pre-alpha policy and pending publication work

Issue #61 selects one rolling tag per architecture:
`ghcr.io/levitateos/soda-os:dev-x86_64` and
`ghcr.io/levitateos/soda-os:dev-aarch64`, retaining source-revision tags. Matching-native
publication and installed validation are independent for each architecture.
Authorized repository publishers, authenticated publication, HTTPS, and content
integrity are the selected development trust boundary—not production authenticity.
No version bump, signature, GitHub release, installer, or sibling qualification
is required by this runtime updater.

The OCI-only preparation/publication replacement is **not implemented yet**.
Do not treat the existing ISO-candidate wrapper as that new command or assume a
development tag is available. A digest-pinned VM requires a separately authorized
native switch to an available development tag; ordinary upgrade cannot follow a
moving tag while pinned. Soda never performs this switch implicitly. Native
administration and exact-digest fallback remain available; arbitrary downgrade
compatibility and direct `bootc rollback` are not established.

## Verification boundaries

Source tests cover root authorization, fixed command arguments, stream separation,
runner injection, locking, unchanged/same-version/cached/staged facts, native
errors, lifecycle retirement, bounded progress, and disconnect/readback behavior.
They simulate commands and browser responses; they do not prove an installed update.
The replacement passed `just check` and focused Updates Go race tests on native
x86-64. AArch64 must reproduce the source/browser checks on matching hardware;
neither architecture has installed-update evidence for this replacement yet.

`cockpit/tests/updates-installed.test.ts` is opt-in and read-only. Set
`SODA_UPDATES_BROWSER_TARGET` to an operator-owned JSON file containing `url`,
`username`, `passwordFile` (absolute protected file), and `evidenceDirectory`.
The disposable account needs passwordless sudo for stock Cockpit elevation.
Run `vp -C cockpit test tests/updates-installed.test.ts`. It compares displayed
source/digest against native JSON status and saves a screenshot. It never checks
the registry, updates, changes sources, or restarts; the operator owns credential
and account cleanup. Normal source checks without this setting skip it.

The explicitly mutating installed update/reconnect journey remains step 5 of
#61. Both architectures need their own authorized live evidence. No source test
or read-only page smoke establishes account/data preservation across an update.

### Historical preview evidence (old updater)

An earlier x86_64 Soda 0.6.3/bootc 1.16.10 VM preview used a temporary `/usr`
overlay to show the old signed-release page, stock administrative elevation,
installed status, and an empty GitHub release feed. No deployment was staged or
reboot requested. Its temporary account/credentials were removed; at that
checkpoint the overlay was expected to remain until reboot. That preview was
not RPM installation, an update journey, or AArch64 evidence, and does not
validate the replacement described here.
