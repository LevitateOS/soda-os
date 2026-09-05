# Soda OS release operations

This is an internal operator document. The accepted release contract is in
[architecture-reset.md](architecture-reset.md). No command in this document is
evidence that a public release exists.

## Product contract

A push to protected `production` coordinates one release for x86-64 and
AArch64. Each architecture produces:

- one bootc OCI image stored in GHCR;
- one network installer ISO and checksum stored in GitHub Releases;
- one compressed reusable QCOW2 and checksum stored in GitHub Releases;
- one strict release record and signing bundle; and
- image signatures and provenance attached to the exact OCI digest.

GHCR images are first-class update artifacts. GitHub Releases stores downloads.
OIDC supplies short-lived authentication and stores nothing.

Version and product identity derive from `distro/soda.toml`. The protected
production commit, Git tag, both OCI digests, artifact names, checksums, signed
records, release notes, and remote assets must agree.

## Evidence before release CI

Expensive product validation runs before the production push on user-controlled
matching-native machines. It covers:

- graphical one-ISO installation;
- native installation, mandatory welcome and Cockpit Tailscale on ISO and reusable QCOW2;
- LAN and cloud/Tailscale access;
- identities, Forgejo SSH keys, Projects, workspaces, `mise`, and deletion;
- manual update and account-preserving fallback; and
- absence of forbidden Soda control planes and copied credentials.

Fallback uses the previous signed published OCI image by immutable digest. Do
not rebuild the previous image or any unused historical ISO/QCOW2.

The completed sibling runs are submitted to the maintained `Native acceptance
evidence` workflow on exact `main`, together with the strict AArch64 candidate
release record. Each decoded input is limited to 12 KiB. The workflow requires
both summaries' source and suite revisions to equal its own source SHA, binds
the AArch64 candidate digest to its schema-3 release record, and produces one
strict JSON acceptance record containing:

- schema;
- exact source commit;
- acceptance-suite revision or digest;
- both architectures;
- required scenario names and pass results;
- previous fallback OCI digest;
- completion time; and
- approved signer identity.

The record is signed and verified through Cosign/Sigstore with the fixed
`native-acceptance-evidence.yml@refs/heads/main` workflow identity. The two
summaries, AArch64 release record, combined record, and signature bundle remain
available as a one-day Actions artifact. The workflow receives no credentials,
runs no VM, publishes no image, and creates no release. Its record authenticates
the claim about the pre-release runs; it does not claim that any later CI-built
bytes were boot-tested. Soda creates no attestation service.

## Build-once production workflow

Release CI follows this order:

1. Verify protected branch identity, clean source identity, version, collision
   state, and the signed acceptance record.
2. Run cheap source and unit checks once.
3. Start x86-64 and AArch64 matching-native build jobs in parallel.
4. Build each architecture's release image B exactly once.
5. Structurally inspect that OCI output and publish its immutable candidate
   digest to GHCR.
6. Build the network ISO and raw/compressed QCOW2 from that same OCI output.
7. Check only artifact structure, architecture, identity, checksum, size,
   signature, provenance, and remote publication facts.
8. Promote the accepted OCI digests to the release's immutable architecture
   tags.
9. Create and sign the strict release records.
10. Create the Git tag and draft GitHub Release, upload both architecture asset
    sets, and re-read every remote fact.
11. Publish the draft only after all OCI, file, record, signature, provenance,
    source, and production-branch identities agree.

The locked release account allocates each build's native temporary directory
directly beneath its home and links it from the immutable run directory. This
keeps QEMU monitor sockets below the host's Unix-socket path limit while the
source checkout, Go cache, artifacts, and the link identifying that temporary
directory remain grouped with the exact run.

CI publishes the exact checked files unchanged. It never rebuilds a release
copy and never builds fallback A.

Release CI runs no QEMU, graphical installation, first-boot provisioning,
product acceptance, update/fallback suite, NoCloud, ConfigDrive, or
acceptance-only Tailscale enrollment. It receives no guest Tailscale keys.

## Native ISO candidate preparation

**Execution status:** the development check image and unchanged complete
`just check` passed on a native AArch64 development snapshot on this MacBook
(2026-09-05). Candidate orchestration still has only mocked execution tests;
Soda artifact construction, publication and graphical installation remain
unverified for this workflow. Native x86-64 execution awaits matching hardware.
Commands below build and publish artifacts; documentation is not operational
authorization.

Select one architecture explicitly on matching hardware:

```sh
# On an AArch64 build machine (Linux, or this Apple Silicon MacBook):
scripts/prepare-native-iso-candidate.sh aarch64

# On a native x86-64 build machine, when available:
scripts/prepare-native-iso-candidate.sh x86_64
```

**Candidate preparation publishes an OCI image to GHCR.** It is neither a
local-only builder nor a signed release. Both architectures use the same
orchestration and their own existing platform locks. No sibling-architecture
emulation is supported.

### Prerequisites and verification boundary

- A clean Git checkout, including untracked files, with `HEAD` equal to the
  local `origin/main` reference. The command does not fetch, update refs,
  commit, or exclude investigation files. Resolve source identity explicitly
  before executing it; a local tracking ref is not a live remote-branch check.
- Bash, Go, Git, just, jq, Skopeo, `sha256sum`, the pinned Vite+/Node toolchain,
  and ordinary shell utilities on the build host. Frontend assets are rebuilt
  by the existing RPM builder on that host; check-container output is discarded.
- A local Docker context with a matching Linux daemon and a running, single
  integrated Buildx `docker` worker on that same endpoint. The wrapper checks
  daemon OS/architecture and worker driver/endpoint, not its advertised platform
  list. It binds subsequent calls with process-local `DOCKER_CONTEXT` and
  `BUILDX_BUILDER`; it changes no global selection. Use a context rather than
  `DOCKER_HOST`. Remote and independently managed workers are currently rejected
  because this small wrapper does not establish their native execution identity.
- The selected platform's exact locked inputs, native build capacity, network
  access for existing input fetchers, and Skopeo credentials permitting GHCR
  candidate publication. Anonymous retrieval must also work after publication.
  If the locked Fedora manifest is no longer available remotely, supply the
  exact retained archive matching the platform's archive checksum. Do not
  substitute a current Fedora tag or another base. Loading the retained archive
  remains owned by the existing builder.

`scripts/check-native.sh <architecture>` owns the source gate. On Linux it runs
unchanged `just check` directly; invoke source verification as an ordinary user,
not root. On Darwin it builds the development-only
`tools/check/Containerfile` using Go from `go.mod`, the existing Vite+ installer,
Node from `cockpit/.node-version`, and Linux source-check dependencies. It then
runs unchanged `just check` inside a disposable native Linux container as the
unprivileged `check` account (UID 1000). Its Vite+ binary uses the Linux install
path under that account's `.local/share/vite-plus/bin`; `jq` is installed for the
script tests. This requires dependency downloads and writes Docker image/cache
data; it does not publish images. Its development base is unrelated to the
locked Soda bootc base.

The canonical source is mounted read-only. A private exact-commit Git clone in
the container's writable Linux filesystem receives generated frontend files and
caches. That clone excludes host `node_modules` and ignored generated outputs;
the read-only canonical mount is used to clone Git objects, not as the test
working directory. No separate credential directory, Docker socket, writable
host output mount, or privileged mode is passed into the check container. Linux verification
is not replaced with Darwin implementations or a reduced test suite. The
canonical checkout must remain clean at the same revision after verification;
there is no success stamp or skip flag.

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

### Construction and publication order

The candidate command refuses existing final OCI, ISO, or ISO checksum paths
(including dangling symlinks), and fails closed if candidate-tag enumeration
fails. Lower-level builders retain their existing scratch-directory behavior;
use one preparation process per checkout, not concurrent builds in shared
scratch paths.

It runs the complete Linux gate, then `just oci` once, which already builds the
RPMs. It checks OCI OS, architecture, version and source labels, loads the
archive, verifies its image ID against the manifest's config digest, and runs
that exact image without pulling. Linux runtime architecture, `os-release`, and
Soda RPM versions must pass before publication.

Next, `soda-release image-stage` publishes
`ghcr.io/levitateos/soda-os:sha-<full-source-revision>-<architecture>` without
replacing an existing candidate. Preparation immediately reports confirmed
publication, verifies anonymous remote/local manifest digest equality, and runs
`just iso` against that archive. Existing deep Linux squashfs/initramfs,
configuration and branding inspection remains mandatory. The wrapper also checks
the ISO checksum sidecar and exact published-digest installer source. Its final
summary reports the revision, architecture, tag, digest, ISO and checksum—not
successful installation.

**Publication can succeed and ISO construction can fail.** On failure the
command reports whether publication was attempted or confirmed, plus the
candidate tag and expected digest. A failed publication command can itself have
changed remote state. Stop and inspect that state explicitly: there is no
retry, deletion, compensation, reconciliation or persisted workflow state.
A full rerun will refuse existing candidate tags or final output paths.

### Independent Linux/libvirt placement

On a matching Linux destination, place an existing ISO separately:

```sh
scripts/place-libvirt-iso.sh aarch64 <verified-ISO> <destination-directory>
scripts/place-libvirt-iso.sh x86_64 <verified-ISO> <destination-directory>
```

Placement requires the source `.sha256` sidecar, an existing writable destination
with appropriate permissions/SELinux policy, GNU `stat`, passwordless sudo for
the required `qemu`-account operations, and the matching `qemu-system-*` binary.
It validates the source checksum, actual destination path traversal, no-overwrite
copy, destination checksum, `qemu` readability, `virt_image_t`, and a non-booting
QEMU open. It neither builds nor publishes nor repairs permissions or labels.
A failure after copying starts may leave a partial or complete destination; the
command reports it and does not remove or overwrite it automatically.

### Installation evidence remains separate

This Apple Silicon MacBook is both the planned AArch64 build and installation
test machine. No Mac Mini or separate general-purpose Linux build VM is required.
Only a demonstrated Docker Desktop limitation would justify evaluating the
latter. Native x86-64 execution remains pending matching hardware, independently
of AArch64 progress.

For the first bounded local installation test, use the existing ARM QEMU/HVF,
UEFI, VirtIO and Cocoa configuration in `internal/acceptance/qemu.go`, reviewed
against the resulting ISO. The full `soda-acceptance` CLI is not an ISO-only
launcher: it additionally requires QCOW2/fallback artifacts and credentials.
Do not invoke that larger suite implicitly. VM creation/boot needs separate
approval, and the network ISO needs access to its exact published OCI digest.
Verify graphical Anaconda, reboot, installed identity and native onboarding;
loopback user-network forwarding is not proof of general LAN reachability.

Candidate preparation and placement create no installation VMs, reusable
QCOW2s, release records, signatures, version tags or GitHub Releases.

## Publication boundaries

Soda release tooling remains a fixed wrapper around Git, Skopeo, Cosign, and
GitHub CLI. Those tools own authentication, transport, registry, Sigstore, and
GitHub protocols. Soda validates its own fixed inputs and resulting remote
facts.

Known input errors and collisions fail before mutation. If an external action
partially succeeds, report the exact remote state and stop. Do not overwrite,
delete, compensate, retry automatically, or reconcile partial state.

The release has a 30-minute wall-clock target. At 45 minutes, the workflow must
identify the active or slow stage and continue. This is a warning, not a
timeout.

## Final publication gate

Publication refuses to proceed unless:

- the signed acceptance record matches the exact source commit;
- each architecture built B once on matching-native hardware;
- each exact OCI digest is anonymously retrievable;
- image signatures and provenance verify against the release workflow;
- ISO and QCOW2 structures, architectures, identities, and checksums pass;
- signed records bind the correct source, architecture, base, image, and file
  checksums;
- all expected remote assets exist once with the exact checked bytes;
- release notes name both exact OCI update digests; and
- the remote `production` head remains the original release commit.

No moving OCI tag policy is implied without a separate decision. No release
daemon, workflow database, credential store, retry engine, or reconciliation
system is allowed.

## Current implementation

At checkpoint `5cf31df`, the repository contains release tooling, a production
workflow, strict records, OCI/ISO/QCOW2 construction, Cosign integration, and
matching-native executor support. It still performs work rejected by the
contract above:

- rebuilds fallback A from source;
- runs installed VM and B-to-A-to-B acceptance in paid release CI;
- creates acceptance-only Tailscale guest keys;
- exercises NoCloud and ConfigDrive provisioning; and
- repeats source checks across architecture/build phases.

The current workflow and operator commands must not be used as release-day
instructions until those differences are removed and the signed pre-release
record boundary is implemented. No public `0.5.0` release is claimed here.
