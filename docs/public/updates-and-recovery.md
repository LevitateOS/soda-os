Soda OS changes its operating-system image with native **bootc** operations.
Bootc is Fedora's image-based deployment tool: it can inspect, download, and
select a complete OS image while machine-specific data remains outside the
replaceable image layer.

Administrators encounter this workflow when moving to a newer known Soda image
or deliberately selecting an earlier known image. There is no public Soda OS
release or production update channel to use today.

## Product contract

### Updates are explicit administrator actions

A Linux administrator chooses the exact Soda image reference, inspects current
deployment state, downloads the image, makes it the next bootable deployment,
and performs a controlled reboot. An exact reference identifies immutable
image content rather than a moving version label.

Soda does not automatically discover releases, poll an update service,
download images, activate deployments, or reboot the machine. Fedora's
automatic bootc update timer is disabled. There is no Soda update daemon,
deployment database, custom update page, or wrapper command.

Publication is a separate concern. A website may eventually link to release
artifacts, but it is not update authority; the exact published image digest is.

### State that an image change must preserve

Normal image replacement and supported fallback must preserve current
machine-specific state, including:

- primary human accounts and per-person, per-project workspace accounts;
- passwords, groups, and administrator membership;
- homes, workspace clones, and project-local data;
- the Soda project catalog, which lists the repositories offered on the
  machine;
- Forgejo repositories and mutable state;
- the Tailscale node identity; and
- SSH host and authorized-key state.

That data is owned by Linux and the relevant services, not by the replaceable
Soda image.

### Fallback means selecting an earlier known image

The supported recovery path creates a new deployment from an explicitly
selected earlier Soda image using the same native bootc switch mechanism as a
forward image change. The administrator must know the exact earlier image
reference; Soda does not provide a release-discovery or recovery catalog.

Direct `bootc rollback` is unsupported. That command can restore the previous
deployment's historical `/etc`, which may also restore historical account and
group state. Soda's supported fallback must keep the machine's current account
state instead.

An earlier image is not a backup of deleted workspace data. Project removal
and Soda-aware human removal permanently delete local files and are not undone
by selecting another OS image.

See [Administration](administration.md) for destructive account and project
actions and [Installation model](installation-model.md) for fresh installation.

## Current implementation

There is currently no public production release, signed artifact set,
published update digest, or supported production channel. The commands below
describe the current native mechanism, but there is no public Soda image
reference to substitute into them today:

```sh
sudo bootc status
sudo bootc switch --download-only <exact-soda-image-reference>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

The download-only step does not change the running deployment. The later
switch selects the already downloaded image for a deployment, and the
administrator controls when to reboot. Supported fallback repeats this
sequence with an earlier exact Soda reference; it does not use `bootc
rollback`.

The current image keeps native bootc commands, masks the automatic update
timer, and no longer includes Soda's former runtime updater, release-discovery
client, translated deployment state, update API, or update page.

A native x86-64 A-to-B-to-A-to-B exercise has selected an earlier exact image
and then moved forward again while preserving current Linux accounts,
workspaces, catalog data, Forgejo state, Tailscale identity, SSH state, and
other captured mutable state. This is installed-system evidence for x86-64,
not a public recovery guarantee or a published update channel.

Matching-native AArch64 still needs to repeat the current image-selection and
state-preservation exercise. Direct `bootc rollback` remains unsupported on
both architectures.

Production publication is not ready. Local unsigned candidate artifacts and
test records are development evidence only. The remaining temporary
health-only `sodad` and `sodactl health` surface does not participate in update
checks, image selection, activation, fallback, or recovery.
