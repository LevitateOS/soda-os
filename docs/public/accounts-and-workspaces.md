Soda OS uses ordinary Linux accounts to separate a person's identity from the
independent environments where their projects run.

## Product contract

Each person has one primary Linux account. That account represents the human
for Cockpit login, Forgejo login, project discovery, workspace setup, and Linux
administrator status. Membership in `wheel` is the administrator fact; Soda
does not keep a separate person or role database.

For every project a person sets up, Soda creates one derived Linux workspace
account for that person-project pair. Each workspace has its own:

- Linux UID and process ownership;
- private home;
- complete, independently writable Git clone;
- user-local dependencies and caches; and
- project-local data.

The complete checkout lives below `$HOME/Projects/<repository>` in the
workspace account. Two people setting up the same project receive different
accounts, homes, clones, credentials, caches, and processes. They collaborate
through Git rather than a shared writable worktree.

This separation prevents ordinary development conflicts; it is not a hostile
tenant security boundary. Workspace accounts share the host network and kernel.
Projects choose non-conflicting ports and may use ordinary rootless Podman when
they need additional isolation.

## Current implementation

Primary and workspace identities are represented through Linux-native account
state. Workspace accounts are password-disabled, belong to the
`soda-workspaces` group, use a private primary group and home, and carry a
validated marker connecting the primary username to the immutable project ID.
Their system username is generated deterministically from that association.

Before creating anything, **Set up for me** requires at least one valid public
key in the primary account's standard `~/.ssh/authorized_keys`. The current
keys are copied once into the new workspace. Later changes are not synchronized.
Inbound workspace SSH keys and outbound Git-host credentials remain separate;
Soda does not create or retain a private Git credential for the workspace.

Focused tests cover account classification, missing-key failure before
mutation, one-time key installation, complete clone publication, and cleanup
ordering. Native x86-64 installation evidence includes a working derived
workspace and direct attributed SSH execution. Final installed multi-user and
destructive-failure automation, plus matching-native AArch64 repetition, remain
incomplete.

Next, read [projects and Git](projects-and-git.md) for how a catalogued project
becomes one of these workspaces.
