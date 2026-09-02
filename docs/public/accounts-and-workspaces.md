Soda OS gives each person one identity for using the machine and a separate
Linux environment for every project they work on. This keeps one project's
files, dependencies, caches, and processes from becoming another project's
problem.

Administrators encounter the distinction when they add or remove people.
Developers encounter it when they sign in to Cockpit and select **Set up for
me** for a project.

## Product contract

### Primary accounts represent people

A **primary account** is an ordinary Linux account that represents one human.
It is used for:

- signing in to stock Cockpit;
- discovering and managing entries on the **Projects** page;
- setting up personal project workspaces;
- signing in to **Forgejo**, the bundled Git hosting and collaboration service,
  through Linux authentication when that path is available; and
- holding Linux administrator status when it belongs to `wheel`.

Linux is the source of truth for the account, password, groups, home, and
administrator status. Soda does not keep a separate people or role database.
Primary usernames are stable identifiers: renaming one while its workspaces
exist is not a supported Soda workflow.

An owner or administrator creates and manages later primary accounts with
ordinary Linux and stock Cockpit account tools. Making someone an
administrator means changing Linux `wheel` membership; it does not change that
person's role in Forgejo.

### Workspace accounts represent development environments

A **workspace account** is a real Linux account derived from one primary
account and one project. If Alice and Bob both select the same project, Soda
creates two independent accounts:

```text
alice + website -> Alice's website workspace
bob   + website -> Bob's website workspace
```

Each workspace owns its own:

- Linux user ID and processes;
- private home directory;
- complete, independently writable Git clone;
- dependencies, caches, and virtual environments; and
- project-local data.

The clone is ready below `$HOME/Projects/<repository>`. Developers enter the
workspace directly through ordinary OpenSSH, so shells, editor sessions,
remote commands, SCP, and SFTP run as the workspace's actual Linux user.

A workspace account is not another human identity. It is not a Forgejo user,
an administrator, a shared project login, or a Soda service account.

### What a developer does

Before the first setup, the primary account must have at least one public key
in its standard `~/.ssh/authorized_keys` file. If no valid key is present,
**Set up for me** stops before it creates an account or clone.

On successful setup, Soda copies the primary account's current authorized
public keys into the new workspace once. Later changes to the primary account's
keys are not synchronized. Add, remove, or rotate workspace keys with ordinary
Linux and OpenSSH tools after creation.

The copied key controls inbound SSH to the Soda workspace. It does not provide
outbound credentials for GitHub, Forgejo, or another Git host. Each developer
chooses ordinary Git authentication for each workspace, such as an SSH agent
or credentials configured privately inside that workspace. Soda retains no Git
credential.

Read [Projects and Git](projects-and-git.md) for the setup journey and
[Connect and develop](connect-and-develop.md) for SSH examples.

### What isolation does and does not mean

Separate Linux homes and user IDs prevent ordinary development conflicts. They
separate working trees, package caches, local databases, files, and process
ownership.

They are not hostile-tenant isolation. Workspace accounts share the host
kernel and network. Projects must choose non-conflicting host ports. A project
may use ordinary rootless Podman when it wants container isolation, but Podman
is optional and Soda does not manage it.

### Removing accounts and workspaces

Removing a project from Soda permanently removes every person's local
workspace for that project. Removing a person through the supported
administrator action permanently removes that person's local workspaces and
then their primary Linux account.

These actions can destroy uncommitted or unpushed work, home-directory files,
dependencies, caches, and project-local data. They do not delete canonical Git
repositories or Forgejo accounts. See [Administration](administration.md) for
the exact distinction between Soda-aware removal and generic Linux account
deletion.

## Current implementation

Primary and workspace identities are represented by ordinary Linux state. A
workspace is a password-disabled Linux account with a generated username,
private home, interactive shell, and a marker tying it to one primary username
and one immutable project ID. Workspace accounts are explicitly excluded from
Cockpit and the intended Forgejo PAM login path.

Code-level verification covers account classification, missing-key failure
before setup changes anything, one-time key installation, complete clone
placement, password-disabled workspace checks, and deletion ordering. A native
x86-64 installation has demonstrated a real derived workspace, its home and
SSH key labels, and a direct SSH command running as that workspace user.

The latest full installed workflow has not yet been repeated on matching-native
AArch64. Final installed acceptance coverage for multiple users and complete
destructive and failure scenarios is also incomplete.

Later-created primary users can use Linux and Cockpit, but they cannot
currently use the intended Forgejo PAM login. That unresolved limitation does
not turn workspace accounts into Forgejo identities; workspace Forgejo login
remains rejected by design.
