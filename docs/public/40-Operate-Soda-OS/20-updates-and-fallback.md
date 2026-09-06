# Updates and fallback

Soda does not update automatically. An administrator explicitly updates through
Cockpit or native bootc and verifies the image that actually booted afterward.
Back up irreplaceable mutable data before changing the deployed image.

## Update through Cockpit

Open **Soda Updates** and enable Cockpit administrative access using a primary
Linux administrator account.

1. Review the tracked native image source, actual booted digest, and any pending
   deployment. **Check for updates** optionally fetches metadata, not image layers.
2. When ready to interrupt SSH sessions and development workloads, select
   **Update and restart**. Bootc resolves the source at operation start, downloads
   and activates as needed, and owns the restart. There is no separate download
   or approval ceremony, and no requirement that the version number increase.
3. Reconnect and refresh status. Verify the **actual booted digest**, not merely
   the version, progress text, or presence of a staged deployment.
4. Check core services and existing workspaces before resuming work.

Check is informational: the source may advance before Update. Cached metadata
and request success are not proof of a boot. An unchanged image with no pending
update needs no extra Soda reboot. Compatible staged updates are handled by
native bootc. Connection loss establishes neither success nor failure; inspect
native status before retrying. The page reports native compatibility, queued
rollback, read-only system, missing-source, and `/usr` overlay diagnostics.

Coordinate native deployment administration while using the page. Soda's lock
serializes its own operations, not other bootc commands. It does not override
administrator source choices or automatically retry/revert ambiguous outcomes.

## Native commands and development tracking

```sh
sudo bootc status --json
sudo bootc upgrade --check
sudo bootc upgrade --apply
```

The selected pre-alpha policy uses architecture-specific rolling development
sources:

- x86-64: `ghcr.io/levitateos/soda-os:dev-x86_64`
- AArch64: `ghcr.io/levitateos/soda-os:dev-aarch64`

Publication trusts authorized repository publishers, uses authenticated HTTPS
publication and content-digest integrity, and retains revision tags. It does not
establish production-release authenticity. Soda Updates requires no signature,
GitHub release, version bump, installer, or sibling result.

`just dev-image <architecture>` prepares and publishes one OCI on matching-native
build hardware. Its source implementation does not prove those tags are already
published. Once an appropriate image is available and the operator
has authorized the source change/restart, a digest-pinned VM needs one explicit
native switch. For example, on the selected native x86-64 VM:

```sh
sudo bootc switch --apply ghcr.io/levitateos/soda-os:dev-x86_64
```

Use `dev-aarch64` on the AArch64 VM. Do not switch again if already tracking the
intended source. The page never chooses a tag on your behalf. An immutable
`@sha256:DIGEST` source remains pinned; publishing a moving tag cannot update it.

After reconnecting:

```sh
sudo bootc status --json
systemctl --failed
tailscale status
```

Verify the source, actual booted digest, current accounts and administrator access,
Cockpit, Forgejo, SSH, Projects, and an existing workspace.

## Select an earlier image

Deliberately selecting an earlier exact Soda OCI digest remains native
administration, not a historical-image browser in Cockpit:

```sh
sudo bootc switch --apply PREVIOUS_EXACT_SODA_IMAGE
```

Replace the placeholder with an architecture-matched `ghcr.io/levitateos/soda-os@sha256:DIGEST`
from the intended previously published image. Verify the booted digest and current
Linux accounts, passwords, groups, homes, Forgejo data, catalog, workspaces,
Tailscale identity, and SSH state afterward. Selecting a digest pins future
upgrades; explicitly switch back to the intended tracking tag when appropriate.

Fallback must preserve current mutable state. This is not a claim that arbitrary
older images or application/database downgrades are compatible. Do not use direct
`bootc rollback`: selecting a historical deployment can restore historical `/etc`
state rather than preserving current account and administrator state. See the
[upstream deployment mechanics](https://bootc.dev/bootc/upgrades.html).

## If something fails

- Inspect the native bootc error and registry/network access. Do not manually
  reboot merely because an update command started.
- Reconnect and read native state before deciding whether to retry.
- If mutable state differs after reboot, stop writes, preserve evidence, and
  restore from backup where necessary.

Fallback changes the operating-system image. It does not restore deleted
workspace files, Forgejo content, databases, or damaged disks. Maintain independent
backups and test restoration.

Read [Data safety and removal](30-data-safety-and-removal.md).
