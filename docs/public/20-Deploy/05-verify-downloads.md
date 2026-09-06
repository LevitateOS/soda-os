# Verify downloads

Verify the signed release record and the exact architecture-matched installer or disk image before using it.

## Choose the files

Open the [Soda OS releases](https://github.com/LevitateOS/soda-os/releases/latest)
and read the release notes. Choose **x86-64** for x86-64 hardware or **AArch64**
for ARM64 hardware, including a matching-architecture VM. Download:

- the ISO for graphical installation, or `.qcow2.zst` for disk-image import;
- that artifact's `.sha256` sidecar;
- the matching architecture's `.release.json` record and `.sigstore.json` bundle.

Keep one release's files together. Use the actual filenames below instead of
wildcards that could accidentally select an older download. Verification needs
[Cosign](https://docs.sigstore.dev/cosign/system_config/installation/), a SHA-256
tool, and a way to read JSON such as `jq` on your client.

## Verify the record first

Set `RECORD` to the downloaded filename. This command pins the Soda production
workflow and GitHub Actions issuer—not an identity supplied by the download:

```sh
RECORD='soda-os-VERSION-ARCHITECTURE.release.json'
cosign verify-blob \
  --bundle "$RECORD.sigstore.json" \
  --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "$RECORD"
```

Stop if verification fails. Inspect the verified JSON for the expected version,
source revision, architecture, and immutable `soda_image_reference` ending in
`@sha256:DIGEST`. Do not substitute a moving image tag. See
[Sigstore verification](https://docs.sigstore.dev/cosign/verifying/verify/)
for certificate and bundle semantics.

## Match the actual file bytes

Set `FILE` to the ISO or compressed QCOW2 filename you downloaded:

```sh
FILE='REPLACE_WITH_EXACT_DOWNLOADED_FILENAME'
sha256sum "$FILE"
sha256sum --check "$FILE.sha256"
```

On macOS, use `shasum -a 256 "$FILE"` and `shasum -a 256 --check "$FILE.sha256"`.
On Windows, `Get-FileHash -Algorithm SHA256` provides the file hash.

Compare the computed hash with the **verified release record**, not only its
sidecar: use `iso_sha256` for the installer or `qcow2_zst_sha256` for the compressed
disk. A checksum sidecar alone does not authenticate who produced an image.
A mismatch, missing file, wrong architecture, or invalid signature means stop.

For QCOW2, decompress after verification and compare the result with the
record's `qcow2_sha256` before import:

```sh
zstd --decompress "$FILE"
```

## Verify an OCI image for native updates

The Soda Updates page performs image verification for you. If using native
bootc commands, take the exact architecture-matched reference from the verified
record and check its signature and provenance first:

```sh
IMAGE='ghcr.io/levitateos/soda-os@sha256:REPLACE_WITH_VERIFIED_DIGEST'
cosign verify \
  --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "$IMAGE"
cosign verify-attestation --type slsaprovenance \
  --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "$IMAGE"
```

Confirm the verified provenance identifies the expected source revision and
image. Keep the verified record and exact digest with your administrative notes.

Continue with [ISO installation](20-install-on-premises.md),
[cloud/VM import](10-deploy-to-cloud.md), or
[updates and fallback](../30-Use-Soda/60-updates-and-fallback.md).
