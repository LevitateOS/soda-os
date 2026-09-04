# Administration

How stock Cockpit, Linux administrator status, and Soda's focused Projects page fit together.

Soda keeps general machine administration in stock Cockpit and ordinary Linux
tools. The Projects page contains only Soda-specific project, workspace, and
person lifecycle actions.

## Product contract

### Accounts and administrator status

Stock Cockpit owns browser authentication, sessions, TLS, overview, metrics,
services, logs, accounts, terminal, storage, and networking.

Every primary Linux account can sign in. Workspace accounts cannot. Linux
`wheel` membership is the only administrator fact; use Cockpit Accounts to list
people and promote or demote administrators.

All team members are trusted. Administrator is a capability boundary, not a
hostile-user model.

### Add a person

The administrator-only Soda action creates one ordinary primary Linux account,
one matching Forgejo account, and registers the person's public SSH key with
Forgejo. The new account is not an administrator unless later added to `wheel`
through Cockpit or Linux.

Soda does not create Tea or gh credentials. The person authenticates those
clients manually in each workspace.

### Remove a workspace or project

Each person can remove only their own workspace.

Only an administrator can remove an entire project. This permanently deletes
the shared Soda project entry and every local workspace, including homes,
clones, dependencies, project data, and uncommitted work. The canonical Forgejo
repository remains intact.

The trusted team coordinates before removal. There is no approval, transfer,
archive, preservation, rollback, or recovery workflow.

### Remove a person

The administrator-only Soda action performs this order:

1. Delete the person's workspaces.
2. Delete the Forgejo account through Forgejo's native operation.
3. Delete the primary Linux account last.

If a step fails, Soda stops and shows exactly what succeeded and remains. An
administrator may retry explicitly. No result is hidden and no rollback occurs.
Forgejo determines the consequences for data it owns. Review that account's
repositories and contributions before confirming; Soda does not transfer,
archive, or preserve them.

Do not confuse this with generic Cockpit account deletion or `userdel`; those
native Linux actions delete only the selected Linux account and do not cascade.

### Operate services and updates

Use Cockpit, `systemctl`, and `journalctl` for host inspection. Use native bootc
commands for explicit image update and fallback. Soda has no runtime daemon,
health API, update service, or general administration CLI.

Read [Projects and Git](../20-Core-workflow/20-projects-and-git.md) for workspace behavior and
[Updates and recovery](20-updates-and-recovery.md) for image lifecycle.
