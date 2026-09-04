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
| Remove person | Workspaces, Forgejo account, primary Linux account | Only data unaffected by Forgejo's verified native deletion consequences |
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

1. Ask the person to preserve needed workspace data.
2. Inspect whether their Forgejo account owns repositories, organizations, or
   packages.
3. Resolve that ownership through Forgejo before starting. Forgejo refuses its
   normal non-purge account deletion while any of those objects remain owned by
   the person.
4. Review Forgejo access, keys, tokens, issue work, and pull-request work whose
   ownership or attribution matters to the team.
5. Start the administrator-only person removal from Projects.

The operation deletes in this order:

1. Every validated local workspace belonging to the person.
2. The Forgejo account.
3. The primary Linux account and home.

Forgejo's normal account deletion removes that person's access tokens,
registered SSH keys, repository collaborations, organization memberships, and
other personal access records. Authored issue and comment history remains as
deleted-user history. Owned repositories, organizations, or packages block the
operation rather than being purged. Consult the [Forgejo user
guide](https://forgejo.org/docs/latest/user/) before changing repository-owned
data.

## Partial failure

Soda stops at the first failed deletion and shows exactly which steps succeeded
and which objects remain. It does not hide the partial result or recreate
already deleted data.

Before retrying:

1. Read the full result.
2. Inspect the remaining objects through Projects, Cockpit, Linux, and Forgejo.
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
