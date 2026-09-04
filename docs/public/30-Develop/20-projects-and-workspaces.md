# Projects and workspaces

Maintain the shared project list and create one isolated Linux workspace for each person-project pair.

## Prerequisites

- Sign in to Cockpit with a primary account.
- Make sure the primary account has at least one valid public key in its
  standard `~/.ssh/authorized_keys` file.
- Make sure the person's public key is registered with the authoritative Git
  host.
- Obtain repository access from Forgejo or the external Git host before
  setting up a private repository.

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

To create a repository in bundled Forgejo:

1. Select **Create Forgejo project**.
2. Enter the requested project information.
3. Submit the operation as your Forgejo identity.
4. Confirm that Forgejo contains the new empty repository and Projects displays
   it.

The canonical repository remains Forgejo-owned. For external hosts, follow that
host's native repository and access controls.

## Set up your workspace

1. Select a project.
2. Choose any initial toolchain conveniences you need. Multiple choices are
   allowed and are a starting point, not a restriction.
3. Select **Set up for me**.
4. Wait for the synchronous result.
5. Record the derived workspace username and connection guidance.

The result is a distinct Linux account with a private home and a complete clone
under `$HOME/Projects/REPOSITORY`. Its UID, files, dependencies, caches, and
processes are separate from every other person's workspace.

Workspace setup copies current public SSH keys once into standard
`authorized_keys`. It does not copy private keys or Tea, GitHub CLI, coding
assistant, or other credentials.

## Add tools later

Use `mise` after connecting. Choose **my workspace** for a personal tool or
**this project** for a shared project tool. See [Connect and
develop](30-connect-and-develop.md#manage-development-tools).

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
  the account, key, or repository access there.
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
