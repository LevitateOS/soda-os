# Build and release operations

This internal runbook owns artifact production and publication boundaries.
[The product contract](product-contract.md) defines the release; the
[handbook](public/10-Start-here/10-index.md) owns installation and updates.
Commands here are not evidence of execution or permission to publish.

## Inputs and artifacts

Read `justfile` and the selected script before executing it. `distro/soda.toml`
owns version/product identity, `distro/platforms/` selects each architecture's
base and locks, and `internal/build` constructs RPMs, OCI, ISO, QCOW2, and strict
release records. Keep x86-64 work on x86-64 hardware and AArch64 work on AArch64.

The network ISO uses its separately locked installer base, not a duplicated
bootc runtime root. Graphical Anaconda retrieves the exact published OCI digest.
ISO inspection rejects an inherited `/sysroot` in SquashFS, checks installer
configuration/branding, and distinguishes `docker://` source transport from the
persisted digest-only target reference. Installation needs anonymous retrieval
of that exact image; reaching Anaconda does not prove network installation.

The pinned Anaconda bootc mount correction exposes nested SELinuxFS through
Anaconda's own tracked mount/teardown sequence. Source version/checksum guards,
bytecode regeneration, and ISO inspection bind the correction. Do not replace
it with a Soda relabel service or weaker SELinux policy. Observe actual inode
labels; Anaconda copies logs before its final relabel task. Installer `/var/tmp`
scratch is not persistent target `/var`; verify mount identity in the correct
namespace/chroot and prove installer-only resources absent after boot.

Artifacts live under `.artifacts/`, never in Git. Reusing an unchanged lower-layer
artifact requires verified checksum, platform, digest, and relevant inventory.
Rebuild every changed layer above it, record installer and runtime revisions
separately, and use a fresh guest disk after installer changes. A stale adjacent
checksum is not proof of bytes, and source checks do not rebuild artifacts.

## Native candidate preparation

These commands **publish an OCI candidate to GHCR**, then build an installer:

```sh
scripts/prepare-native-iso-candidate.sh aarch64
# On matching x86-64 hardware instead:
scripts/prepare-native-iso-candidate.sh x86_64
```

Prerequisites:

- Clean checkout, including untracked files; HEAD equals the local `origin/main`.
  The script does not fetch, so that tracking ref is not a live remote check.
- Bash, Go, Git, just, jq, Skopeo, `sha256sum`, native shell utilities, and the
  pinned [frontend toolchain](cockpit-development.md#build-host-setup).
- Local Docker context, matching Linux daemon, and one running integrated Buildx
  `docker` worker on the same endpoint. Advertised platform lists do not prove
  native execution. The wrapper binds process-local context/worker variables;
  it changes no global selection. Remote/independent workers are a current
  wrapper restriction, not a product hardware rule. Use a context, not DOCKER_HOST.
- Exact platform inputs, disk/memory capacity, network access for existing input
  fetchers, and Skopeo credentials permitting candidate publication. If a locked
  Fedora manifest has expired remotely, provide its exact retained archive and
  verify the platform checksum; do not replace it with a moving tag or sibling
  base. Existing construction owns archive loading.

[Development](development.md) explains the native Linux source gate, including
the disposable check container on Darwin. Its development base is not the Soda
bootc base. Frontend assets are rebuilt by the existing RPM builder on the host;
check-container output is not reused as an artifact.

The candidate script:

1. Verifies source/native identity and refuses occupied candidate tags and final
   OCI/ISO/checksum paths, including dangling symlinks. Tag enumeration failure
   is an error, not evidence of absence.
2. Runs unchanged `just check`, then `just oci` once; OCI construction already
   builds RPMs. Run only one preparation per checkout because scratch is shared.
3. Checks OCI OS, architecture, version, and source labels; loads the archive,
   checks its actual image ID against its config/manifest digest, and executes
   that exact ID with `--pull=never` for runtime identity and RPM checks. Docker's
   classic store uses the config ID; its containerd store uses the manifest ID.
4. Uses `soda-release image-stage` to publish the immutable candidate
   `ghcr.io/levitateos/soda-os:sha-<full-revision>-<architecture>`, then verifies
   anonymous remote/local digest equality.
5. Builds and deeply inspects the ISO from that archive, checking its checksum
   sidecar and exact published-digest installer source. The summary reports
   source, architecture, tag, digest, ISO, and checksum—not installation success.

**Publication can succeed and ISO construction can fail.** Report attempted
versus confirmed publication, candidate tag, and expected digest. A failed
external command can itself have changed remote state. Stop and inspect; do
not overwrite, delete, compensate, retry automatically, or add saved workflow
state. A full rerun refuses occupied final paths/tags. Lower-level builders'
scratch replacement does not imply permission to replace a published candidate.

Preparation creates no installation VM, reusable QCOW2, signed release record,
version tag, or GitHub Release. Booting and installing require separate approval.
Use the existing matching-native QEMU configuration in `internal/acceptance/qemu.go`;
the complete acceptance CLI is not an ISO-only launcher and needs more inputs.
Loopback forwarding is not independent LAN evidence.

## Independent Linux/libvirt placement

On the matching Linux destination, place an existing verified ISO separately:

```sh
scripts/place-libvirt-iso.sh aarch64 VERIFIED_ISO DESTINATION_DIRECTORY
# Or on the x86-64 destination:
scripts/place-libvirt-iso.sh x86_64 VERIFIED_ISO DESTINATION_DIRECTORY
```

Placement needs the source `.sha256` sidecar, an existing writable destination
with suitable SELinux policy, GNU stat, passwordless sudo for its exact qemu-
account operations, and matching `qemu-system-*`. It checks actual directory
traversal, no-overwrite copy, both checksums, qemu readability, `virt_image_t`,
and a non-booting QEMU open. It neither builds nor publishes nor repairs host
permissions. Failure can leave a partial/complete destination; it does not
silently remove or overwrite that output. Mac QEMU/HVF testing does not require
libvirt placement or a second physical Mac.

## Qualification before production

Expensive ISO, cloud-init, product, networking, update, and fallback checks run
before release CI on user-controlled matching-native machines. Fallback A is
the previous signed published OCI digest, not a rebuild or historical ISO/QCOW2.

The [acceptance coverage map](../tests/acceptance/README.md#run-reports-versus-qualification-schema-2)
is authoritative for evidence. **The current runner cannot qualify a release:**
it does not establish installed onboarding observations, independent trusted-LAN
access, or public-ingress rejection. A successful itinerary is not full coverage.
Do not edit summaries or remove required checks to sign a report.

Qualified schema-2 sibling reports and both schema-3 candidate release records
are supplied to the maintained `Native acceptance evidence` workflow on exact
main. Each decoded input is limited to 12 KiB. Source/suite revisions must equal
that workflow's SHA; candidate digests/platforms must match their records.
Cosign signs and verifies the combined record using the fixed
`native-acceptance-evidence.yml@refs/heads/main` identity. The two reports, two
candidate records, combined record, and bundle are retained as a one-day Actions
artifact. The workflow receives no guest credentials and runs no VM/publication.

Production downloads that successful exact-SHA artifact and runs
`soda-acceptance verify`. Missing, expired, mismatched, or invalid evidence stops
before native preparation. Promote the same commit SHA: a new merge commit
needs its own acceptance. There is no signer override or ancestor/tree-equivalence
fallback. Signing authenticates earlier source-level observations, not a claim
that later CI-built bytes were boot-tested. See acceptance for exact CLI inputs.

## Build-once production sequence

A separately authorized push to protected `production` coordinates both siblings:

1. Check branch, clean source, version, collisions, and signed acceptance record.
2. Run cheap source/unit checks once; start matching-native builders in parallel.
3. Build each release B once, inspect OCI identity, and publish its candidate digest.
4. Derive network ISO and raw/compressed QCOW2 from the same OCI output.
5. Check structure, architecture, identity, checksums, size, signatures, provenance,
   and remote facts. No VM/product/fallback suite or guest enrollment runs in CI.
6. Promote exact accepted digests to immutable architecture release tags; create
   and sign strict release records.
7. Create the Git tag and draft GitHub Release, upload both asset sets, re-read
   every remote fact, and publish only when all identities and bytes agree.

The native release account puts temporary directories directly under its home,
linked from the immutable run directory, to keep QEMU Unix-socket paths short.
Source, cache, artifacts, and that link remain attributable to the exact run.
No release copy or fallback A is rebuilt.

Strict records bind version, source, architecture, base, exact OCI digest,
RPM inventory, ISO, raw QCOW2, and compressed QCOW2 checksums. GHCR owns image
storage; GitHub Releases owns downloads; native Git/Skopeo/Cosign/GitHub CLI own
transport and credentials. OIDC is short-lived authentication, not storage.

Publication requires anonymous retrieval, correct workflow signature/provenance,
exact remote file bytes, both OCI update digests in release notes, and an
unchanged remote production head. Known collisions fail before mutation;
partial external success is reported without cleanup/reconciliation machinery.
The wall-clock target is 30 minutes; at 45 minutes report the active slow stage
and continue rather than imposing a hard timeout. No moving-tag policy is implied.

## Historical evidence and remaining native work

Exact hashes, logs, and original observations remain in the pinned
[pre-renewal operations record](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/release-operations.md)
and [incident record](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/bug-notes.md).
Their evidence files are not deleted by documentation renewal.

- AArch64 2026-09-05 source checks passed in Docker's native Linux environment
  on a dirty development snapshot, with evidence in `.artifacts/native-check.g2vtPG/`.
  That does not certify a later clean candidate. Correct Vite+ Linux PATH, jq,
  and an ordinary check UID are necessary; no gate was weakened.
- Native ARM backend probes passed OCI export/local contexts, nested Podman,
  file ownership through the host bind, and disposable mount/loop operations
  (`.artifacts/native-probes.DQJACq/`). The observed overlay warning and slow local
  context resolution were performance findings, not demonstrated incompatibility.
  These probes did not prove full ISO construction, capacity, or installation.
- At `bf4ea45`, AArch64 DNF exposed mismatched OpenSSL CLI/libraries. Its runtime
  lock was corrected without changing the pinned base. Native Cosign is now
  source-built by the existing RPM path for both architectures, not downloaded
  at runtime. At `b027b4e` the AArch64 OCI rebuild and runtime identity checks
  passed; bootc lint retained two warnings about `/run`/`/tmp` and `/var` tmpfiles
  (`.artifacts/native-oci.iGvZss/`). That proves neither installation nor publication.
- Historical x86-64 installer composition removed a duplicated runtime root and
  reached graphical Anaconda. Earlier x86-64 and AArch64 B→A→B runs exercised
  superseded provisioning and tools; neither qualifies the present release.
- Native x86-64 must repeat applicable source/backend/build/inspection checks
  using its own locks. Both siblings still require current graphical ISO,
  QCOW2, native frontend, signed-update/fallback, and full qualification evidence.

Do not turn whichever machine was available for a historical run into a product
prerequisite. Check actual tools, inputs, and resources before the next run.
