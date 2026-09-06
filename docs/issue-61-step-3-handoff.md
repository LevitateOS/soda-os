# Issue #61, milestone 3 — paused source checkpoint

## Status and authorization

The owner requested that implementation stop because product direction may
change. This is a saved work-in-progress checkpoint, **not milestone 3
completion**. Do not continue implementation, build the proposed A image, publish,
or mutate a VM without renewed direction. Leave issue #61 step 3 unchecked.

Work began on clean `main` after:

- `8c28c356aaaad7558c90cdc8c9cc65fcca02dfcd`: runtime updater replacement (step 1).
- `85ff2224e9b2dbf6f59b328678232a1e9b6be082`: explicit OCI destinations/results (step 2).

The commit containing this handoff saves the implementation below. It is not yet
an approved planned A source. No push, artifact publication, or installed
qualification is implied.

## Implemented in this checkpoint

- Added `internal/build/oci` as the shared inspector for publication, installers
  and acceptance. It returns in-memory native manifest/config digests, platform,
  version, revision, exact base and RPM-inventory checksum facts. Archive path,
  blob-integrity, platform and inventory-sidecar checks are included. Flattened
  inventory inspection honors whiteouts. Tiny synthetic OCI fixtures are shared
  by source tests in `internal/build/oci/ocitest`.
- Added `image.Builder.PublishImage` and `soda-image publish --archive` using only
  the selected native specification and injected runner. Publication uses the
  archive's source/version/base, not publisher HEAD. It checks/reuses an identical
  revision tag or copies a new one, verifies its digest, advances only the native
  development tag from that exact digest, then verifies anonymously. Conflicts
  and partial failures stop without automatic retry or reconciliation.
- Replaced the old ISO-candidate wrapper with `scripts/prepare-native-image.sh`
  behind `just dev-image <architecture>`. It accepts any clean committed source,
  runs the source gate once, prepares reviewed inputs, builds OCI/RPMs once,
  consumes the actual returned archive path, preserves native runtime identity
  checks and publishes OCI only. Independent ISO/QCOW2 and libvirt placement
  remain.
- Converted acceptance to candidate OCI/ISO/raw-QCOW2 and fallback OCI paths.
  It validates native OCI facts and installer checksum sidecars. Fallback uses
  its own version/base. Added candidate booted-digest readback after ISO and
  QCOW2 startup. Existing B → earlier A → B preservation, fixtures, cleanup,
  diagnostics, partial reports and `RunSummary.Validate`/`Qualify` remain.
  Missing onboarding/LAN/public-ingress observations still cannot qualify.
- Added raw-QCOW2 checksum sidecar output, preserving compressed output/checks.
- Removed the old `cmd/soda-release` and `internal/build/release` packages,
  `soda-image record`, signed/combined acceptance record/verify commands and
  implementations, fallback signature verification and Cosign preflight.
- Removed production release/signing-evidence workflows, the old executor and
  obsolete guards/tests. Updated CI, `justfile` and product-identity checks.
- Removed distribution GitHub-repository and platform release-channel fields
  and their fixture entries. Updated the affected guidance and build/installer/
  acceptance/public instructions; retained historical observations as historical.

The **soda-release RPM**, OS identity/branding, source labels, reviewed native
inputs and native installer inspection are retained. The Cosign package, its
fetch/build chain and current Go cache lifetimes are deliberately unchanged:
those were step 4, not this assignment. No operator artifacts, accounts, registry
tags or Git history were cleaned up.

## Verification actually exercised

On the native x86-64 development host:

- Focused Go tests for OCI, image, installer, acceptance, both affected CLIs and
  scripts passed with `-count=1`.
- `just check` passed, including a final run while winding down after the last
  summary-reader relocation. This includes source formatting/complexity,
  frontend lint/types/build/tests, Go vet/tests, acceptance race checks and
  both architecture-static configuration checks.
- Focused `go test -race -count=1` passed for those same packages before the last
  test-only summary-reader relocation. The final `just check` also exercised
  the acceptance race gate after that relocation.
- `git diff --check` passed.

Publication/wrapper tests use fake runners/commands. OCI fixtures are tiny
synthetic test data, not native Soda artifacts. No Soda OCI/RPM/installer build,
GHCR write, VM update, restart or account-preservation journey was performed.
AArch64 native source reproduction and real artifact/installed evidence on both
architectures remain unverified. Source success does not establish platform
support or production authenticity.

## Remaining before milestone 3 could be called complete

1. Obtain the owner's decision whether to keep this direction at all.
2. If retained, finish the final whole-change review. The implementation was
   exercised and reviewed incrementally, but the complete large deletion/change
   diff has **not** received a final exhaustive review. In particular, verify
   that every useful old inspection/acceptance check has its intended surviving
   owner and that no release-only helper or stale active instruction remains.
3. Review new publication command arguments, partial-failure reporting and
   same-revision concurrency assumptions. The current implementation documents
   operator serialization, not registry compare-and-swap or atomic publication.
4. Review direct-artifact installation binding and reporting boundaries. Initial
   ISO defaults are recorded before the subsequent booted-digest assertion;
   assess whether that ordering adequately attributes partial observations to
   the candidate. No new qualification passes were added for missing scenarios.
5. Review test coverage and dependency/API changes (`go mod tidy` added the
   inspector validator's existing go-cmp dependency and removed unused direct
   x/term). Add or change anything only if the owner reauthorizes implementation.
6. After any authorized fixes, rerun focused/race tests and `just check`, inspect
   the full intended diff, and make a completion commit. Only then designate its
   exact clean revision as planned A and update issue #61 step 3.

Actual native publication and installed validation belong to later separately
appointed/authorized steps. Do not build A merely to finish this source review.

## Navigation

- Inspection: `internal/build/oci/{archive,image}.go`, `archive_test.go`.
- Publication: `internal/build/image/publication.go`, `publication_test.go`;
  `cmd/soda-image/publish.go`, `publish_test.go`.
- Wrapper: `scripts/prepare-native-image.sh`, `prepare_native_image_test.go`,
  `native_script_fixture_test.go` and retained `check_native_test.go`.
- Acceptance: `internal/acceptance/artifacts.go`, `runner.go`, `runner_init.go`,
  `runner_vm.go`, `fallback.go`, `summary.go` and adjacent tests;
  `cmd/soda-acceptance/{command,run}.go`.
- Installer: `internal/build/installer/{builder,qcow2}.go` and adjacent tests.
- Current instructions: `docs/release-operations.md`, `docs/installer.md`,
  `tests/acceptance/README.md`, relevant AGENTS and public deployment/update docs.

Temporary check logs are under `/tmp/soda-step3-*.log`; they are convenience
handoff context, not durable evidence artifacts or a resume database.
