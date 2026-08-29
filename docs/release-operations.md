# Soda OS artifact operations

Local development produces platform-specific OCI archives and installer ISOs
without publishing or signing them. Architecture selection is explicit and
`aarch64` and `x86_64` are equal sibling targets.

## Build a local installer

```sh
ARCH=x86_64 # or aarch64
just check
just oci "$ARCH"
just iso "$ARCH" ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar"
```

The ISO builder validates the archive platform, derives the exact manifest
digest from the local OCI layout, copies the payload into ISO-local container
storage, and writes the ISO plus its SHA-256 sidecar under `.artifacts/images/`.
It does not contact GHCR, push an image, invoke Cosign, or require any signing
key, passphrase, signature, or registry credentials.

The optional metadata command independently inspects the completed ISO and
writes an unsigned local record containing the image labels, exact local digest,
RPM inventory checksum, and ISO checksum:

```sh
just record "$ARCH" \
  ".artifacts/images/soda-os-0.3.1-${ARCH}.oci.tar" \
  ".artifacts/images/SodaOS-0.3.1-${ARCH}.iso"
```

## Distribution and updates

`soda-release` accepts completed AArch64 and x86-64 ISOs and records, creates a
paired release index, uploads the files to a GitHub draft, verifies the uploaded
bytes, and publishes the draft. This publishing step is separate from local OCI
and ISO construction.

Installed hosts resolve their platform entry from the release index, require an
exact `ghcr.io/levitateos/soda-os@sha256:...` reference, inspect the image's
platform and Soda metadata, and stage it with `bootc switch --download-only`.
Activation remains a separate administrator-confirmed reboot. Soda does not
poll, download, activate, or reboot automatically.

The current development and distribution flow is unsigned. Release signing and
provenance are outside this contract and require a separate product decision.

## Acceptance evidence

`tests/acceptance/bootc.sh` keeps disposable acceptance guests separate from
production installations. It can install or boot an architecture-selected VM,
wait for SSH and Cockpit health, exercise an attributed SSH workload, capture
host and guest evidence, and request a clean ACPI shutdown. Captures may include
the release index, local record, ISO hashes, bootc status, service state, and
QEMU state; they do not require or capture signature bundles.
