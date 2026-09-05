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

## Host-local Cockpit ISO candidate

On a native x86-64 build host, an operator can prepare one manually installable
Cockpit-selectable candidate ISO from the current exact `origin/main` revision:

```sh
scripts/prepare-x86_64-cockpit-iso-candidate.sh
```

The command refuses a non-x86-64 host, a dirty checkout, a `HEAD` that differs
from `origin/main`, an existing immutable GHCR candidate tag, or an existing
`/home/libvirt/images/SodaOS-<version>-x86_64.iso`. It runs the established
checks, builds the x86-64 RPM inputs and OCI archive through `just`, publishes
one source-revision candidate with `soda-release image-stage`, verifies the
anonymous GHCR digest, builds the matching network ISO, validates the installer
source and runtime identity, copies the ISO into `/home/libvirt/images`, and
checks libvirt/QEMU readability. It does not create VMs, QCOW2s, release
records, signatures, version tags, GitHub Releases, or source changes.

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
