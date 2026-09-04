# Projects and workspaces

Maintain the shared project list and create one isolated Linux workspace for each person-project pair.

## Prerequisites

- Sign in to Cockpit with a primary account.
- Make sure the primary account has at least one valid public key in its
  standard `~/.ssh/authorized_keys` file.
- Obtain repository access from Forgejo or the external Git host before
  setting up a private repository, and be able to add an SSH public key to your
  account there.

## Use the shared project list

Open **Projects** in Cockpit. Every primary user may view and edit the shared
project information shown there. The list helps people find projects and create
workspaces; it does not replace repository permissions or collaboration rules
owned by the Git host.

To add an existing repository:

1. Select **Add existing project**.
2. Enter the project information requested by the interface.
3. Use the credential-free SSH clone URL from the authoritative Git host.
4. Save the project and confirm it appears in the shared list.

The project ID and canonical URL are fixed when the project is added. Editing a
project changes its display information or additional metadata, not its URL.

To create a repository in bundled Forgejo or an external Git host:

1. Sign in to the authoritative Git host and create the repository through its
   native interface.
2. Copy its credential-free SSH clone URL.
3. Return to **Projects** and select **Add existing project**.
4. Enter the project information and clone URL, then save it.
5. Confirm that the native Git host contains the repository and **Projects**
   displays it.

The canonical repository remains owned by its Git host. Repository creation,
access, and collaboration use that host's native interfaces.

If the canonical URL is wrong or must be replaced, coordinate with an
administrator. The supported replacement is to remove the project from Soda,
which permanently deletes every local workspace and its uncommitted data, and
then add the project again with the replacement SSH URL. The Git host's
repository is not deleted.

## Set up your workspace

1. Select a project.
2. Select **Set up for me** and wait for the synchronous result.
3. If setup reports that it retained the workspace and shows its outbound Git
   public key, add that key to your account through the Git host's native user
   interface.
4. Retry **Set up for me** to complete the clone.
5. Record the derived workspace username and connection guidance from the
   successful result.

Projects listing and setup work without Tailscale once Cockpit is reachable on
an approved network path. The page builds the displayed SSH command from the
hostname used to open Cockpit, so open it with the LAN hostname or Tailnet name
that the client should use.

The result is a distinct Linux account with a private home and a complete clone
under `$HOME/Projects/REPOSITORY`. Its UID, files, dependencies, caches, and
processes are separate from every other person's workspace.

Workspace setup copies current public SSH keys once into standard
`authorized_keys` for inbound login. The outbound Git private key stays only in
the workspace, while its public half is registered manually with the Git host.
Setup does not copy private keys or Tea, GitHub CLI, coding assistant, or other
credentials from the primary account.

**Workspace account exists** means that the derived Linux account exists. A
failed clone may retain that account and its outbound key, so this label does
not claim that the repository clone is complete. Register the reported key and
retry **Set up for me**; the existing workspace can also be removed explicitly.

## Add tools later

Use `mise` directly after connecting to the workspace. Personal choices belong
to that user; shared choices belong in repository configuration. See [Connect
and develop](30-connect-and-develop.md#manage-development-tools).

## Remove your workspace

Select **Remove my workspace** only after committing, pushing, or otherwise
copying anything you need. This permanently deletes your local workspace and
all uncommitted files in it. It preserves the shared project entry, every other
person's workspace, and the canonical repository.

## Remove an entire project

Only an administrator can remove a project. The operation permanently deletes
the shared Soda entry and every local workspace for every person, including
uncommitted files. The canonical Forgejo or external repository remains
intact.

Coordinate with the trusted team before using this action. Read [Data safety
and removal](../40-Operate-Soda-OS/30-data-safety-and-removal.md) first.

## If something fails

- A Git authorization error is owned by the authoritative Git host; correct
  the account, reported workspace key, or repository access there, then retry
  setup.
- A missing-key error occurs before workspace mutation; add a valid public key
  to the primary account and retry.
- An ambiguous existing account, directory, or ownership state stops setup for
  administrator inspection.
- A removal failure stops immediately and reports what remains. Retry only
  after inspecting the reported state.

## Expected result

The project is visible to the team, while each developer receives a private,
directly addressable Linux workspace containing a full repository clone.

## Next step

Read [Connect and develop](30-connect-and-develop.md).
