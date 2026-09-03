Soda OS is an understandable, human-owned remote development appliance. Its
product model is deliberately small: established systems remain authoritative,
and Soda supplies only the composition and focused transitions needed to turn
them into one repeatable development workflow.

## People and responsibilities

The **infrastructure owner** chooses the cloud account or physical machine,
selects the architecture, controls the Tailnet, protects provisioning secrets,
and decides who may administer the Soda machine.

An **administrator** is a primary Linux user in the `wheel` group. An
administrator adds people, manages the host through stock Cockpit and native
Linux tools, controls operating-system image changes, and uses Soda-aware
removal when a person and their local workspaces must be deleted.

A **developer** uses a primary account to discover projects and a separate
workspace account for each project. Daily development happens directly in
those workspaces over OpenSSH.

One person may be both infrastructure owner and administrator. Administrator
status is always the Linux `wheel` fact; it is not a separate Soda role or a
Forgejo role.

## Primary and workspace accounts

A primary account represents one human. Its username is stable and is used to
associate that person with their derived workspaces. It signs in to Cockpit and
the bundled Forgejo service and owns the person's standard SSH authorization.

A workspace account represents one primary user working on one catalogued
project. It is a real, password-disabled Linux account with its own user ID,
home, files, processes, and complete clone. It is not another human identity,
a Forgejo account, a shared project login, or a Soda service account.

For example:

```text
alice + storefront -> Alice's storefront workspace
bob   + storefront -> Bob's storefront workspace
```

This convention keeps each person's checkout, dependencies, caches, virtual
environments, local databases, and processes independent.

## Who owns each fact

| Fact or responsibility | Authoritative owner |
| --- | --- |
| Accounts, passwords, groups, homes, permissions, and processes | Linux |
| Administrator status | Linux `wheel` membership |
| Private reachability and device policy | Tailscale and the Tailnet owner |
| Remote shells, commands, SCP, and SFTP | OpenSSH |
| Browser sessions and general host administration | Stock Cockpit |
| Repositories, collaborators, branches, reviews, issues, and releases | Forgejo or the external Git host |
| Per-user Forgejo CLI login | Tea and Forgejo |
| Operating-system image selection | Bootc |
| The three-field project list and workspace association | Soda OS |
| Project-specific packages, services, ports, and additional tools | The developer and project |

Soda does not duplicate these upstream facts. Its focused Cockpit experience
invokes narrow operations and then leaves Linux, Git, OpenSSH, Forgejo,
Tailscale, Cockpit, and bootc in charge of the resulting state.

## The trusted Tailnet boundary

A **Tailnet** is the private network created by Tailscale for an organization
or group of devices. Soda's managed OpenSSH, Cockpit, and Forgejo services are
reached through that trusted network. The infrastructure owner decides which
people and devices may join and applies the Tailnet access policy.

Developers connect from authorized laptops or other lightweight clients. Soda
does not add another remote-access gateway or session protocol between the
client and OpenSSH.

## What workspace isolation means

Separate Linux user IDs and homes prevent ordinary development conflicts. One
developer cannot accidentally reuse another developer's checkout, user-local
dependencies, or process ownership simply because they selected the same
project.

Workspaces still share the host kernel and network. They are designed for a
trusted team, not hostile multi-tenant workloads. Projects choose
non-conflicting host ports themselves. Developers may use ordinary rootless
Podman when a project benefits from containers, but Podman is optional and is
not Soda's workspace model.

## Projects and canonical repositories

The Project catalog is a discovery list, not a permission system. Every
primary user can add, edit, set up, and remove catalog entries, so the trusted
team coordinates shared catalog changes.

The canonical repository stays in Forgejo or another Git host. A Soda
workspace is a local clone. Branching, pushing, reviewing, granting access, and
deleting the canonical repository remain native Git-host activities.

Read [Projects and workspaces](projects-and-workspaces.md) for the exact setup
journey and [Data safety and removal](data-safety-and-removal.md) for the
boundary between local deletion and canonical Git data.
