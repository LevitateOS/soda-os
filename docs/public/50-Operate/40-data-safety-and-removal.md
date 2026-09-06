# Data safety and removal

Review exactly which local accounts and files will disappear before removing a workspace, project, runner, or person.

## What each action removes

| Action | Authority | Deleted locally | Preserved elsewhere |
| --- | --- | --- | --- |
| Remove my workspace | Workspace owner | That workspace account/home, checkout, uncommitted files, dependencies, private data | Canonical repository and shared catalog |
| Remove project | Linux administrator | Catalog entry and every local workspace for that project | Canonical repository and Git-host account data |
| Remove local person | Linux administrator | The person's workspace accounts/homes first, then primary account/home | Canonical repositories and Forgejo account/data |
| Remove runner | Linux administrator | Runner account, native client state, working files and dependencies | Provider registration/history until separately removed |

These are destructive local operations, not an undoable archive. Commits existing
only locally, untracked files, databases, and credentials in deleted homes disappear.

## Before removing a workspace

1. Confirm the project and exact workspace account/home.
2. Use `git status` and inspect local branches; push needed commits to an
   authorized remote. Preserve untracked files, databases, and non-Git data through
   [your backup procedure](30-backups-and-restoration.md).
3. Stop tasks and disconnect sessions that use the workspace. Coordinate with
   others if its development service is shared.
4. In **Projects → Actions → Remove my workspace**, review affected accounts
   and folders, acknowledge task/data warnings, and enter the requested exact text.
5. Confirm removal and check its final result.

A new setup after removal creates a new workspace, not a recovery of local files.
Review/revoke old outbound Git keys through the host when they are no longer needed.

## Before removing a project

An administrator follows the same preservation steps with **every affected
person**, not only their own workspace. Use **Projects → Actions → Remove project**
and review the complete local scope before confirming.

This removes the shared catalog entry and all local workspaces but never deletes
the canonical repository. Deleting the repository itself is a separate Git-host
action with its own scope and recovery policy. Correcting an immutable catalog
URL requires this removal and re-addition; it is not a harmless metadata edit.

## Before removing a person

Use **Projects → People → Remove local person** with Linux administrative access.
The page lists primary humans only and disables your own signed-in account.
The final-administrator protection prevents removing the last primary administrator.

1. Coordinate the person's departure/data handoff. Preserve local work, including
   their primary home and every workspace, before confirming.
2. Review the exact primary username, derived accounts, and homes. Resolve running
   tasks and acknowledge irreversible deletion.
3. Confirm the reviewed scope. Soda removes workspaces first and the primary Linux
   account last; failure to remove a workspace leaves the primary account intact.
4. Check the reported remaining state before any retry.
5. Separately manage Forgejo account ownership, sessions/tokens/keys, external Git
   access, and Tailnet access with their native administrators.

**The Forgejo account and repository data survive Soda person deletion.** Linux
account removal does not guarantee all Git-host access is revoked. Account deletion
inside Forgejo is separate; review repository ownership before using it.

Stock **Cockpit → Accounts** and generic Linux account deletion are non-cascading:
they do not find/remove Soda workspaces for you. Do not delete the primary account
first and expect a later cleanup operation to reconstruct its relationships.

## Review is not retry

A failed operation may already have removed accounts or files. Read and preserve
the partial result: what completed, what failed, and what remains. Inspect the
native Linux/service problem and correct it before acting again.

**Review remaining removal** or **Check current state** only reads current scope.
It neither restores earlier work nor authorizes another mutation. Review the new
scope and confirm again before retrying. If a response is lost, do not infer that
nothing changed.

If a failed account is now absent or has a different UID/home, do not assume its
old files are gone. Have an administrator inspect and resolve those files through
native tools before closing the task and starting a fresh removal. Inspection
neither cleans orphaned homes nor reconstructs a missing primary identity.
Retain important results before leaving the page; there is no durable deletion
history. For unexpected data loss, stop writes, preserve diagnostics, and use
your tested backups.

For runners, follow the separate [local/provider cleanup procedure](../30-Use-Soda/50-ci-runners.md#remove-or-replace-a-runner).
For OS maintenance, [image fallback](../30-Use-Soda/60-updates-and-fallback.md#select-an-earlier-image)
keeps current data; it does not undo any deletion described here.
