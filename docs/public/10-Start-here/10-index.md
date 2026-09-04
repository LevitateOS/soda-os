# Soda OS documentation

Understand Soda OS and follow the path from a new machine to a ready development workspace.

Soda OS turns one powerful cloud or on-premises machine into a private remote
development system for a trusted team. Developers keep using their own laptops,
editors, terminals, and coding agents while builds, tests, language tools,
databases, and project services run on the Soda machine.

The normal journey begins when an infrastructure owner deploys Soda OS. An
administrator then adds the people who will use it. Each developer selects a
project, creates a personal workspace, and connects directly to that workspace
with ordinary SSH tools.

## Choose where to deploy

Soda OS is cloud-first. For a cloud virtual machine, import the reusable QCOW2
disk that matches the machine's architecture and provide protected first-boot
input through NoCloud or ConfigDrive. Start with
[Deploy to a cloud](../20-Deploy/10-deploy-to-cloud.md).

For a physical machine or locally managed virtual machine, boot the matching
network ISO, attach protected OEMDRV input, and complete Fedora's graphical
Anaconda installer. Follow [Install on premises](../20-Deploy/20-install-on-premises.md).

AArch64 and x86-64 are equal Soda OS architectures. Always use the release
assets for the destination machine's architecture; the resulting product and
user workflow are the same.

Both deployment paths create:

- the first primary Linux account with administrator access through `wheel`;
- the account's SSH public key in standard `authorized_keys`;
- a same-named Forgejo site administrator and private Tea login;
- one protected attempt to join the owner's Tailnet; and
- the Soda Projects experience inside stock Cockpit.

After deployment, [Make the first connection](../20-Deploy/30-first-connection.md) from a
device already authorized on the Tailnet.

## Understand the people and workspace model

A **primary account** is the ordinary Linux identity for one human. It is used
to sign in to Cockpit, discover projects, and manage personal workspace setup.
Linux owns its password, groups, home, and administrator status.

A **workspace account** is a separate Linux identity for one person working on
one project. It has its own home, complete Git clone, dependencies, data, and
processes. Alice and Bob can work on the same repository without sharing a
writable checkout or user-local tools.

Read [Product model](20-product-model.md) for the ownership and trust boundaries,
then use [Add people and manage access](../30-Develop/10-people-and-access.md) to onboard the
team.

## Go from a repository to a ready workspace

The **Project catalog** is the machine-wide list of repositories offered to the
team. It stores only a stable project ID, a display name, and a credential-free
canonical Git URL. Forgejo or an external Git host remains authoritative for
the repository, access, branches, reviews, issues, and releases.

A developer opens **Projects** in Cockpit and selects **Set up for me**. Soda
creates the derived workspace account, copies that person's SSH authorization
and private Tea configuration once, and leaves a complete clone under the
workspace's `$HOME/Projects/<repository>` directory.

Continue with [Projects and workspaces](../30-Develop/20-projects-and-workspaces.md), then
[Connect and develop](../30-Develop/30-connect-and-develop.md).

## Operate the machine with familiar systems

Soda OS composes established Linux services rather than replacing them:

- Tailscale owns private network membership and policy.
- OpenSSH owns remote shells, commands, SCP, and SFTP.
- Stock Cockpit owns browser authentication and general host administration.
- Linux owns accounts, permissions, homes, groups, and processes.
- Forgejo or the selected external Git host owns repositories and collaboration.
- Bootc owns operating-system image selection and deployment.

Soda owns the installable composition, focused Cockpit pages, minimal project
catalog, workspace convention, and the narrow operations that join those parts
into a coherent product.

Use [Administration](../40-Operate-Soda-OS/10-administration.md) for routine operation and
[Updates and fallback](../40-Operate-Soda-OS/20-updates-and-fallback.md) for explicit image changes.

## Protect work before removing anything

Soda workspaces hold real files and processes. **Remove project** permanently
deletes every local workspace for that catalog entry, including uncommitted or
unpushed work, while preserving the canonical repository. **Remove person…**
permanently deletes that person's local workspaces and primary Linux account;
Forgejo repositories and the Forgejo identity remain.

Before either action, push important commits and separately export local data
that does not belong in Git. Read [Data safety and removal](../40-Operate-Soda-OS/30-data-safety-and-removal.md)
before removing a project or person.
