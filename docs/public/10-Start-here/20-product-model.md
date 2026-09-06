# Product model

Understand what the machine shares, what belongs to a project, and what stays private to each developer.

## Three layers

| Layer | What lives there |
| --- | --- |
| Shared foundation | Machine, OS image, system tools, compute, storage, private access, project catalog |
| Shared project | Canonical repository reference and any development services the team chooses to run |
| Developer-project workspace | Linux account, UID, home, full clone, dependencies, caches, files, data, processes |

A project is not a shared writable checkout. Teammates collaborate through
ordinary branches, commits, pushes, and reviews at the authoritative Git host.
Each person chooses which workspaces to create rather than receiving one for
every project automatically.

Separate accounts prevent accidental mixing of dependencies, files, and process
ownership. Soda is for trusted people, not hostile multitenancy. Workspaces share
host ports; choose non-conflicting ports for project services. User-operated
containers are optional, not the workspace foundation.

## Accounts and roles

- Your **primary Linux account** is your stable identity for Cockpit, project
  discovery, and administration. Develop in a workspace, not this home.
- A **workspace account** belongs to you for one project. Connect directly as
  that username with OpenSSH; its files and processes use its real Linux UID.
- Your **Forgejo profile** is created by ordinary first login with your Linux
  username and password through PAM. Workspace accounts do not get Forgejo profiles.
- A **Linux administrator** has `wheel` membership. This does not make them a
  Forgejo administrator; [Forgejo administration](../30-Use-Soda/30-forgejo.md#create-a-forgejo-administrator)
  is granted explicitly and independently.

All primary humans can view and edit the shared catalog and create their own
workspaces. The Git host still decides who may read or write each repository.
Catalog visibility does not grant repository access.

## Native tools, clear responsibilities

| Task | Use |
| --- | --- |
| Host services, logs, storage, networking, people | [Cockpit](../30-Use-Soda/10-cockpit.md) and native Linux tools |
| Repository creation, permissions, collaboration | [Forgejo](../30-Use-Soda/30-forgejo.md) or your external Git host |
| Project catalog and workspace lifecycle | [Projects](../30-Use-Soda/20-projects-and-workspaces.md) |
| Remote sessions and file transfers | OpenSSH |
| Private cloud reachability | [Tailscale](../30-Use-Soda/40-tailscale.md) |
| Development tools and repository configuration | [mise](../40-Develop/10-connect-and-develop.md#manage-development-tools) |
| Local provider CI capacity | [Runners](../30-Use-Soda/50-ci-runners.md) |
| Explicit OS update and restart | [Soda Updates and bootc](../30-Use-Soda/60-updates-and-fallback.md) |

## Access and credentials

Use the trusted LAN for local installations, or Tailscale for cloud access.
Enrolling a LAN server does not disable its LAN route. Firewall allowances for
Forgejo and development services remain administrator choices.

Your client keeps your personal private SSH key. Workspace setup copies only
current public authorized keys, once. Each workspace keeps its own outbound
Git private key; register its public half at the Git host. Tea, GitHub CLI, and
assistants are authenticated separately in each workspace.

## Ownership includes deletion

You can remove your own workspace. Administrators can remove a whole project
and every local workspace, or remove a person's workspaces and primary account.
These actions destroy local work, including uncommitted files, but preserve the
canonical repository. Forgejo account deletion is separate.

Coordinate before removal and maintain [tested backups](../50-Operate/30-backups-and-restoration.md).
[Data safety and removal](../50-Operate/40-data-safety-and-removal.md) explains
scope, confirmation, and partial results. Image fallback is not file recovery.
