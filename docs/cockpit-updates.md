# Soda Updates implementation

The approved browser workflow is: find the latest published stable Soda release,
verify its architecture-specific signed record and exact OCI image, download that
digest, and explicitly apply and restart. Development candidate tags are not an
update channel. Native bootc remains authoritative for deployment state; this
work adds no updater daemon, database, automatic updates, or copied account state.

## Release verification boundary

`internal/updates` implements the first milestone: read-only discovery and
verification. It is not yet installed or exposed through Cockpit. The native
Cosign and Skopeo commands are invoked through the process runner, with temporary
record/bundle files removed on success and failure. GitHub responses are bounded
and requests have a timeout. No GitHub token is required or retained.

Discovery uses GitHub's `releases/latest` endpoint, not registry tag sorting.
It requires a published, non-prerelease `vMAJOR.MINOR.PATCH` release and exactly
one host-architecture record and Sigstore bundle. Verification binds the record
to the production workflow certificate identity and requires schema 3, matching
version/platform/channel, a full source revision, and an exact Soda GHCR digest.
Image signature and provenance verification use that digest, followed by an
anonymous OCI metadata check against the record. Artifact checksums remain owned
by publication; this boundary reads only the signed identity fields.

No release, transport failure, invalid signature, and mismatched image are
separate failure cases, never evidence that an installation is up to date.
Stable versions compare numerically without integer overflow. Non-stable local
versions require an explicit native administrator decision rather than an
automatic downgrade or guessed ordering. A same-version/different-digest image
must be displayed as a distinct installation, not silently replaced.

## Native bootc contract to preserve

Source inspection of bootc `v1.16.10`, matching the current Soda runtime lock,
finds `downloadOnly` on staged deployment entries and `rollbackQueued` in host
status. This is evidence of the JSON contract, not an installed-system test.

The established Soda CLI sequence is:

1. `bootc switch --download-only EXACT_VERIFIED_DIGEST`
2. Inspect bootc state and confirm the selected digest is downloaded.
3. `bootc switch --from-downloaded`
4. Restart explicitly after successful finalization.

The future page must reread native state after reconnect and before activation,
and refuse a changed target. Determine how to bind the comparison and activation
against concurrent native CLI operations before claiming stale-target safety.
In `v1.16.10`'s `crates/lib/src/cli.rs`, `SwitchOpts.target` explicitly conflicts
with `from_downloaded`, and `apply_from_downloaded_ostree` unlocks whichever
staged deployment it finds. It accepts no expected digest. A status check in a
separate command is therefore not an atomic guard against a concurrent native
switch. This is a concrete activation integration gap, not a reason to claim
that a separate status check guarantees the selected image.
Do not solve this by adding a Soda deployment-state file or asserting that a
Soda-only lock also protects ordinary bootc commands.

Direct `bootc rollback`, rollback-based cancellation, automatic-update controls,
arbitrary image switching, and deployment deletion/pinning are outside the
initial page. Supported fallback remains the account-preserving exact-image
switch procedure in the administrator handbook.

## Remaining implementation and prerequisites

- Add the bounded command entry point and Cockpit React/PatternFly page.
- Implement native status, exact verified download, and confirmed activation.
- Confirm all native flags and disconnect/concurrency behavior against the
  pinned bootc version, including downloaded versus finalized staged state.
- Package the page and its verifier through the existing RPM workflow.
- Cosign is installed on the development host but is absent from Soda's explicit
  runtime RPM locks. Verify actual base/tool availability and add reviewed native
  dependency inputs before claiming the installed page can verify releases.
- Dependency resolution, artifact construction, and live acceptance must be
  performed independently on native x86_64 and AArch64. No sibling artifact
  support is established by mocked architecture-selection tests.
- Verify the full update/restart flow and preservation of current Linux accounts,
  passwords, groups, homes, project workspaces, and services on disposable guests.

Builds, publication, guest creation, installation, and live updates remain
separately authorized operations. Source checks and mocked command tests do not
claim a usable installed update page.
