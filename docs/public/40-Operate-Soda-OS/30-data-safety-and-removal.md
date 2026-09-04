# Data safety and removal

Protect important work and understand exactly what destructive Soda OS actions remove.

Soda OS stores real development work in Linux homes and canonical Git
repositories. Before any destructive action, identify which system owns the
data, push or export what must survive, and verify the backup from another
machine.

## Know what each action changes

| Action | Permanently removes | Preserves |
| --- | --- | --- |
| **Remove project** | The catalogued project's local workspace accounts, homes, clones, dependencies, caches, processes, and project-local data | The canonical Forgejo or external Git repository |
| **Remove person…** | The person's local Soda workspaces and primary Linux account | Forgejo identity, Forgejo repositories, external repositories, and other people's workspaces |
| Change `wheel` membership | Nothing | The account, homes, workspaces, and Git data |
| Update or select an earlier image | Nothing in the supported mutable state | Accounts, homes, workspaces, catalog, Forgejo state, Tailnet identity, and other machine data |

An update is not a backup, and selecting an earlier image is not a recovery
mechanism for deliberately deleted local data.

## Before removing a project

Every primary user may remove a catalog entry, and removal affects every
person's workspace for that Project ID. Coordinate with the whole trusted team.

Before selecting **Remove project**:

1. Identify every person who set up the project.
2. In each workspace, inspect `git status` and the local branches.
3. Push every commit that must survive to the canonical repository.
4. Export databases, generated assets, secrets, build outputs, or other local
   data that does not belong in Git.
5. Stop important project processes.
6. Verify the pushed commits and exported files from another machine.

The confirmation permanently deletes all associated local workspace accounts
and homes, including uncommitted changes and commits that were never pushed.
The catalog entry is removed only after the local workspaces have been removed.

Soda never deletes the canonical repository as part of this action. Repository
deletion remains a separate operation in Forgejo or the external Git host.

## Before removing a person

Only an administrator can use **Remove person…**. Coordinate directly with the
person and the owners of every affected project.

Before confirming:

1. Find the person's Soda workspaces.
2. Push or transfer work that must survive.
3. Export non-Git data from the primary home and workspace homes.
4. Stop important processes owned by those accounts.
5. Verify that another authorized person can reach the preserved data.
6. Re-enter the **Primary username** requested by the confirmation dialog.

Soda removes the person's derived workspaces first and the primary Linux
account last. The person's Forgejo identity and repositories remain in
Forgejo. Manage or remove those separately through Forgejo when the organization
requires it.

Use **Remove person…** for the complete Soda-aware sequence. Ordinary Linux
account deletion changes only the selected Linux account and does not perform
the workspace cleanup described above.

## Protect the canonical repository

A canonical repository preserves only data that has been committed and pushed.
It does not automatically contain:

- uncommitted edits;
- untracked files;
- local-only branches or commits;
- development databases;
- generated artifacts that are excluded from Git; or
- credentials and user-local configuration.

Decide which of those files should be committed, exported, or deliberately
discarded. Never assume that a successful `git push` backs up the entire
workspace home.

## Maintain recoverable backups

Back up the Project catalog, Forgejo state and repositories, important home
data, and any project service data that cannot be regenerated. Keep at least
one copy outside the Soda machine and outside its cloud volume or physical
storage failure domain.

Regularly test a restore into an isolated location. A backup is useful only
when the team can locate it, decrypt it, and recover the expected files.

For routine host responsibilities, return to [Administration](10-administration.md).
