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

## Temporary serial first-fallback bootstrap

The first ordinary signed release needs one previous signed image per
architecture, but no signed Soda image predates 0.5.0. The owner selected
artifacts built from `f212ed9f61eaae22587f484d507adae1f077bfe4` and a
separate 0.5 release line so that each native architecture can qualify its
retained artifact without blocking development on the other architecture.

The first reviewed release-line commit contains only the retained x86-64
digest:
`ghcr.io/levitateos/soda-os@sha256:d57060e9eb5953043e7ce18b8e002422010f6e17c1408211907d31fd1cfa5edd`.
Creating protected `production` at that exact commit triggers one x86-64 job.
The job pulls and checks the committed digest as `linux/amd64`, refuses unknown
or conflicting signature state, signs it with the production workflow OIDC
identity, and immediately verifies the exact workflow claims. Re-running the
same commit is idempotent.

This bootstrap accepts no inputs and contains no normal release jobs. It
creates no version image tag, Git tag, attestation, draft, release asset, or
public release. A later reviewed fast-forward commit substitutes the exact
native AArch64 digest and job. Subsequent 0.5 release work removes the
bootstrap entirely before constructing or publishing 0.5.0.

## Evidence before release CI

Expensive product validation runs before the production push on user-controlled
matching-native machines. It covers:

- graphical one-ISO installation;
- the current Soda Setup journey on ISO and reusable QCOW2;
- LAN and cloud/Tailscale access;
- identities, Forgejo SSH keys, Projects, workspaces, `mise`, and deletion;
- manual update and account-preserving fallback; and
- absence of forbidden Soda control planes and copied credentials.

Fallback uses the previous signed published OCI image by immutable digest. Do
not rebuild the previous image or any unused historical ISO/QCOW2.

The completed sibling runs produce one strict JSON acceptance record for the
exact source commit. It contains:

- schema;
- exact source commit;
- acceptance-suite revision or digest;
- both architectures;
- required scenario names and pass results;
- previous fallback OCI digest;
- completion time; and
- approved signer identity.

The record is signed and verified through Cosign/Sigstore. It authenticates the
claim about the pre-release runs; it does not claim that the later CI-built
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

CI publishes the exact checked files unchanged. It never rebuilds a release
copy and never builds fallback A.

Release CI runs no QEMU, graphical installation, first-boot provisioning,
product acceptance, update/fallback suite, NoCloud, ConfigDrive, or
acceptance-only Tailscale enrollment. It receives no guest Tailscale keys.

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
