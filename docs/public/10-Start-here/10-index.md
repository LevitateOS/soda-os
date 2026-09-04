# Soda OS documentation

Understand the Soda OS product model and the path from installation to a private development workspace.

Soda OS turns one powerful Linux machine into a remote development system for
a trusted team. Developers keep using their laptops, editors, terminals, and
browsers while builds, tests, agents, databases, and project processes run on
the Soda machine.

## Product contract

### From one ISO to development

The owner boots one architecture-matched network ISO and uses stock graphical
Anaconda for storage, networking, and installation. After reboot, the common
interactive first-boot setup creates the administrator, installs the SSH public
key, prepares Forgejo, and selects Tailscale or LAN-only access.

Reusable QCOW2 systems use that same setup through their console. The same
bounded setup operations can be reopened in Cockpit.

### People and workspaces

Each person has one primary Linux account for identity and administration.
Linux `wheel` membership means administrator. Development happens in a separate
workspace account for each person-project pair.

Every workspace has its own Linux UID, private home, full Git clone,
dependencies, processes, and mutable state. Developers connect directly to it
through ordinary OpenSSH.

### Projects and Git

Stock Cockpit supplies host administration and one Soda Projects page. Everyone
can view and edit the shared project list, add an existing repository, create a
native empty Forgejo repository, and set up their own workspace.

Forgejo or the external Git host owns repositories, permissions, and
collaboration. Soda registers each person's public SSH key with Forgejo, and
Git uses SSH.

Tea and GitHub CLI are available in every workspace. Sign in to each manually
inside that workspace; Soda does not copy tokens or configuration.

### Tools and services

`mise` installs and selects development tools for one workspace or for the
project. Shared project tools are stored once and use upstream-native caches.
Soda does not run a toolchain downloader or maintain a version database.

On a trusted LAN, SSH, Cockpit, Forgejo, and development servers are directly
reachable. Cloud deployments use Tailscale and never expose those services to
the public Internet.

### Destructive actions

Each person can remove only their own workspace. An administrator can remove a
whole project, permanently deleting every local workspace and uncommitted file
while leaving the canonical Forgejo repository intact.

Removing a person deletes their workspaces first, their Forgejo account second,
and their primary Linux account last. Failures show exactly what succeeded and
remains. The trusted team coordinates before destructive actions; Soda adds no
approval, archive, transfer, rollback, or recovery workflow.

Read [Installation model](20-installation-model.md),
[Accounts and workspaces](../20-Core-workflow/10-accounts-and-workspaces.md), or
[Projects and Git](../20-Core-workflow/20-projects-and-git.md) to continue.
