Soda OS uses native bootc operations for explicit operating-system updates and
fallback. An administrator selects an exact signed Soda image digest, downloads
it, reviews the resulting deployment state, activates it, and reboots.

An **exact digest** is an immutable image reference ending in
`@sha256:<digest>`. It identifies one precise architecture-specific image,
unlike a moving tag.

## Before changing the image

1. Read the release notes for both the installed and selected releases.
2. Download and verify the signed release record for the machine's
   architecture.
3. Confirm that the record identifies the intended exact image digest.
4. Check the machine and its important services.
5. Back up mutable data that the team cannot recreate.

Use the x86-64 digest on an x86-64 machine and the AArch64 digest on an AArch64
machine.

## Download and activate an update

Inspect the existing deployment:

```sh
sudo bootc status
```

Download the exact selected image without activating it:

```sh
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
```

Inspect the state again, then activate the downloaded deployment:

```sh
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

After reconnecting through the Tailnet, verify:

```sh
sudo bootc status
systemctl --failed
```

Also check Cockpit, Forgejo, Projects, and one representative workspace before
returning the machine to normal use.

## Select an earlier image

Fallback is the same explicit operation using an earlier signed Soda OS digest
for the same architecture:

```sh
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<earlier-digest>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Verify the active deployment and services after the reboot just as you would
for an update.

## State preserved across image selection

Updating or selecting an earlier Soda image preserves machine-specific mutable
state, including:

- primary and workspace Linux accounts;
- passwords, groups, and `wheel` membership;
- home directories, clones, keys, and user-local dependencies;
- the Project catalog;
- Forgejo repositories and mutable service state;
- Tailscale identity; and
- other data stored in the machine's designated mutable locations.

Image selection changes the operating-system deployment. It does not restore a
workspace, account, repository, or local file that was deliberately deleted.

For deletion and backup consequences, read
[Data safety and removal](data-safety-and-removal.md).
