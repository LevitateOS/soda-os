# Data safety and removal

Understand exactly what each destructive Soda action removes, what it preserves, and how partial failure is reported.

All team members are trusted, and administrators have broader destructive
capability. Coordinate before deleting shared local work. Soda does not add an
approval, archive, transfer, preservation, or recovery workflow.

## Prerequisites

- Know whether you are removing one workspace, a whole project, or a person.
- Use a primary account in `wheel` for project-wide or person removal.
- Confirm that everyone affected has preserved the data they need.
- Keep a tested backup independent of the Soda machine.

## Action matrix

| Action | Deletes | Preserves |
|---|---|---|
| Remove my workspace | That user's local workspace and uncommitted data | Project entry, other workspaces, canonical repository |
| Remove project | Shared entry and every local workspace | Canonical Forgejo or external repository |
| Remove person | Local workspaces and primary Linux account | Forgejo account and data |
| Image update or fallback | No supported mutable state | Accounts, homes, Forgejo, catalog, workspaces, Tailscale, SSH state |

Deletion is permanent for local files. A canonical repository cannot recover a
change that was never committed and pushed.

## Before removing your workspace

1. Stop your workspace processes.
2. Run `git status` in every working tree.
3. Commit and push wanted changes, or copy them to a protected destination.
4. Export untracked application data that is not represented in Git.
5. Confirm that you selected **Remove my workspace**, not the administrator-only
   whole-project action.

Only the current user may remove their own workspace. The shared project and
other people's workspaces remain.

## Before removing a project

Only an administrator may remove an entire project.

1. Tell every project user that all local workspaces will be deleted.
2. Ask each person to stop processes and inspect uncommitted and untracked data.
3. Confirm that required changes are pushed to the canonical repository or
   copied elsewhere.
4. Confirm the canonical repository in Forgejo or the external Git host.
5. Review the Projects warning and select **Remove project** only when the team
   is ready.

Soda deletes each local workspace and then removes the shared project entry.
The canonical Forgejo or external repository remains intact.

## Before removing a person

1. Ask the person to preserve needed local data and stop workspace processes.
2. Review the Linux account and local workspaces selected in Projects.
3. Start the administrator-only person removal.

Soda's administrator-only person removal deletes local workspaces first and the
primary Linux account last. It neither inspects nor deletes a same-named
Forgejo account. Forgejo availability, ownership, and deletion restrictions do
not block Linux-person deletion. Delete Forgejo accounts explicitly inside
Forgejo. Linux preflight checks, partial-failure reporting, and generic
non-cascading Cockpit/Linux deletion remain unchanged.

## Partial failure

Soda stops at the first failed deletion and shows exactly which steps succeeded
and which objects remain. It does not hide the partial result or recreate
already deleted data.

Before retrying:

1. Read the full result.
2. Inspect the remaining objects through Projects, Cockpit, and Linux.
3. Correct only the reported cause.
4. Repeat the same supported action. Completed steps are recognized; ambiguous
   or conflicting state stops for explicit inspection.

## Expected result

The selected local responsibility is gone, preserved repositories remain where
specified, and any partial result is visible and actionable.

## If data was removed unexpectedly

Stop writes that could overwrite recoverable storage. Preserve logs and exact
operation output, then use the team's tested backups. Soda has no hidden
recovery database or rollback workflow for deleted user files.

## Next step

Return to [Administration](10-administration.md) and verify the remaining
accounts, workspaces, repositories, and services.
