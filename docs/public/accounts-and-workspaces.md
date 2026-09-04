Soda gives each person one Linux identity for using the machine and a separate
Linux environment for every project they work on.

## Product contract

### Primary accounts represent people

A primary account is an ordinary Linux account used for Cockpit, the Projects
page, and Forgejo. Linux owns its password, groups, home, and permissions.
Membership in `wheel` is the only administrator fact.

Stock Cockpit Accounts lists people and changes administrator membership. Soda
does not maintain a separate people or role database. Primary accounts exist
for identity and administration; development happens in workspaces.

Each supported person receives a same-named Forgejo account. Soda registers the
person's public SSH key there. Git uses SSH.

### Workspace accounts contain development

A workspace account represents one person working on one project. If Alice and
Bob both select the same project, each receives a distinct Linux account, UID,
home, clone, dependencies, caches, processes, and project data.

The clone is ready below `$HOME/Projects`. Interactive shells, commands,
automation, SCP, SFTP, editors, and agents run as the workspace's real Linux
user.

Workspace accounts are not people, administrators, shared project logins, or
Forgejo users.

### Keys and client authentication

Before setup, the primary account needs a valid public key in its standard
`~/.ssh/authorized_keys`. Soda copies the current public keys once into the new
workspace's `authorized_keys`. Later changes are not synchronized.

Soda never copies a private key. Tea and GitHub CLI are available in each
workspace, but you sign in to each manually and separately there. Soda copies
no Tea token, Tea configuration, gh configuration, or other credential.

### Tools and isolation

Use `mise` to install tools for **my workspace** or **this project**. Shared
project tools are stored once and reused by that project's workspaces. Upstream
tool managers own their native shared caches. Installed dependencies and other
mutable state remain private to the workspace.

Coding assistants are selected and authenticated separately per workspace.

Workspace separation prevents ordinary file, dependency, cache, and process
conflicts. It is not hostile-tenant isolation. Workspaces share the kernel and
network, so projects choose non-conflicting ports.

### Removal

You may remove only your own workspace. An administrator may remove an entire
project and every local workspace. Removing a person deletes their workspaces,
then their Forgejo account, then their primary Linux account.

These operations permanently delete local data and may destroy uncommitted
work. Forgejo owns the consequences of deleting its user record; Soda does not
transfer or preserve Forgejo-owned data. Read
[Administration](administration.md) before using these actions.
