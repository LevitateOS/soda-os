# Soda OS image construction and publication

**Paused work-in-progress:** #61 step 3 is saved but not declared complete.
See the [checkpoint handoff](issue-61-step-3-handoff.md) for review gaps and the
owner's stop instruction before using or continuing this implementation.

This internal operator document describes the pre-alpha native development flow
selected in #61. Commands that build, publish, install or restart require their
own operational authorization. Source tests do not prove a published image or
an installed update.

## Native development loop

On the matching-native machine, from any clean committed checkout:

```sh
just dev-image x86_64
# Or, independently on AArch64 hardware:
just dev-image aarch64
```

`just dev-image <architecture>` runs `scripts/prepare-native-image.sh`. It:

1. Checks the selected hardware, local Linux Docker daemon and integrated Buildx
   worker, then the clean source revision. It never fetches or changes Git refs.
2. Runs the complete source gate once through `scripts/check-native.sh`.
3. Prepares the existing reviewed inputs and builds one OCI archive, including
   RPM construction once, under `.artifacts/images/<architecture>/<revision>/`.
4. Uses the actual path returned by `soda-image oci`, checks its source/platform
   metadata, loads it, and executes the exact verified image ID without pulling.
   Linux architecture, os-release identity and Soda RPM versions must agree.
   Both classic Docker config IDs and containerd manifest IDs are accepted only
   when they match the archive's own identities.
5. Publishes that archive through `soda-image publish` and stops.

It builds no ISO/QCOW2, creates no VM, and performs no installed update. It
requires neither `origin/main` equality nor GitHub-published source, a version
increase, production-branch promotion, signature, release record, installer,
prior qualification or sibling result. Both architectures keep their own native
locks and tags. No new CI publication control plane replaces the removed one.

## Prerequisites and source verification

Use Bash, Go, Git, just, jq, Skopeo, Docker, the pinned Vite+/Node toolchain and
ordinary shell tools. Supply the selected architecture's exact locked Fedora
base and reviewed fetched inputs, adequate native build capacity, and ordinary
Skopeo credentials with push permission. HTTPS and anonymous image retrieval
must work. Credentials alone do not establish push permission.

The wrapper verifies a local Docker context with a matching Linux daemon and one
running integrated `docker` Buildx worker. It sets process-local `DOCKER_CONTEXT`
and `BUILDX_BUILDER`, not global configuration. Use a local context rather than
`DOCKER_HOST`. These are current execution checks, not a universal restriction
on Soda's product platforms.

On Linux, `scripts/check-native.sh <architecture>` runs `just check` as the
ordinary build user. On Darwin it builds `tools/check/Containerfile` and runs
that same gate inside a native Linux container as the unprivileged check user.
The canonical checkout is mounted read-only; a private exact-commit clone owns
its generated frontend files and caches. No Docker socket, privileged mode or
host output mount is provided to that check container. This Darwin source gate
itself writes a development image/cache and needs build authorization.

Use one build per checkout: revision-scoped OCI destinations do not isolate the
shared RPM/build scratch directories. The current Cosign package/fetch chain and
Go cache-lifetime corrections are pending step 4, not publication requirements.

## OCI output destinations

`image.Builder.BuildImage(ctx, outputDir)` returns `(archivePath, error)`. Empty
`outputDir` selects `.artifacts/images`; relative destinations resolve against
`Builder.Root`, and absolute destinations are used directly. The result is an
absolute `soda-os-<version>-<architecture>.oci.tar` path. Native enforcement,
clean source checks before/after RPM building, locked inputs, image identity,
RPM checks and bootc lint remain mandatory. OCI construction loads the archive
locally for lint, but does not publish it.

`just oci <architecture> [output_dir]` prepares inputs and constructs locally.
For direct path capture after the existing input-fetch recipes have completed:

```sh
ARCH=x86_64 # or aarch64 on matching hardware
revision=$(git rev-parse HEAD)
archive=$(go run ./cmd/soda-image --architecture "$ARCH" oci \
  --output-dir ".artifacts/images/$ARCH/$revision") || exit 1
```

The lower-level OCI CLI emits only the absolute path plus a newline on stdout;
progress and traces go to stderr. Do not scrape `just` or native progress.
A rebuild replaces only the selected archive. Filesystem/export/load/lint failure
returns an error and no successful path; a partial or unvalidated archive may
remain. A result-write failure also returns an error. Other revision directories
are not cleaned up. There is no artifact-history database or resume state.

## Publication and partial failures

To publish an already-built archive, without building again:

```sh
go run ./cmd/soda-image --architecture "$ARCH" publish --archive "$archive"
```

`internal/build/oci` inspects the archive's native manifest/config/digests,
version, source revision, exact base reference and RPM inventory/sidecar. It
rejects unsafe archive paths, wrong platforms and corrupt blobs. These facts
come from the image bytes; publication never substitutes publisher HEAD or the
current specification's version/base for an older archive's identity.

With authenticated Skopeo and HTTPS, publication checks
`ghcr.io/levitateos/soda-os:sha-<archive-revision>-<architecture>`. If absent it
copies the archive with digest preservation, then verifies the remote digest.
An existing identical revision digest may be reused on an explicit republish;
a conflicting digest or failed lookup stops before development advancement.
The verified **exact remote digest**, not the mutable revision tag, is copied to
`dev-x86_64` or `dev-aarch64`, then anonymously inspected for the same digest.
Neither sibling tag is touched. Authorized publishers own registry credentials
and must serialize publication of a given revision/tag; this is not a registry
transaction or an atomic compare-and-swap service.

Copy failures may have changed remote state. Errors distinguish revision-copy
failure, revision verification failure, development advancement failure and
anonymous verification failure. Inspect the named tag/digest before an explicit
retry. There is no automatic retry, compensation, cleanup or reconciliation.
The completion message identifies the actual image reference and source; it is
not installed-update evidence or cryptographic provenance from a trusted signer.

## Independent installers and acceptance

When separately requested, `just iso <architecture> <archive>` and
`just qcow2 <architecture> <archive>` construct graphical network ISOs and raw/
compressed QCOW2 disks from that exact OCI. They use the shared inspector and
retain native tooling, exact digest binding, ISO squashfs/initramfs/branding
checks and ordinary checksum sidecars. Raw QCOW2 now also has a `.sha256`
sidecar. Network ISOs require anonymous access to the embedded exact digest.

`scripts/place-libvirt-iso.sh <architecture> <ISO> <destination>` remains a
separate matching-Linux operation: checksum verification, no-overwrite copy,
qemu traversal/readability, SELinux label and non-booting QEMU open. It does not
build, publish, repair host permissions or boot an installation VM.

The [acceptance runner](../tests/acceptance/README.md#go-runner) consumes candidate
OCI/ISO/raw-QCOW2 paths and an earlier fallback OCI directly. It checks native
OCI facts and installer checksum sidecars, then installed booted-digest readback
binds the actual ISO and QCOW2 deployments to the candidate. Fallback facts use
the earlier image's own version/base, independent of the current specification.
Self-computed checksums detect byte changes; they are not provenance.

Per-run `RunSummary`, partial results, credential handling, cleanup and truthful
`Validate`/`Qualify` semantics remain. Missing onboarding, independent LAN and
public-ingress observations still do not qualify. Combined signed acceptance
records, record commands, the release executable/package, production release CI
and signing-evidence CI have been deleted. The **soda-release RPM** and OS
identity/branding are retained. No release accounts, branch history, registry
tags or operator artifacts are cleaned up by this source change.

## Planned A and verification limits

The clean commit completing #61 step 3 is the planned **A source**; its exact
commit ID is recorded in the implementation handoff. Do not build it during this
source assignment. Step 4 removes the unused installed Cosign pipeline; the later
B source can demonstrate a real same-version image difference without a dummy
feature. Neither source checks nor historical evidence below establish that this
new publisher has run against GHCR or that A has booted. Both architectures need
their own explicitly authorized native artifact and installed evidence.

## Historical build evidence (not validation of the replacement)

The following observations predate the native development publication replacement.
They describe those checkpoints only, not current prerequisites or live PASS.

### Bounded AArch64 source-check evidence

On 2026-09-05 the verified `desktop-linux` integrated Docker worker ran the
complete gate on this MacBook's native Linux/AArch64 backend, without privileges
or a Docker socket. Tool versions were Go 1.26.7, Node 24.20.0, Vite+ 0.3.0,
just 1.40.0 and jq 1.7. The check image's local identity was
`sha256:d1d4e5edfecce624f3536f38a8c9a684802fd5f143b2fe93e1940ffd90f0304f`.

The checked development snapshot contained 455 source files, based on
`fbc3082357fa81e6f5361e52c9abcdb796686af3` plus the uncommitted shared-workflow
changes. Its tar SHA-256 was
`477287bef579baf2b22028ef95326fbdef6c7ed9d3776626c010161951d63c91`.
Canonical source was mounted read-only; its private Linux clone was overlaid
with that snapshot only inside the container. Source-file checksums and canonical
Git status matched after the run. This evidence paragraph was added afterward.

All unchanged gate stages passed: formatting, complexity/release source checks,
frontend install/check/build/test, `go vet ./...`, acceptance race tests,
`go test ./...`, and both architecture-static image configuration checks.
Frontend results were 81 passed and the two existing opt-in RPM/installed tests
skipped; no artifact or installed acceptance is claimed.

The first check-image build exposed the incorrect Mac-style Vite+ path. The
first full gate then exposed that root was rejected by Projects caller-identity
tests. Only the development environment and its source-contract test changed:
correct Linux path, explicit jq dependency, and an ordinary check user. No gate,
Linux product behavior, platform lock or bootc base was changed.

Local evidence is retained under `.artifacts/native-check.g2vtPG/`:
`build.log`, `build-v2.log`, `check.log`, `build-v3.log`, `check-v3.log`, and the
`v3/` snapshot/checksums. Containers were removed after exit; the development
image and Docker build cache remain. These are test evidence, not resume state.
The dirty snapshot run does not certify a clean candidate commit or bypass the
candidate's clean-source gate. Both that exact-commit run and matching-native
x86-64 verification remain outstanding.

### Bounded AArch64 backend-probe evidence

On 2026-09-05 the same MacBook/backend passed disposable native ARM probes using
Image Builder 81.0.0 at the exact `installer-image-builder-aarch64.toml` digest
`sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a`
(Podman 5.8.4, Skopeo 1.22.2). The probe image was derived from that tooling
image, not a replacement Soda bootc base. No locked bootc archive was loaded.

Passed boundaries:

- Buildx local `docker-image://` context resolution, actual Linux/AArch64 build
  execution, and `type=oci` export with OCI media types and timestamp rewriting.
- Skopeo archive import into a dedicated named volume at
  `/var/lib/containers/storage`, then privileged nested Podman execution.
- Nested writes to a Docker Desktop host bind (reported as `fuse`), symlink
  preservation, Linux `chown -R` to the host's numeric UID/GID, host readback,
  owner mode adjustment and removal of the generated inspection tree.
- Nested tmpfs and bind mounts, creation of an ext4 filesystem on a newly
  allocated loop device backed only by a 64 MiB sparse probe file, writes and
  successful read-only remount/readback. The mounts and loop device were released.

The first nested command hit the probe image's inherited Image Builder
entrypoint, not a backend failure. Explicitly selecting Bash for the probe
resolved it; no product invocation changed. Skopeo reported non-native overlay
diff due to the kernel's `CONFIG_OVERLAY_FS_REDIRECT_DIR` setting, and local
Buildx context resolution took 90 seconds. These are observed performance
caveats, not demonstrated incompatibilities.

Evidence and the disposable OCI archive remain under
`.artifacts/native-probes.DQJACq/`. Its manifest digest is
`sha256:cfe59d459aa2e4ab263d73992e5daeaee73724afed7d51a1d703774ed2fcb92c`.
Logs include `oci-export.log`, `storage-import.log`, `nested-podman-v2.log`,
`ownership.log`, `mount-probe.log`, `host-readback.log` and `cleanup.log`.
No nested containers or loop devices remained at cleanup; the probe volume and
local base tag were removed. The locked tooling image and build cache remain.

These checks do not prove RPM construction, locked bootc-base loading, actual
`bootc-generic-iso` construction, squashfs/initramfs inspection, full-build
capacity, publication or installation. No need for a separate Linux build VM
was demonstrated. Native x86-64 must reproduce these probes on matching
hardware; no sibling-architecture execution occurred here.

### Native OCI dependency fixes

The first real AArch64 OCI attempt at `bf4ea45` loaded the exact retained Fedora
base and built the Soda RPMs on this MacBook, then stopped at DNF resolution.
The AArch64 runtime lock paired OpenSSL 3.5.7 with its 3.5.8 libraries; native
repository queries confirmed the matching 3.5.8 CLI is available. Only that
AArch64 Fedora input is revised; the pinned bootc base is unchanged.

Soda Updates also required `/usr/bin/cosign` without a packaged provider.
`just cosign-source` now fetches the checksum-locked upstream Cosign 3.1.3
source commit. The existing native Linux builder produces `soda-cosign`, checks
its upstream version/commit and verification commands, and includes its Apache
license. There is no new runtime downloader or signing credential. Both
architectures declare the same source-built RPM; this is not x86-64 dependency
resolution or artifact validation. Native x86-64 construction and runtime checks
remain required on matching hardware.

Initial failure evidence is retained under `.artifacts/native-oci.iGvZss/`.
The clean rebased source at `bf4ea45` had passed the unchanged Linux `just check`
gate in `.artifacts/rebased-native-check.XXXXXX.log`; those source checks did
not prove that an image dependency transaction would succeed.

At `b027b4e`, the real AArch64 OCI rebuild succeeded, including the native Cosign
RPM, the locked package inventory, and `bootc container lint` (11 checks passed,
one skipped). Lint reported two warnings: content in `/run`/`/tmp`, and `/var`
paths without tmpfiles entries. They are retained in `build-fix.log`, not
suppressed. Installed-system behavior is still unverified. Direct execution
confirmed Linux/AArch64, Soda OS 0.6.3, matching OpenSSL 3.5.8 packages, Cosign's
pinned upstream version/commit, and an unchanged RPM inventory.

That image also reproduced the containerd manifest-ID distinction above; the
corrected runtime verifier passed against the actual archive. Subsequent loop
logs and cleanup evidence remain in `.artifacts/native-oci.iGvZss/`. Checks and
local OCI construction do not imply GHCR publication or installation approval.
