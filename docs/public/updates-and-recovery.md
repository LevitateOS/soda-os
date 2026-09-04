Soda changes its operating-system image with native bootc operations while
Linux and the owning services retain machine-specific state.

## Product contract

### Updates are explicit administrator actions

An administrator chooses an exact signed Soda OCI digest, inspects deployment
state, downloads the image, selects it as the next deployment, and controls the
reboot.

Soda does not discover, download, activate, or reboot for updates automatically.
The automatic bootc update timer is disabled. There is no Soda updater,
deployment database, custom update page, or wrapper command.

### Preserve authoritative state

Update and supported fallback preserve:

- primary and workspace accounts, passwords, groups, and `wheel` membership;
- homes, authorized keys, workspace clones, and project-local data;
- the shared project catalog;
- Forgejo users, repositories, and mutable state;
- Tailscale node identity; and
- SSH host state.

Those facts belong to Linux and the owning services rather than the replaceable
image.

### Select an earlier signed image for fallback

Supported fallback selects the previous signed published OCI image by immutable
digest through the same native deployment mechanism. It does not rebuild the
old release.

Direct `bootc rollback` is unsupported because it may restore historical
`/etc` state instead of preserving current accounts and groups.

An earlier image is not a backup of deleted workspace data. Project and person
removal permanently delete local files and are not undone by image fallback.

Use the exact digest and native bootc sequence published for the release. Read
[Administration](administration.md) before destructive project or person
actions.
