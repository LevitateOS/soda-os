# Soda Updates

The administrator-only **Soda Updates** Cockpit package implements the approved
workflow: discover the latest published stable Soda release, verify its signed
architecture-specific record and exact OCI image, download that digest, and
explicitly apply and restart. Candidate tags are not an update channel.

## Ownership and commands

- `cockpit/src/updates`, `cockpit/src/pages/UpdatesPage.tsx`: presentation and
  transient interaction state, using the existing React/PatternFly build.
- `/usr/libexec/soda/soda-updates` (`cmd/soda-updates`): synchronous root-only
  `status`, `check`, `download`, and `apply` operations. Cockpit uses its native
  administrative-access boundary; the executable is not setuid and installs no
  privileged service or generic bridge.
- `internal/updates`: published-release verification and native bootc invocation.
- `soda-runtime`: packages the executable and static Cockpit assets and requires
  native bootc, Skopeo, and Cosign executables.

Native bootc owns deployments. There is no Soda deployment database, update
service, timer, automatic update, account snapshot, or browser workflow cache.
`status` and `check` emit JSON; download/apply stream native progress. A page
reload recovers the downloaded/staged deployment from bootc, not browser state.

## Release verification

Checks query GitHub's `releases/latest`, not registry tag sorting. They require a
published, non-prerelease `vMAJOR.MINOR.PATCH` and exactly one host-architecture
record and Sigstore bundle. Cosign verifies the existing production-workflow
certificate identity and issuer. Schema 3, version/platform/channel, source
revision, and exact Soda GHCR digest must agree. Image signature and provenance
verification use that digest, followed by anonymous Skopeo identity inspection.
Temporary record/bundle files are removed on both success and failure.

No release, transport failure, invalid signature, and mismatched image are not
"up to date". Stable versions compare numerically; development versions and
same-version/different-digest installations are not automatically replaced.

Download and Apply reverify the selected *published version*, not whatever
became latest in the meantime. A removed release or changed digest fails closed.
Apply does not trust release details held in the browser, including after reload.

## Download and activation

Download uses `bootc switch --download-only EXACT_VERIFIED_DIGEST`. It refuses
an existing staged deployment, downgrade, incompatible image, queued rollback,
or transient `/usr` overlay. Its resulting digest and `downloadOnly` state are
checked before success is reported.

Apply rereads and verifies the exact staged target, runs
`bootc switch --from-downloaded`, checks the resulting target and unlocked state,
then requests a normal `systemctl reboot`. The confirmation explicitly warns
that SSH sessions and development workloads will be interrupted. A failed
restart request leaves the enabled-for-next-restart deployment visible. A
connection loss never establishes success or failure: reconnect and refresh.

**Coordinate administration during Apply.** Bootc 1.16.10 rejects an expected
target with `--from-downloaded`; there is no atomic compare-and-activate argument.
The helper's ephemeral `/run/soda-updates.lock` serializes Soda mutations only,
not ordinary bootc or OSTree commands. Before/after checks detect changed native
state but cannot eliminate a concurrent administrator race. The UI warns not to
run other deployment commands during Apply. If a post-activation check fails,
Soda does not request reboot and warns that native pending state may already
have changed; inspect bootc before *any* restart. There is no compensating
rollback or claim that a Soda-only lock protects native CLI operations.

Direct rollback, rollback-based cancellation, arbitrary image switching,
automatic-update controls, and deployment deletion/pinning are outside this
page. Native CLI administration and the handbook's account-preserving fallback
remain available.

## Evidence and remaining prerequisites

Source/unit/browser tests exercise selection, verification, command ordering,
stale targets, failures, recovery after reload, confirmation, and privilege
requirements. They do not prove a live upgrade across versions.

The operator-provided ephemeral **x86_64** VM runs Soda 0.6.3 and bootc 1.16.10.
Native inspection confirmed privileged JSON status, Skopeo availability, missing
Cosign, a masked automatic-update timer, and clean refusal to activate without a
staged deployment. The expected-target/`--from-downloaded` conflict was reproduced
without changing its deployment. GitHub's latest Soda release endpoint returned
404 during this implementation: there is no approved release to exercise the
full positive discovery/update path at that checkpoint.

Before producing a release image, resolve/lock the verification dependencies on
matching-native hardware. The runtime RPM now requires Cosign; the previously
installed image did not provide it. Native x86_64 and AArch64 artifact builds and
full update/restart/account-preservation acceptance remain independently needed.
Never claim sibling artifact validation from mocked architecture-selection tests.
