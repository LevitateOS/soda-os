# Projects and Git

How the project catalog, Forgejo, external Git hosts, and Set up for me divide responsibility.

The Soda Projects page is the shared list of repositories that developers can
turn into private workspaces.

## Product contract

### The catalog is shared discovery

Every primary account can view and edit the shared project list. The catalog
makes projects discoverable; it does not own repository permissions,
collaborators, branches, credentials, clones, processes, ports, or runtime
status.

The catalog has no closed metadata field list. Project information shown by
the page supports the approved workflow without becoming a parallel Git-host
or membership database.

### Add a repository

Use the Projects page to add an existing Git repository or create a native empty
repository in bundled Forgejo. Forgejo or the external Git host owns the
repository from then on.

Soda registers each person's public SSH key with Forgejo. Git uses SSH and
reports native authentication errors directly.

### Set up a workspace

Make sure your primary account has a valid public key in
`~/.ssh/authorized_keys`, then select **Set up for me**.

When setup returns successfully, you have:

- one derived Linux account for you and that project;
- your current public authorized keys copied once; and
- a complete clone owned by that account below `$HOME/Projects`.

Connect directly with the SSH guidance shown by Projects. Authenticate Tea and
GitHub CLI manually inside the workspace when you need them.

Tool conveniences may contain several choices. They are not limited to a fixed
language list. Use `mise` to add tools for only your workspace or once for the
project's shared tool scope.

### Remove your workspace

You may remove only your own workspace. This permanently deletes its account,
home, clone, installed dependencies, caches, processes, project-local data, and
uncommitted work. It does not remove another person's workspace or the project
from the shared list.

### Remove the whole project

Only an administrator can remove an entire project. The operation permanently
deletes every local workspace and then the shared Soda project entry. The
canonical Forgejo repository remains intact.

The trusted team coordinates first. Soda provides no approval, transfer,
archive, preservation, rollback, or recovery workflow. If removal fails, the
page shows exactly what succeeded and remains so an administrator can retry.

Read [Accounts and workspaces](10-accounts-and-workspaces.md) for the identity
model and [Administration](../30-Operate-Soda-OS/10-administration.md) for person deletion.
