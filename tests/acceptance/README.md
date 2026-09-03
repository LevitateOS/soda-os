# Single-run installed-product acceptance

The product outcomes and acceptance criteria are governed by
[architecture-reset.md](../../docs/architecture-reset.md). This directory owns
one public raw-QEMU workflow:

```text
tests/acceptance/unattended.sh run
```

Run it independently on matching-native x86-64 and AArch64 hardware. Evidence
from one sibling architecture does not qualify the other.

## Inputs

The runner requires a candidate B ISO, OCI archive, reusable QCOW2, and schema-3
release record; an earlier A OCI archive and release record for native fallback;
three fresh single-use Tailscale keys in protected regular files for the ISO,
NoCloud, and ConfigDrive machines; and a new evidence path.

```sh
tests/acceptance/unattended.sh run \
  --evidence-dir .artifacts/acceptance/run-$(date -u +%Y%m%dT%H%M%SZ) \
  --candidate-iso .artifacts/images/SodaOS-0.5.0-x86_64.iso \
  --candidate-record .artifacts/releases/soda-os-0.5.0-x86_64.release.json \
  --candidate-oci .artifacts/images/soda-os-0.5.0-x86_64.oci.tar \
  --candidate-qcow2 .artifacts/images/SodaOS-0.5.0-x86_64.qcow2 \
  --fallback-record .artifacts/fallback/soda-os-0.5.0-x86_64.release.json \
  --fallback-oci .artifacts/fallback/soda-os-0.5.0-x86_64.oci.tar \
  --tailscale-auth-key-file /secure/iso-auth-key \
  --nocloud-tailscale-auth-key-file /secure/nocloud-auth-key \
  --configdrive-tailscale-auth-key-file /secure/configdrive-auth-key
```

The evidence directory must not already exist. The key file must be regular,
non-symlink, and inaccessible to group and other users. Neither the password nor
the Tailscale key is accepted through argv or environment values.

Host prerequisites are `curl`, Docker, Go, `jq`, OpenSSL, QEMU, `qemu-img`,
`sha256sum`, OpenSSH clients, and `xorriso`. Docker may be available directly or
through passwordless sudo. The runner uses the exact registry and Skopeo tool
containers pinned beside it; the registry binds only host loopback and is
removed at the end. A matching-native host `skopeo` is used when already
available. The runner publishes no image or release.

## Owned workflow

One process owns the complete lifecycle:

1. Validate matching-native release records, the candidate ISO checksum, OCI
   files, and protected credential input.
2. Generate a fresh administrator password and key and create protected OEMDRV
   answer media through `soda-image installer-input`.
3. Create generated administrator credentials, OEMDRV media, and the qcow2 disk
   in one protected disposable host directory outside the evidence tree. Start
   one exact disposable registry, copy A and B with preserved manifest
   digests, and expose it only to QEMU's host endpoint.
4. Create one fresh qcow2 disk and install candidate B through raw QEMU.
5. Require the guest-requested OEMDRV ejection, remove the medium from its open
   QMP tray, and delete only that exact answer image.
6. Compare host-visible Tailnet peers before and after installation, identify
   the single newly enrolled Soda node, then prove direct administrator SSH and
   stock Cockpit through its Tailnet address. `SODA_ACCEPTANCE_GUEST_HOST` may
   instead require a specific Tailnet IP or MagicDNS name; no unclaimed hostname
   is assumed and QEMU's loopback forwards are not used as product evidence.
7. Seed current authoritative Linux, catalog, workspace, Forgejo, Tailscale,
   password, group, home, key, and host-key state on B.
8. Select exact A with native `bootc switch --download-only` followed by
   `bootc switch --from-downloaded`, reboot, and compare normalized current
   mutable state.
9. Select exact B the same way, reboot, compare again, and remove the disposable
   guest registry configuration.
10. Exercise product behavior and capture the final installed-product evidence.
11. Provision fresh copies of the candidate QCOW2 through NoCloud and
    ConfigDrive, proving disk growth, native onboarding, Tailnet exposure,
    Projects/workspace behavior, Tea, and removal of cloud-init secret state.
12. Boot another fresh QCOW2 without a datasource, prove ordinary startup and
    the absence of a new Tailnet node, and perform no provisioning.
13. Shut down every VM cleanly, scan retained evidence for all generated
    passwords, private keys, and Tailscale keys, then remove the exact
    disposable runtime directory and registry.

The private non-executable scripts below `tests/acceptance/internal/` are
implementation details. They are not alternative public workflows, do not
create `runner.env`, and require no two-terminal coordination or VNC.

## Product scenarios

The single run proves:

- stock Anaconda/Kickstart installation, the initial Linux administrator,
  standard authorized key, native Forgejo administrator, one-attempt Tailscale
  enrollment, secret absence, and installed image digest;
- stock Cockpit package discovery, Soda branding and Projects, primary login,
  and workspace-account PAM rejection;
- the exact sorted three-field catalog and edit-without-reconciliation behavior;
- missing-key failure before workspace-account mutation;
- native empty Forgejo creation and canonical-repository preservation;
- administrator-only Add person creation of ordinary primary accounts, followed
  by native PAM Forgejo creation and private Tea login publication;
- wrong-password rejection and correct-password PAM authentication for later
  primary humans, with no local Forgejo password verifier and with `wheel`
  changes remaining non-administrative;
- installer-administrator, primary-human, and derived-workspace Tea identity,
  including one-time opaque configuration copying and distinct human tokens;
- Tea-authenticated repository, issue, pull-request, and release creation under
  the fixed human-owned token scopes;
- workspace-account Forgejo PAM rejection and independent Forgejo-user
  persistence after Linux account deletion;
- one complete clone and derived Linux account per selected human-project pair;
- distinct Alice and Bob UIDs, homes, checkouts, local files, and processes for
  the same project;
- ordinary direct OpenSSH command, SCP, and SFTP behavior as a derived UID;
- non-conflicting project-selected host ports without Soda port state;
- Soda-aware cascading human deletion with the primary account removed last;
- generic Linux account deletion remaining non-cascading;
- project removal deleting derived accounts and catalog state without deleting
  the canonical Forgejo repository;
- the complete immutable command manifest, representative Go, Python, Rust,
  Node, Bun, C, and C++ execution, and rootless Podman for primary and derived
  accounts;
- native Linux/Cockpit ownership and absence of the removed installer add-on,
  dashboard, forced SSH, telemetry, updater, runtime toolchain state, daemon,
  general CLI, control socket, API group, daemon logs, and protobuf/gRPC
  boundary.

The runner proves the authorized Forgejo password-verifier boundary directly:
`/etc/shadow` has the dedicated group and exact mode, `git` is not an NSS group
member, only `forgejo.service` receives the supplementary group, the PAM rules
retain primary/workspace classification, and SELinux remains enforcing. A
protected later-primary password fixture reaches `chpasswd` and Forgejo only on
stdin; it is never placed in argv, environment values, logs, or normalized
evidence.

## Failure and evidence

Known collisions and invalid inputs fail before mutation. The runner does not
retry, repair, compensate, reconcile, or keep durable workflow state. A failed
run retains redacted evidence for diagnosis, but removes its generated
credentials, OEMDRV image, and qcow2 disk; repetition requires a fresh evidence
directory and runtime inputs. Any credential material detected in retained
evidence is redacted and makes the run fail.

Normalized fallback manifests deliberately exclude boot IDs, timestamps, PIDs,
logs, WAL bytes, and deployment selection. They include current account fields,
hashed shadow records, groups, home and key facts, the catalog, workspace trees
and Git state, native Forgejo user facts, the shadow-access contract, Tailscale
identity and Fedora-owned state path, SSH host keys, and automatic-update timer
state. Raw password hashes and credentials are never written to evidence.

Final capture requires installed absence of the deleted control plane and
proves that `soda-runtime` owns no daemon or API artifacts. Final issue #25
closure occurs only after this post-capstone run passes on both matching-native
architectures.

The final workflow passed on native x86-64 at candidate B `0d6ca31` and fallback
A `ba4de43`, including Add person and Tea onboarding, PAM users without local
verifiers, B→A→B preservation, all retained product scenarios, credential
absence, and final control-plane absence. Matching-native AArch64 repetition
remains required for issue #44 and release-level completion.

## Architecture notes

x86-64 uses KVM and OVMF. The existing AArch64 launch implementation is for
matching-native Apple Silicon with HVF and Homebrew QEMU. A matching-native
Linux AArch64 runner must supply its native QEMU/firmware boundary rather than
reusing or inspecting x86-64 artifacts. Temporary host paths are test
infrastructure facts, not Soda product requirements.
