# Soda OS artifact operations

> [!IMPORTANT]
> Publication remains a later architectural-reset milestone. Installed Soda
> systems do not consume the paired release index at runtime; administrators
> select exact published image digests through native `bootc` operations.

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
Forgejo and Bun source inputs, and every command in the installed immutable
tool manifest. Bun source and RPM construction occur only on matching-native
hardware. There is no runtime source lookup or tool download path.

The optional metadata command independently inspects the completed ISO and
writes an unsigned local record containing the image labels, exact local digest,
RPM inventory checksum, and ISO checksum:

```sh
just record "$ARCH" \
  ".artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar" \
  ".artifacts/images/SodaOS-0.4.0-${ARCH}.iso"
```

## Distribution infrastructure decision

Production releases use these two distribution services:

- GHCR stores each architecture-specific Soda OS OCI image. The exact OCI
  manifest digest, never a mutable tag, is the update authority.
- GitHub Releases stores the paired AArch64 and x86-64 installer ISOs, their
  SHA-256 files, architecture-specific release records, the paired release
  index, and the release index's Sigstore bundle. The marketing website may
  link to these releases, but is not an update authority.

Published release data is append-only. A production publisher must fail before
publishing if the Git tag, GitHub Release, or any intended asset name already
exists. It must never replace a published version asset or move a published
version to different bytes or digests.

One future protected GitHub Actions workflow owns production publication:

- workflow: `.github/workflows/release.yml`;
- OIDC issuer: `https://token.actions.githubusercontent.com`;
- certificate identity:
  `https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/tags/v<VERSION>`,
  expanded to the exact release tag being verified;
- protected GitHub environment: `production-release`, with required human
  approval.

The workflow must run architecture-owned build, inspection, signing, and
publication jobs on native AArch64 and x86-64 runners. A coordinating job may
create and publish the paired index only after both architecture records report
the same Soda version and source revision.

For each architecture, the future workflow pushes the image to GHCR, resolves
its exact manifest digest, and signs that digest with Sigstore keyless signing.
It then generates the paired release index, signs the index as a blob with
Sigstore keyless signing, retains the bundle beside the index, and publishes
both architectures' ISOs, SHA-256 files, and release records in one GitHub
Release. The index continues to identify images by exact GHCR digest.

Verification must require both the exact workflow certificate identity for the
release tag and the GitHub Actions OIDC issuer. The retained blob bundle carries
the signature, signing certificate, and transparency-log proof required to
verify the paired release index.

The current `soda-release` command can assemble a paired index and GitHub draft,
but it is not the production publisher described above until the protected
workflow, collision checks, GHCR publication, and Sigstore verification are
implemented.

This release cycle records the infrastructure decision only. It does not add
the workflow, configure the GitHub environment or runners, provision a service,
push an image, sign an artifact, upload an asset, or publish a release.

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

Local development artifacts and records remain unsigned. Production signing and
publication belong only to the protected workflow defined above.

## Acceptance evidence

`tests/acceptance/bootc.sh` keeps disposable acceptance guests separate from
production installations. It can install or boot an architecture-selected VM,
wait for SSH and Cockpit health, exercise an attributed SSH workload, capture
host and guest evidence, prove native account-preserving image selection, and
request a clean ACPI shutdown. Captures may include the local record, ISO
hashes, bootc status, service state, stock Cockpit package discovery, the exact
immutable command manifest, absence of the former toolchain mount and state,
and QEMU state. Local acceptance does not create or require production
signatures.
