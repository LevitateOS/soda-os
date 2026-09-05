# Updates and fallback

Select exact signed Soda OCI images with native bootc commands while preserving current accounts and data.

Soda does not update automatically. An administrator decides when to stage an
image, reboot into it, or select an earlier signed image.

## Prerequisites

- Use a primary Linux account in `wheel`.
- Back up irreplaceable mutable data before changing the deployed image.
- Read the release notes for the target version.
- Obtain the architecture-matched exact OCI digest from the signed release
  record or the [latest GitHub
  Release](https://github.com/LevitateOS/soda-os/releases/latest).
- Verify the release record and OCI signature with the release's copy-ready
  Cosign commands before staging.

An exact reference ends in `@sha256:DIGEST`. Do not substitute a moving tag.

Verify the exact image reference before staging it:

```sh
cosign verify \
  --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  EXACT_SIGNED_SODA_IMAGE
cosign verify-attestation \
  --type slsaprovenance \
  --certificate-identity 'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  EXACT_SIGNED_SODA_IMAGE
```

## Update through Cockpit

Open **Soda Updates** and enable Cockpit administrative access.

1. Select **Check for updates**. Soda checks the latest published stable release
   and verifies its signed record, image signature, provenance, architecture,
   version, source revision, and exact digest.
2. Read the release notes, then select **Download update**. Downloading does not
   restart the server or enable the deployment for the next restart.
3. When ready to interrupt SSH sessions and running development workloads,
   select **Apply and restart…** and confirm.
4. After reconnecting, select **Refresh status** and verify the installed version.

Do not run other bootc or deployment commands while applying through Cockpit.
The target is rechecked before and after activation, but bootc does not provide
an atomic expected-digest activation guard. A detected change stops the restart
request and requires inspection of native pending state before any restart.

The page recovers pending deployment state after a refresh. Connection loss is
not proof of either success or failure; reconnect and inspect before retrying.
No published release, unavailable registry, missing verification tool, or invalid
signature is reported as a successful update check. Development candidates and
same-version/different-digest images are not replaced automatically.

Existing installations without this package continue using the native commands
below. See the [engineering notes](../../cockpit-updates.md) for implementation
and current validation evidence.

## Inspect and stage an update with native commands

```sh
sudo bootc status
sudo bootc switch --download-only EXACT_SIGNED_SODA_IMAGE
sudo bootc status --verbose
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Replace `EXACT_SIGNED_SODA_IMAGE` with the complete GHCR digest reference for
this machine's architecture.

After reconnecting, verify the selected image and core services:

```sh
bootc status --verbose
systemctl --failed
tailscale status
```

Then verify Cockpit, Forgejo, SSH, Projects, and one existing workspace before
resuming normal work.

## Select an earlier image

Soda's supported fallback uses the same `switch` sequence with the previous
signed immutable Soda digest:

```sh
sudo bootc switch --download-only PREVIOUS_EXACT_SIGNED_SODA_IMAGE
sudo bootc status --verbose
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Verify the booted digest and mutable state afterward. Current Linux accounts,
passwords, groups, homes, Forgejo data, project catalog, workspaces, Tailscale
identity, and SSH state remain current while the image selection changes.

Do not use direct `bootc rollback` for Soda fallback. Direct rollback can select
a historical deployment with historical `/etc` state, which does not satisfy
Soda's preservation rule. The [bootc upgrade and rollback
documentation](https://bootc.dev/bootc/upgrades.html) explains the upstream
deployment mechanics.

## Expected result

The machine boots the selected exact signed image while current user and
project data remains in place.

## If something fails

- If signature or record verification fails, do not stage the image.
- If download or staging fails, inspect the native `bootc` error and network or
  registry access; do not reboot merely because a command started.
- If `bootc status --verbose` does not show the expected downloaded deployment,
  stop before `switch --from-downloaded`.
- If mutable state differs after reboot, stop using the machine for writes,
  preserve evidence, and restore from backup where necessary.

## Fallback is not backup

Fallback changes the operating-system image. It does not reconstruct deleted
workspace files, Forgejo content, application databases, or damaged disks.
Maintain independent backups and test their restoration.

## Next step

Read [Data safety and removal](30-data-safety-and-removal.md).
