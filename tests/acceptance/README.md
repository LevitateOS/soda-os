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

The runner requires a candidate B ISO, OCI archive, and schema-2 release record;
an earlier A OCI archive and release record for native fallback; one fresh
single-use Tailscale key in a protected regular file; and a new evidence path.

```sh
tests/acceptance/unattended.sh run \
  --evidence-dir .artifacts/acceptance/run-$(date -u +%Y%m%dT%H%M%SZ) \
  --candidate-iso .artifacts/images/SodaOS-0.4.0-x86_64.iso \
  --candidate-record .artifacts/releases/soda-os-0.4.0-x86_64.release.json \
  --candidate-oci .artifacts/images/soda-os-0.4.0-x86_64.oci.tar \
  --fallback-record .artifacts/fallback/soda-os-0.4.0-x86_64.release.json \
  --fallback-oci .artifacts/fallback/soda-os-0.4.0-x86_64.oci.tar \
  --tailscale-auth-key-file .tailscale_auth_key
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
3. Start one exact disposable registry, copy A and B with preserved manifest
   digests, and expose it only to QEMU's host endpoint.
4. Create one fresh qcow2 disk and install candidate B through raw QEMU.
5. Require the guest-requested OEMDRV ejection, remove the medium from its open
   QMP tray, and delete only that exact answer image.
6. Wait for the enrolled Tailnet identity, direct administrator SSH, and stock
   Cockpit.
7. Seed current authoritative Linux, catalog, workspace, Forgejo, Tailscale,
   password, group, home, key, and host-key state on B.
8. Select exact A with native `bootc switch --download-only` followed by
   `bootc switch --from-downloaded`, reboot, and compare normalized current
   mutable state.
9. Select exact B the same way, reboot, compare again, and remove the disposable
   guest registry configuration.
10. Exercise product behavior and capture the final installed-product evidence.
11. Shut down QEMU cleanly and remove only the exact disposable registry.

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
  dashboard, forced SSH, telemetry, updater, and runtime toolchain state; and
- the intentional pre-#39 health-only Soda daemon boundary.

The stopped later-primary Forgejo PAM `/etc/shadow` privilege decision is not
implemented or claimed by this runner. The installer-created administrator is
used for native Forgejo operations.

## Failure and evidence

Known collisions and invalid inputs fail before mutation. The runner does not
retry, repair, compensate, reconcile, or keep durable workflow state. A failed
run retains its evidence directory for diagnosis and requires a fresh directory,
disk, OEMDRV image, and Tailscale key for repetition.

Normalized fallback manifests deliberately exclude boot IDs, timestamps, PIDs,
logs, WAL bytes, and deployment selection. They include current account fields,
hashed shadow records, groups, home and key facts, the catalog, workspace trees
and Git state, native Forgejo facts, Tailscale identity and Fedora-owned state
path, SSH host keys, and automatic-update timer state. Raw password hashes and
credentials are never written to evidence.

Before issue #39, final capture requires `sudo sodactl health` to prove the
intentional health-only shell. The #39 capstone changes that assertion to
installed absence; final issue #25 closure occurs only after that post-capstone
run passes on both matching-native architectures.

## Architecture notes

x86-64 uses KVM and OVMF. The existing AArch64 launch implementation is for
matching-native Apple Silicon with HVF and Homebrew QEMU. A matching-native
Linux AArch64 runner must supply its native QEMU/firmware boundary rather than
reusing or inspecting x86-64 artifacts. Temporary host paths are test
infrastructure facts, not Soda product requirements.
