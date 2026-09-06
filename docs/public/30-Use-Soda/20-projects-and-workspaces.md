# Projects and workspaces

Add a repository to the shared catalog and create your own Linux workspace with a complete clone.

## Before setup

Sign in to Cockpit with your primary account and open **Projects**. First add
your [personal public SSH key](../20-Deploy/30-first-connection.md#add-your-personal-ssh-public-key)
to your primary account. You also need access to the authoritative repository
and permission to register your workspace's public key with that Git host.

| Input | What it does |
| --- | --- |
| Personal public SSH key | Lets your client log in; copied once from your primary account to each new workspace |
| Workspace Git public key | Lets this workspace authenticate outbound; register it manually at the Git host |
| Repository SSH address | Identifies the canonical repository; it is neither a key nor a password |

Keep private keys private. Projects accepts no Git-host password or token.

## Add a repository

Create a new repository through [Forgejo](30-forgejo.md#create-a-repository)
or your external Git host, or choose one that already exists. Copy its
credential-free **SSH clone address**, then:

1. Select **Add repository** in Projects.
2. Complete the catalog fields and save.
3. Confirm that the repository appears in the shared list.

| Field | Meaning |
| --- | --- |
| Project name | Human-readable display name; editable later |
| Project ID | Stable identifier chosen at addition; cannot change in place |
| Repository SSH address | Canonical clone URL from the host; cannot change in place |
| Metadata JSON | Optional additional project information as a JSON object; editable, not credentials or tool configuration |

Use the host's actual SSH URL, such as `git@example.test:team/project.git`,
not its browser URL or a URL containing a password. Keep mise tool definitions
in the repository, not catalog metadata. Everyone with a primary account can
view/edit catalog information; repository permissions remain with the Git host.

**Actions → Edit project** changes display information and metadata. If the
canonical URL is wrong, an administrator must remove the project and every local
workspace, then re-add it. **That deletes uncommitted local work.** Coordinate
and preserve data before using [project removal](../50-Operate/40-data-safety-and-removal.md#before-removing-a-project).
The canonical repository remains intact.

## Set up your workspace

1. Select **Set up for me**. Soda checks your personal authorized keys before
   creating an account. An existing account offers **Review setup** instead.
2. If personal keys are missing, add one through stock **Accounts**, then select
   **Check setup**. Checking is read-only: it creates no account or clone.
3. Setup prepares a private workspace account and outbound Git key, then attempts
   the clone. If the host needs that key, register the displayed public key in
   [Forgejo's SSH settings](30-forgejo.md#register-a-workspace-key) or the external
   host's native key interface. Do not upload the private key.
4. Select **Retry setup** after correcting the cause. Other failures can concern
   network access, the repository address, or ownership; read **Technical details**
   instead of assuming every clone failure is authorization.
5. When inspection confirms the workspace, copy its SSH command and repository
   path from **Connection details**.

The result is a real Linux account and private home with a full clone beneath
`$HOME/Projects/REPOSITORY`. Use the returned username, not a guessed naming rule.
[Connect your editor and install tools](../40-Develop/10-connect-and-develop.md)
inside that workspace.

Projects works without Tailscale once Cockpit is reachable through an approved
route. It uses the browser's hostname for SSH guidance, so open Cockpit through
the LAN or Tailnet address your client should use.

## Understand inspection and uncertain results

**Setup not confirmed** means the Linux workspace account exists but the page
has not verified its keys/checkout. A retained account after clone failure is
still visible and removable. **Ready** follows native inspection, not remembered
completion; refreshing the catalog clears earlier observations. Inspection does
not prove Git-host reachability, repository permissions, or a clean working tree.

**Review setup**, **Connection details**, and **Check setup** inspect without
mutation. If setup loses its response, check before retrying: failure to receive
a result does not mean no account or files were created.

Public keys are copied only when a workspace is created. Later primary-key
changes do not update existing workspaces. Tea, GitHub CLI, assistants, and
other credentials are never copied into the new home.

## Remove local work

**Actions → Remove my workspace** deletes only your workspace; administrators
also have **Remove project** for all project workspaces and the shared entry.
Both preserve the canonical repository. Review the affected accounts/folders,
stopped-task warning, and exact confirmation before proceeding.

Follow [Data safety and removal](../50-Operate/40-data-safety-and-removal.md)
for preservation and partial results. A failed deletion can already have removed
files. **Review remaining removal** or **Check current state** does not retry it;
review and confirm again after resolving the reported native problem.
