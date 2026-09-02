# Soda OS artifact operations

> [!IMPORTANT]
> `soda-release` is an operator-side boundary around Git, `gh`, and GitHub
> Releases. It is not installed in Soda OS, and installed systems consume no
> release index. Administrators select exact published image digests through
> native `bootc` operations.

Local development produces platform-specific OCI archives and installer ISOs
without publishing or signing them. Architecture selection is explicit and
`aarch64` and `x86_64` are equal sibling targets.

Each architecture's artifact work runs on matching native hardware: run
`aarch64` RPM, OCI, ISO, record, installation, and artifact-validation work on
an AArch64 host, and run the corresponding `x86_64` work on an x86-64 host.
`soda-image` rejects a mismatched selected architecture before resolving
artifact inputs or invoking Docker. `just check` remains a cross-architecture
source and contract check and may run on either host.

## Build a local installer

```sh
ARCH=x86_64 # run this only on an x86-64 host; use aarch64 only on an AArch64 host
just check
just oci "$ARCH"
just iso "$ARCH" ".artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar"
```

The ISO builder validates the archive platform, derives the exact manifest
digest from the local OCI layout, copies the payload into ISO-local container
storage, and writes the ISO plus its SHA-256 sidecar under `.artifacts/images/`.
It does not contact GHCR, push an image, invoke Cosign, or require any signing
key, passphrase, signature, or registry credentials.

The OCI build also verifies the architecture-owned package lock, the fixed
Forgejo, Bun, and Tea source inputs, their narrowly carried authentication
patches, and every command in the installed
immutable tool manifest. Bun and Tea source and RPM construction occur only on
matching-native hardware. There is no runtime source lookup or tool download
path.

The metadata command independently inspects the completed ISO and writes an
unsigned local record containing the image labels, exact local digest, RPM
inventory checksum, and ISO checksum. The record is optional for artifact
construction but required when creating protected installer input:

```sh
just record "$ARCH" \
  ".artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar" \
  ".artifacts/images/SodaOS-0.4.0-${ARCH}.iso"
```

## Create protected installation input

Every installation pairs the Soda product ISO with a new protected, removable
OEMDRV answer medium. Create it on the same matching-native architecture:

```sh
go run ./cmd/soda-image --architecture "$ARCH" installer-input \
  --iso ".artifacts/images/SodaOS-0.4.0-${ARCH}.iso" \
  --release-record ".artifacts/releases/soda-os-0.4.0-${ARCH}.release.json" \
  --username soda-admin \
  --ssh-public-key-file "$HOME/.ssh/id_ed25519.pub" \
  --tailscale-auth-key-file /secure/path/tailscale-auth-key \
  --output /secure/path/soda-installer-input.iso
```

The command prompts twice for the administrator password. For automation,
`--password-file /secure/path/administrator-password` supplies it from a
mode-`0600` regular file. The password and Tailscale files must not be symlinks;
do not put either secret in argv, environment values, logs, or repository
files. The command validates the release record, ISO checksum, and selected
platform, refuses to overwrite an output, and creates the OEMDRV image with
mode `0600`.

Attach both images and boot the product ISO. The ISO selects the secret-free
Kickstart from OEMDRV; stock Anaconda owns storage and installation while fixed
installer-only hooks create native account input and perform the bounded
Forgejo handoff. The guest must eject OEMDRV before installation continues.
Remove and destroy the exact host copy after ejection. The first boot gives the
one-use key to native `tailscale up` once, then deletes the key and disables the
unit regardless of success.

There is no long-running or reusable runtime bootstrap service, credential
store, or in-place Soda recovery workflow. On installer failure, discard the
incomplete target, correct the input, generate a new OEMDRV image, and perform
a fresh installation. On a failed Tailscale attempt, use native local recovery
or reinstall with a fresh one-use key.

## Distribution and publication boundary

Production releases use these two distribution services:

- GHCR stores each architecture-specific Soda OS OCI image. The exact OCI
  manifest digest, never a mutable tag, is the update authority.
- GitHub Releases stores the paired AArch64 and x86-64 installer ISOs, their
  SHA-256 files, and independently justified release records and signature
  material. The pre-reset paired release index is not a preservation contract
  now that installed systems do not consume it. The marketing website may link
  to these releases, but is not an update authority.

Published release data is append-only. Draft creation fails before mutation if
the version tag or release already exists; each architecture upload fails
before mutation if any of its intended asset names exists. The workflow never
replaces a published version asset or moves a published version to different
bytes or digests.

GitHub CLI is the maintained GitHub Release boundary. `soda-release` constructs
only fixed `gh` operations and leaves authentication, transport, tags, drafts,
assets, and releases under GitHub and GitHub CLI ownership. Authenticate the
operator beforehand with native `gh auth`; Soda does not accept, read, copy, or
store a GitHub token.

Production publication starts from one clean checkout whose full `HEAD` is the
intended release tag target. The architecture-specific release records must
name that same source revision. This is a publication restriction, not an
artifact-construction rule: local installer work may still reuse a validated
runtime OCI whose image revision predates an installer-only change, but that
candidate cannot be published through the current command until its provenance
is represented without ambiguity.

Prepare a regular release-notes file, then create the absent tag and empty
draft:

```sh
go run ./cmd/soda-release --spec distro/soda.toml draft \
  --notes-file /path/to/release-notes.md
```

The command requires a clean tracked worktree, verifies that `HEAD` exists in
the configured GitHub repository, and fails before mutation if the version tag
or release already exists. Untracked operator notes or ignored build artifacts
do not affect source identity. It creates the tag at that exact revision and
then creates an empty draft named `Soda OS <version>`.

Each matching-native builder uploads only its own three validated assets. Run
this once on AArch64 and once on x86-64, from the same source revision:

```sh
ARCH=x86_64 # use aarch64 only on matching-native AArch64 hardware
go run ./cmd/soda-release --spec distro/soda.toml upload \
  --architecture "$ARCH" \
  --iso ".artifacts/images/SodaOS-0.4.0-${ARCH}.iso" \
  --record ".artifacts/releases/soda-os-0.4.0-${ARCH}.release.json"
```

The checksum sidecar is derived as `<ISO>.sha256`. Before upload, the command
validates the exact filenames, regular-file boundaries, release record, source
revision, image reference, ISO bytes, and sidecar. It refuses an existing
architecture-owned asset and invokes `gh release upload` without `--clobber`.
GitHub's returned asset name, size, uploaded state, and SHA-256 digest must then
match the local input.

The complete draft contains these six required base assets:

```text
SodaOS-<version>-aarch64.iso
SodaOS-<version>-aarch64.iso.sha256
soda-os-<version>-aarch64.release.json
SodaOS-<version>-x86_64.iso
SodaOS-<version>-x86_64.iso.sha256
soda-os-<version>-x86_64.release.json
```

Independently produced signature material may coexist with these assets. It is
not generated or interpreted by `soda-release`.

> [!CAUTION]
> `soda-release publish` is not a signing boundary. It neither signs GHCR
> images or GitHub assets nor proves that the product's separate production
> signing requirement has been satisfied. The release owner must complete and
> verify the separately authorized maintained-tool signing process before
> invoking `publish`. Source completion or a six-asset draft alone is not a
> signed production release.

After that external signing gate has passed, publish the complete draft:

```sh
go run ./cmd/soda-release --spec distro/soda.toml publish
```

The command requires the same clean source revision, exact tag and draft, both
architectures' six base assets, and valid GitHub SHA-256 metadata for every
asset. It publishes with `gh release edit --verify-tag --draft=false`, then
verifies that the release state changed without changing its assets.

Published release data is never replaced. If tag creation, draft creation, or
an upload partly succeeds, Soda reports the GitHub-owned state and performs no
retry, deletion, compensation, or reconciliation. Any inspection or cleanup is
a separately authorized native `gh` operation; a fresh publication attempt may
begin only after the operator has deliberately resolved that state.

Publication, signing, image push, and release deployment are separate
operational authorizations. Repository tests exercise validation and fixed
command construction without contacting or changing GitHub. No GitHub Actions
workflow, OIDC identity, or protected-environment mechanism is selected by the
current architecture.

## Installed-system image lifecycle

An administrator selects an exact published image digest and uses the native
sequence:

```sh
sudo bootc status
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Supported fallback uses the same sequence with an earlier exact Soda digest.
Direct `bootc rollback` is unsupported because current account state must be
preserved. Soda does not discover releases, poll, download, activate, or reboot
automatically.

Local development artifacts and records remain unsigned. Production signing
and publication use the separately authorized maintained-tool boundaries above.

## Acceptance evidence

`tests/acceptance/unattended.sh run` is the sole public installed-product
workflow. One process owns a fresh raw-QEMU installation, the protected OEMDRV
medium, a loopback-only disposable registry, native B-to-A-to-B image
selection, product scenarios, evidence capture, and clean shutdown. Its private
helpers are not alternative workflows. Local acceptance does not create or
require production signatures.

The raw-QEMU harness creates OEMDRV through the same
`soda-image installer-input` boundary. It retains QMP evidence that the guest
opened and unlocked the exact removable device, removes the medium only after
that proof, verifies the drive is empty, and deletes the secret-bearing host
image. Capture also requires the installed system to lack saved Kickstart,
installer-hook, legacy installer-extension, and credential state.
