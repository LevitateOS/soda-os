# Soda Updates and fallback

Check approved releases, download an exact verified image, and explicitly choose when the server restarts.

Soda does not update automatically. OS image changes preserve current accounts
and data; they do not replace [tested backups](../50-Operate/30-backups-and-restoration.md).
Use a primary Linux administrator account, review release notes, preserve
irreplaceable data, and coordinate downtime with the team.

## Update through Cockpit

Open **Soda Updates** and enable Cockpit administrative access.

1. Select **Check for updates**. Soda checks the latest published stable release
   and verifies its signed record, image signature/provenance, architecture,
   version, source revision, and exact digest.
2. Read the release notes and select **Download update**. Downloading neither
   restarts the server nor enables the image for the next restart.
3. When ready to interrupt SSH sessions and development workloads, select
   **Apply and restart…**, review the selected target, and confirm.
4. Reconnect after restart, select **Refresh status**, and check the installed
   version and digest. Verify SSH, Cockpit, Forgejo, and an existing workspace
   before resuming shared work.

The installed image, available release, and pending deployment are separate
facts. Refresh recovers pending state from bootc. A lost connection does not
establish success or failure; reconnect and inspect before retrying.

Do not run other bootc/OSTree deployment commands while applying through Cockpit.
Soda checks the target before and after activation, but native administration
can race those checks. A detected change stops Soda's restart request and may
leave changed pending state. Inspect it before **any** restart.

An absent release, unavailable registry, missing verifier, invalid signature, or
mismatched image is not reported as a successful up-to-date check. Candidate
images and same-version/different-digest installations are not silently replaced.

## Native administration

Administrators can use bootc directly. Obtain the exact architecture-matched
reference from the target release's signed record and
[verify its signature and provenance](../20-Deploy/05-verify-downloads.md#verify-an-oci-image-for-native-updates).
Use `ghcr.io/levitateos/soda-os@sha256:DIGEST`, not a moving tag.

Inspect, then download the verified target:

```sh
sudo bootc status --verbose
sudo bootc switch --download-only EXACT_SIGNED_SODA_IMAGE
sudo bootc status --verbose
```

Replace `EXACT_SIGNED_SODA_IMAGE` with the complete verified reference. Confirm
that the downloaded target is the intended digest. Resolve unexpected pending
state or download failure before activation; do not reboot merely because a
command started.

In the coordinated maintenance window, activate the downloaded image and restart:

```sh
sudo bootc switch --from-downloaded
sudo bootc status --verbose
sudo systemctl reboot
```

Verify activation names the intended image before reboot. After reconnecting:

```sh
bootc status --verbose
systemctl --failed
tailscale status
```

Check the actual booted digest and existing accounts, repository access, and
workspaces. See [bootc's native documentation](https://bootc.dev/bootc/upgrades.html)
for the underlying deployment mechanics.

## Select an earlier image

For Soda's account-preserving fallback, use the same native **switch** sequence
with an earlier exact signed Soda digest. Verify that release's record, matching
architecture, signature, provenance, and release notes before downloading it.
Review its compatibility with the application's current mutable data.

Current Linux accounts, passwords, groups, administrator membership, homes,
Forgejo data, catalog, workspaces, Tailnet identity, and SSH state remain current
while the selected OS image changes. Check them after reboot.

**Do not use direct `bootc rollback` for Soda fallback.** It can select a historical
deployment with older `/etc` state rather than preserving current identities.
Use the documented exact-image switch path instead. Fallback cannot reconstruct
deleted files, repair a damaged disk, or undo application data changes.

## Interrupted or failed operations

- **Verification failure:** do not stage the image; retain the diagnostic.
- **Download failure:** inspect native bootc status and network/registry access
  before trying again. Do not assume a partial download is ready to activate.
- **Activation check or restart failure:** inspect pending state. The next normal
  restart may already use the new image even if the requested reboot failed.
- **Connection loss:** reconnect through the preserved route or use the console;
  determine booted/pending state before another mutation.
- **Unexpected mutable-state change:** stop writes, retain diagnostics, and use
  your tested restoration procedure rather than trying arbitrary rollback commands.
