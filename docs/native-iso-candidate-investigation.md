# Shared native ISO candidate workflow investigation

Recorded: 2026-09-05.

Status: investigation and proposed design. Implementation, artifact builds,
publication, VM creation, and host changes have not been approved or performed
as part of this investigation. This document records findings; it does not
authorize the proposed operations.

## Purpose and current user direction

Replace these x86-64-specific files with a small shared workflow for native
AArch64 and x86-64 Linux ISO candidates:

- [`scripts/prepare-x86_64-cockpit-iso-candidate.sh`](../scripts/prepare-x86_64-cockpit-iso-candidate.sh)
- [`scripts/prepare_x86_64_cockpit_iso_candidate_test.go`](../scripts/prepare_x86_64_cockpit_iso_candidate_test.go)

Both architectures are equal siblings. Architecture-specific input preparation,
builds, execution, publication, inspection, and installation require matching
hardware, without sibling-architecture emulation.

The user clarified that **this MacBook is the only machine currently available
for testing**. It is therefore both the AArch64 build machine and the AArch64
installation test machine. A Mac Mini is a possible later destination, not a
prerequisite. Its chip, host OS, and VM application are not confirmed and do not
need to be selected before local AArch64 work can proceed.

Native x86-64 artifact and installation validation remains pending until a
matching machine is available. It must not be replaced with emulation or made a
prerequisite for performing the AArch64 work available here.

Keep these distinctions explicit:

- A **Linux check container** supplies Linux execution for source verification.
- Docker Desktop's existing **ARM Linux backend VM** runs native build containers.
- An **ARM installation test VM** on the MacBook runs the resulting Soda OS.
- A separate general-purpose **Linux build VM** is an option only if a concrete
  limitation of Docker Desktop justifies it; it is not a settled requirement.

The governing product sources are [AGENTS.md](../AGENTS.md),
[principles](principles.md), and the [architecture reset](architecture-reset.md).
Current tools and historical artifacts are implementation evidence, not new
product constraints.

## Confirmed checkout and machine

These are observations from this investigation, not permanent prerequisites.

| Item | Observed value |
| --- | --- |
| Checkout | `/Users/vince/Projects/soda-os` |
| Branch | `main` |
| HEAD | `fbc3082357fa81e6f5361e52c9abcdb796686af3` |
| Local `origin/main` | Same revision as HEAD; no fetch performed |
| Working tree | Clean before and after investigation |
| Computer | Apple M4 MacBook Air, model `Mac16,12` |
| Computer name | `vince’s MacBook Air` |
| Host memory | 16 GiB |
| Host OS | macOS 26.6.2, build `25G83` |
| Kernel and architecture | Darwin 25.6.0, `arm64` |
| Go | `go1.27.0 darwin/arm64`; repository declares Go 1.26.7 |
| Docker Desktop | 4.89.0 |
| Docker Engine | 29.7.2 |
| Active Docker context | `desktop-linux`, local Unix socket |
| Docker daemon | Linux/ARM64, LinuxKit kernel `7.0.12-linuxkit` |
| Backend resources | 10 virtual CPUs, 8 GiB configured RAM |
| Backend virtualization | Apple Virtualization Framework, confirmed by launch log |
| Host file sharing | VirtioFS, including `/Users`, confirmed by launch log |
| Image store | containerd snapshotter; reported driver `overlayfs` |
| Selected Buildx builder | `desktop-linux`, integrated `docker` driver, same daemon |
| BuildKit | v0.32.2 |
| Emulation availability | Rosetta enabled; Buildx advertises additional architectures |
| QEMU | `qemu-system-aarch64` 11.1.1 installed |
| ARM UEFI | `/opt/homebrew/share/qemu/edk2-aarch64-code.fd` exists |
| Filesystem capacity | Approximately 61 GiB available on the checkout filesystem at inspection |
| Running Docker containers | None at inspection |

Docker, Go, `just`, `jq`, Skopeo, `sha256sum`, and `shasum` were available.
Podman, Colima, Lima, and OrbStack commands were not found in the inspected PATH.
The existing build path uses Podman inside containers, so a missing host Podman
command is not by itself a blocker.

The Buildx `docker` driver uses BuildKit integrated into the Docker Engine. For
this observed configuration, the selected worker and native ARM Linux daemon
are connected directly. A supported-platform list alone does not prove native
execution: this machine also advertises emulated architectures.

The current daemon and image-store observations make the existing OCI exporter
path plausible. No OCI export was executed during this investigation. Neither
the RAM allocation nor host free-space observation proves sufficient capacity
for the complete build.

## Existing workflow and ownership

### Shared lower-level construction already exists

The [justfile](../justfile) already accepts architecture arguments for `rpm`,
`oci`, and `iso`. `soda-release image-stage` also accepts either architecture.
The architecture contract and selected inputs are represented in
[`internal/config/config.go`](../internal/config/config.go) and:

- [`distro/platforms/aarch64.toml`](../distro/platforms/aarch64.toml)
- [`distro/platforms/x86_64.toml`](../distro/platforms/x86_64.toml)

The small mapping needed by shell orchestration is:

| Soda architecture | Matching host machine names | OCI platform | RPM suffix |
| --- | --- | --- | --- |
| `aarch64` | Linux `aarch64`; macOS `arm64` | `linux/arm64` | `aarch64` |
| `x86_64` | `x86_64` | `linux/amd64` | `x86_64` |

No default architecture is needed. Platform locks and installer configuration
should remain with their current owners rather than being copied into a new
platform framework.

`BuildImage` calls `buildRPMs` before OCI construction. Consequently, the
wrapper's `just rpm` followed by `just oci` repeats RPM construction. The shared
wrapper should call `just oci` once. See
[`internal/build/image/builder.go`](../internal/build/image/builder.go).

### The current wrapper combines two responsibilities

The existing script:

1. Requires x86-64 hardware and a fixed set of host tools.
2. Requires a clean checkout, including untracked files, and HEAD equal to the
   local `origin/main` reference.
3. Derives the Soda version and source-revision candidate tag.
4. Checks candidate-tag and destination availability.
5. Requires local libvirt destination traversal as the `qemu` account.
6. Runs `just check`, `just rpm`, and `just oci`.
7. Publishes the immutable GHCR candidate using `image-stage`.
8. Compares the anonymous remote manifest digest with the local OCI digest.
9. Checks OCI OS, architecture, version, and source revision.
10. Builds and inspects the network-install ISO with `just iso`.
11. Checks its checksum sidecar and exact digest-bound installer source.
12. Loads and runs the OCI image to check `os-release` and Soda RPM versions.
13. Copies the ISO without overwrite, compares checksums, and verifies libvirt
    traversal, readability, SELinux label, and a non-booting QEMU open.

Its build architecture, OCI architecture, Docker platform, RPM suffix, QEMU
binary, and output text are hardcoded. Placement assumes `/home/libvirt/images`,
the `qemu` account, passwordless sudo, GNU `stat`, and `virt_image_t`. Even the
destination override retains checks against `/home` and `/home/libvirt`.

Those destination requirements do not belong in candidate construction on the
MacBook. They do not establish a requirement for Cockpit/libvirt on a future
Mac Mini either.

### Publication is an explicit external mutation

[`internal/build/release/image_publication.go`](../internal/build/release/image_publication.go)
validates the native architecture, clean source identity, local OCI contents,
and digest. It uses Skopeo to list existing tags, publish the archive, and verify
the resulting digest. The candidate name is:

```text
ghcr.io/levitateos/soda-os:sha-<full-source-revision>-<architecture>
```

The outer wrapper additionally proves anonymous public retrieval and local versus
remote digest equality. Preserve those checks. This command is not a local-only
builder, and candidate preparation is not a complete signed release operation.

Publication currently precedes ISO construction. A later failure leaves the
immutable candidate tag in GHCR and prevents a simple full rerun. Keep that
visible failure boundary. Do not overwrite, delete, compensate, automatically
retry, or reconcile the published state. See
[release operations](release-operations.md#publication-boundaries).

### Existing no-overwrite protection has a specific scope

The wrapper protects the immutable remote candidate tag and the copied
destination ISO. The underlying OCI and ISO builders deliberately remove
existing local output files before rebuilding them.

The proposed candidate wrapper should additionally refuse existing final OCI,
ISO, and checksum paths before invoking those builders. This would make the
candidate command's local no-overwrite behavior explicit without changing the
lower-level builders' scratch/output behavior.

## Linux source-check execution boundary

The complete `just check` recipe includes formatting, source/complexity/release
checks, the frontend lifecycle, `go vet ./...`, the acceptance race tests,
`go test ./...`, and static image configuration checks for both architectures.

Production sources directly use Linux-only APIs, including `unix.Openat2`,
`OpenHow`, `RESOLVE_*`, and `Renameat2`. Examples are
[`internal/linuxhost/descriptors.go`](../internal/linuxhost/descriptors.go) and
[`internal/projects/workspace/descriptors.go`](../internal/projects/workspace/descriptors.go).
A native Darwin compilation attempt reproduced those missing API errors.

This is a Linux execution boundary, not a reason to add macOS substitutes to
Linux product code, exclude runtime packages, or weaken verification gates.

Conversely, `soda-image --architecture <architecture> check` is static source and
configuration validation. It does not call Docker or execute artifact work and
does not invoke the native build guard. Both architecture-static checks passed
on this Mac. Those results are not artifact or platform-support evidence.

Current native build guards compare `runtime.GOARCH` with the selected
architecture. Darwin ARM64 therefore passes the AArch64 hardware guard, which is
reasonable for Docker-backed construction. The guard does not verify the Docker
daemon or Buildx worker, and does not make Darwin a Linux test environment.

### Recommended check arrangement

Add a synchronous `check-native.sh` helper:

- On matching Linux, execute the existing complete `just check`.
- On this Mac, execute it inside a disposable native ARM Linux check container
  on the verified Docker Desktop backend.
- Mount the canonical source read-only and create an exact-commit verification
  copy inside the container's disposable Linux filesystem. Include the Git
  metadata needed by source checks. Do not share macOS `node_modules` or overwrite
  the Mac checkout's generated frontend files with Linux output.
- Give the check container writable temporary source/output/cache locations,
  without privileged mode or the host Docker socket.
- Continue candidate construction only after exit status zero, then recheck the
  canonical source revision and cleanliness. Do not pass success through a
  persisted stamp or a skip flag.

This is a proposed verification copy, not a new editing worktree. No such copy
or container was created during this investigation.

The existing RPM builder image is not a complete check environment. Its exact
package inventory includes Go/GCC/Git/RPM tools but lacks several full-check
prerequisites, including `just`, Vite+, Node, and `rsvg-convert`. Keep that image
focused on RPM construction.

A small development check Containerfile should use Go consistent with `go.mod`,
the established Vite+ 0.3.0 bootstrap, Node 24.20.0, and the required source-test
tools. Reuse version choices from
[`scripts/install-cockpit-toolchain.sh`](../scripts/install-cockpit-toolchain.sh),
[`cockpit/package.json`](../cockpit/package.json), and
[CI](../.github/workflows/ci.yml), rather than introducing independent moving
toolchain choices. Exact image construction still needs implementation and
native validation.

The recommendation does not require changing the `just check` body, CI,
production runtime code, or RPM builder locks.

## Exact base input and fresh-machine preparation

The AArch64 platform currently locks:

```text
Manifest:
sha256:950a52fa1244db4d7fe2673af57fd6784a605a83bec3cd2d716ed8c00ebd366d

Archive:
distro/base/fedora-bootc-44-aarch64/Fedora-bootc-44.20260829.0-aarch64.oci-archive.tar

Archive SHA-256:
6805e85fda436c1afd8f9c540a05f5028982b44b965ae1ff6ca249702a328c7d
```

The archive exists on this MacBook and its full SHA-256 was verified. Git tracks
the reference metadata and checksum, but ignores the archive itself. The
metadata describes an OCI archive with Docker-compatible manifest metadata.

The exact image was not loaded in the current Docker store. A read-only Skopeo
manifest lookup at Quay returned `manifest unknown`. That is a current registry
retention problem, but not a missing-input blocker here because the retained
archive is present and checksum-valid.

[`PrepareLocalBootcBase`](../internal/build/image/bootc_base.go) first attempts
to resolve the exact image locally. Otherwise, it verifies the archive checksum,
loads it, resolves the pinned image, and assigns a digest-derived local tag.
The load/bind path was inspected but not executed in this investigation.

Fresh machines need an explicitly supplied copy of the exact retained archive,
verified against their selected platform contract. No existing base-fetch
helper was found that solves distribution of those retained bytes. Re-pulling
a moving Fedora tag is not an equivalent input, and recreating an archive is
not automatically proof of the locked archive checksum. Do not invent a mirror,
change a lock, or publish retained inputs implicitly.

Other input fetching already has helpers for Forgejo, GitHub Runner, mise, and
Tea. These remain owned by the existing `just oci` prerequisites. Their current
network availability was not comprehensively tested.

## Docker Desktop artifact compatibility

Existing RPM commands run in selected-platform Linux containers using the host
UID/GID and a bind-mounted checkout. OCI construction uses Buildx with the
selected platform and pinned local base context. Historical local logs record
a successful native AArch64 RPM build at source
`8c1d1a2a7b7d9ab96b835adf8f1ff85cc4cdcdf6`. That demonstrates an earlier run,
not the current candidate, current full check gate, or ISO construction.

ISO construction uses the locked Image Builder, not a replacement macOS ISO
implementation. It runs privileged Linux containers with a named volume at
`/var/lib/containers/storage`, then invokes `bootc-generic-iso`.
Inspection runs nested Podman to extract the squashfs and initramfs. Existing
checks validate the extracted installer defaults, exact payload reference,
configuration, branding, scratch mount behavior, and forbidden provisioning
content. The inspection path chowns extracted files back to the host UID/GID
and adjusts modes for reading. Preserve these checks.

Relevant owners:

- [`internal/build/image/rpm.go`](../internal/build/image/rpm.go)
- [`internal/build/installer/builder.go`](../internal/build/installer/builder.go)
- [`internal/build/installer/iso_inspection.go`](../internal/build/installer/iso_inspection.go)
- [`packaging/installer/Containerfile`](../packaging/installer/Containerfile)

The installer Containerfile also checks `/usr/lib/grub/i386-pc/boot_hybrid.img`
unconditionally. Both current platform locks include the relevant noarch GRUB
modules. This was not demonstrated to be an AArch64 blocker and should not be
removed merely because its path looks x86-specific.

Concrete execution questions still to validate on this MacBook are:

- selected-worker native execution and OCI export;
- privileged filesystem operations required by Image Builder;
- nested Podman and its container-storage access;
- required mount/loop facilities;
- readable ISO output and checksum creation on the host bind mount;
- extraction, numeric ownership changes, and cleanup through VirtioFS;
- sufficient backend disk, temporary storage, and memory for the actual build.

None was proved incompatible during this read-only investigation. Do not
disable inspection, silently ignore ownership errors, or replace Linux behavior
with Darwin behavior to make a wrapper appear portable. If an approved test
fails, diagnose that exact boundary before proposing a narrow change or a
separate ARM Linux build VM on the MacBook.

## Recommended candidate and placement commands

The shared candidate script should accept one explicit architecture and:

1. Verify matching hardware, Linux daemon architecture, and actual Buildx worker.
   Keep subsequent commands bound to that verified context/worker. An unknown
   worker must not be treated as native merely because it advertises a platform.
2. Check clean source identity, version, selected inputs, candidate-tag absence,
   and final output collisions.
3. Run the complete native Linux source gate synchronously.
4. Run `just oci` once; do not precede it with redundant `just rpm`.
5. Verify local OCI identity and execute the verified loaded image for Linux
   `os-release` and selected-architecture Soda RPM checks. Prefer binding runtime
   execution to the archive's verified image ID rather than relying only on a
   mutable version tag.
6. Publish explicitly through `image-stage` and compare the anonymously retrieved
   remote digest with the local digest.
7. Build and deeply inspect the ISO using `just iso`, preserving its exact
   published-digest binding and checksum verification.
8. Report source revision, architecture, candidate tag, published digest, ISO
   path, checksum, and installation source.

Moving runtime identity verification before publication catches that failure
before GHCR mutation. Publication still precedes ISO construction; a later
failure must report the exact partial state and stop without recovery machinery.

A separate Linux/libvirt placement command should accept an existing ISO and
destination directory. Preserve source-sidecar validation, no-overwrite copy,
source/destination checksum equality, actual path traversal, `qemu` readability,
SELinux `virt_image_t`, and a matching QEMU binary's non-booting open. Remove
assumed checks against `/home` when another destination is selected.

Placement must never build or publish. A placement failure is independent of
candidate creation, but may leave a copied or partial destination file; report
that state and retain no-overwrite behavior rather than cleaning it up or
claiming an automatic rerun will succeed.

Proposed commands below **do not exist yet and were not executed**:

```sh
# On this MacBook: builds artifacts and PUBLISHES the OCI candidate to GHCR.
/Users/vince/Projects/soda-os/scripts/prepare-native-iso-candidate.sh aarch64

# Only when a matching native x86-64 machine becomes available:
<confirmed-x86-checkout>/scripts/prepare-native-iso-candidate.sh x86_64

# Separately, if a matching Linux/libvirt destination is actually selected:
<destination-checkout>/scripts/place-libvirt-iso.sh \
  x86_64 <verified-ISO-path> /home/libvirt/images
```

Local MacBook installation testing does not require the libvirt placement
command. It should use the existing ARM QEMU/HVF acceptance path if validated.

## Proposed file changes

These paths describe the proposed implementation scope, not changes already made.

| File | Proposed change |
| --- | --- |
| `scripts/prepare-x86_64-cockpit-iso-candidate.sh` | Replace with `scripts/prepare-native-iso-candidate.sh`, without an obsolete forwarding wrapper |
| `scripts/prepare_x86_64_cockpit_iso_candidate_test.go` | Replace with `scripts/prepare_native_iso_candidate_test.go`, covering both architectures |
| `scripts/check-native.sh` | Add synchronous native Linux check execution |
| `scripts/check_native_test.go` | Add focused checks for its execution boundary |
| `tools/check/Containerfile` | Add the development check environment |
| `scripts/place-libvirt-iso.sh` | Extract destination-specific placement |
| `scripts/place_libvirt_iso_test.go` | Test placement checks and failures separately |
| `docs/release-operations.md` | Replace the x86-only instructions with prerequisites, both architectures, explicit publication, placement, and failure boundaries |
| `AGENTS.md` | Update the obsolete convenience-command example |

The existing Go artifact builders, publication implementation, platform locks,
runtime code, CI, and `just check` body need no changes merely to implement this
boundary. Any demonstrated lower-layer failure should result in a separately
explained, concrete adjustment, not speculative portability machinery.

## Native validation plan

### AArch64: entirely on the available MacBook

After separate approval for implementation and the relevant execution effects:

1. Test architecture mapping and wrong-host/daemon/worker rejection, source
   identity changes, tag and output collisions, command sequencing, runtime
   identity failure, digest mismatch, ISO failure after publication, and
   placement independence. Keep external operations mocked in source tests.
2. Build and validate the native Linux check environment. Run the unchanged
   complete `just check` under ARM Linux. A passing container check proves source
   verification, not installed Soda behavior.
3. Validate native container execution and the actual required privileged,
   nested-Podman, and bind-ownership operations using approved disposable work.
   No diagnostic should publish or start a VM implicitly.
4. Build one exact AArch64 OCI candidate using the retained locked base and
   selected locks. Check metadata and real Linux runtime identity.
5. Publish only with explicit approval. Verify anonymous digest equality, build
   the exact-bound network ISO, perform all existing inspection, and verify the
   checksum. Stop and report partial state if anything fails after publication.
6. Create an ARM64 installation test VM locally on the MacBook. The existing
   [`internal/acceptance/qemu.go`](../internal/acceptance/qemu.go) provides a
   Darwin AArch64 path using HVF, ARM UEFI, VirtIO, and Cocoa. It is an
   implementation starting point, not proof of current successful installation.
7. Boot the exact ISO, perform graphical Anaconda installation, reboot, and
   verify installed architecture, source/image identity and digest, Linux
   account creation, mandatory welcome, and current onboarding/access behavior.
   Apply the relevant installed scenarios from
   [native onboarding](native-onboarding.md). Keep any VM networking limitations
   distinct from claims of verified LAN behavior.

The network-install ISO needs access to its exact published OCI digest during
installation. QEMU's ability to open an ISO is not evidence that Anaconda boots,
installation succeeds, or Soda behaves correctly after reboot.

No Mac Mini is required for these steps. If one is used later, repeat the
destination-specific installation evidence there rather than treating MacBook
HVF results as proof of another VM application's compatibility.

### x86-64 sibling: pending matching hardware

When a native x86-64 machine is available, confirm its actual OS, Docker daemon,
Buildx worker, inputs, and installation destination first. Repeat the source
gate, artifact construction, publication, runtime identity, ISO inspection,
checksums, and graphical installation with its own selected locks and artifacts.

Mocked source tests may exercise both architecture command mappings on either
development machine. They are not sibling-architecture artifact execution or
native platform acceptance. No x86-64 build or VM emulation is proposed on this
MacBook.

For each real run, retain the exact source revision, selected lock/base
identities, hardware/backend architecture, OCI digest, ISO checksum, performed
checks, and observed installation results. These are evidence artifacts, not
resume state or a workflow database.

## Verification actually performed

| Check | Result and limit |
| --- | --- |
| Checkout/branch/status/HEAD/local origin inspection | Clean `main` at the recorded commit; no fetch |
| `bash -n scripts/prepare-x86_64-cockpit-iso-candidate.sh` | Passed; shell syntax only |
| Both `soda-image --architecture ... check` commands | Passed; static source/configuration checks only |
| `go test ./internal/config ./internal/build/image ./internal/build/installer ./internal/build/release` | Passed on Darwin; focused source tests, not real artifact execution |
| `go test ./internal/linuxhost -run '^$'` | Failed during Darwin compilation on Linux-only APIs; no runtime tests executed |
| `go test ./scripts -run 'TestPrepareX86CockpitISO' -count=1` | Two success-path tests failed because the fixture hardcodes missing `/usr/bin/sha256sum`; Docker/sudo/Go/just/publication operations were mocked |
| Retained AArch64 base archive SHA-256 | Matches the full selected-platform checksum |
| Exact base in Docker image store | Not present; no load attempted |
| Exact base manifest lookup at Quay | `manifest unknown`; no image pulled |
| Current privileged build, nested Podman, OCI export, ISO construction | Not executed |
| QEMU boot, graphical installation, installed product behavior | Not executed |

The test checksum fixture should use a portable test-side checksum calculation,
such as Go's standard library, rather than an absolute Linux utility path. This
is test-harness portability, not a macOS substitute for Linux product behavior.

Investigation operations were read-only Git inspection, source/configuration
reads, machine/Docker metadata inspection, official documentation lookup,
archive hashing, a read-only registry manifest request, and safe source tests.
Only disposable test files and normal test/compiler caches were used. No source
files were edited during the investigation; no artifact build, image load/pull,
publication, VM operation, host configuration change, commit, push, or fetch was
performed. The later user request to record the findings authorizes this
documentation file only.

## Remaining design questions

There is no outstanding destination decision that blocks the AArch64 proposal:
the user has selected this MacBook for both building and testing. Docker Desktop
compatibility and the existing local QEMU/HVF route need execution evidence after
approval, rather than another speculative environment decision.

The future Mac Mini environment and future x86-64 machine can remain unspecified
until those destinations are actually used. No automatic retries, reconciliation,
persistent workflow state, platform framework, or speculative compatibility layer
is proposed.

## Primary external references

These explain upstream mechanisms; none proves a successful Soda build or
installation on the observed machine.

- [Docker Buildx Docker driver](https://docs.docker.com/build/builders/drivers/docker/):
  integrated BuildKit and its relationship to the Docker Engine/context.
- [Docker bind mounts](https://docs.docker.com/engine/storage/bind-mounts/):
  daemon-side paths and Docker Desktop's host-file-sharing mechanism.
- [Docker Desktop macOS permissions](https://docs.docker.com/desktop/setup/install/mac-permission-requirements/):
  host permissions at the Desktop boundary.
- [Docker Desktop containerd image store](https://docs.docker.com/desktop/features/containerd/):
  image-store capabilities relevant to exporters.
- [Image Builder installation](https://osbuild.org/docs/developer-guide/projects/image-builder/installation/):
  AArch64/x86-64 containers and privileged Linux filesystem requirements.
- [Image Builder ISO contract](https://osbuild.org/docs/developer-guide/projects/image-builder/advanced/bootc/isos/):
  `bootc-generic-iso`, installer inputs, and required tools.

The current Image Builder documentation was used for the ISO path. Older
`bootc-image-builder` macOS examples are not proof of the current locked
`image-builder` execution path or this Docker Desktop backend.
