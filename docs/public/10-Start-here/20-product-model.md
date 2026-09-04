# Product model

Understand the people, accounts, services, and ownership boundaries that make Soda OS predictable.

Soda is designed for a trusted team. Administrators have more destructive
capabilities, but they are not treated as hostile users. Team coordination is
the safeguard before deleting shared local work.

## Prerequisites

No system access is required to understand this model. If you already have a
Soda role, find it below before choosing a deployment or daily workflow.

## Roles

### Infrastructure owner

The infrastructure owner chooses and operates the machine, its deployment
location, storage, network, backups, and update timing. This person is normally
also the first administrator.

### Administrator

An administrator is a primary Linux account in the `wheel` group.
Administrators can create primary Linux accounts through stock Cockpit or
native Linux tools, promote primary accounts through Cockpit, remove people,
and remove entire projects with all local workspaces.

### Developer

A developer has one primary account but performs development inside derived
workspace accounts. A developer may create repositories through their native
Git host, add shared project entries, edit their display information and
additional metadata, create their own workspace, and remove only their own
workspace. Project identity and canonical repository URL do not change in
place.

## Account types

| Identity | Purpose | Authority |
|---|---|---|
| Primary Linux account | One person's stable identity and possible administration capability | Linux and `wheel` |
| Workspace Linux account | One person's isolated development identity for one project | Linux |
| Forgejo account | Repository identity, access, SSH keys, issues, and collaboration | Forgejo |
| Tea or GitHub CLI login | Command-line session for one Git host in one workspace | Tea or GitHub CLI in that workspace |

Primary accounts are not development environments. A workspace account has
its own UID, private home, complete Git clone, installed dependencies,
processes, caches, and mutable files.

## Who owns each fact

| Responsibility | Authoritative owner |
|---|---|
| Accounts, passwords, homes, groups, and processes | Linux |
| Administrator capability | Linux `wheel` membership |
| Host administration | Stock Cockpit and native Linux tools |
| Repositories, collaborators, issues, pull requests, and releases | Forgejo or the external Git host |
| SSH access | OpenSSH and standard `authorized_keys` files |
| Private network identity | Tailscale |
| OS images, update selection, and deployment state | bootc |
| Development-tool versions and installation | `mise` |
| Shared project discovery and workspace lifecycle | Cockpit's Soda **Projects** page |

Soda does not copy these facts into a separate identity, repository,
credential, update, or workflow database.

## Projects and workspaces

The **Projects** page contains the shared information the team needs to find
and set up projects. It is not a repository permission system. Everyone may
view and edit the shared list; the authoritative Git host still decides who
may read or write a repository.

Selecting **Set up for me** creates a separate Linux account and complete clone
for that person-project pair. The person's current public SSH keys are copied
once to the workspace. Private keys and command-line credentials are never
copied. The workspace keeps its outbound Git private key locally. If the Git
host does not yet know its public key, setup reports the key and retains the
workspace so the person can register it through the host's native interface
and retry.

The Projects page reports whether the derived Linux account exists. That fact
remains true after an authorization failure retains the account and key, even
though setup must still be retried to complete the clone. Project listing and
setup do not require Tailscale enrollment. SSH guidance uses the hostname
through which the person opened Cockpit.

## Access model

On a trusted LAN, OpenSSH, Cockpit, Forgejo, and project-selected development
ports are available directly over the LAN and may also be reached through
Tailscale. In a cloud deployment, these services are reached only through
Tailscale and are not exposed to the public Internet.

## Destructive authority

- A developer can remove only their own workspace.
- An administrator can remove a project, which permanently deletes its shared
  Soda entry and every local workspace. The canonical Git repository remains.
- An administrator can remove a person, deleting their workspaces first,
  Forgejo account second, and primary Linux account last.

Soda stops on partial failure and reports what succeeded and what remains. It
does not silently roll back completed deletions.

## Expected result

You can identify the native owner of an account, repository, credential,
service, tool, project entry, workspace, or operating-system image before
changing it.

## If a responsibility is unclear

Start with the authoritative owner in the table above. Use Soda's Projects
interface only for shared project discovery and workspace lifecycle; use the
native system for every other responsibility.

## Next step

Choose [Deploy to a cloud](../20-Deploy/10-deploy-to-cloud.md) or
[Install on premises](../20-Deploy/20-install-on-premises.md).
