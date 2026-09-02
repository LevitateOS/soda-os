Soda OS turns one powerful Linux machine into a private remote development
system for a trusted team. Developers keep using their own laptops, editors,
and terminals, while builds, tests, language tools, databases, agents, and
project processes run on the Soda machine.

You will encounter Soda OS first when an owner installs the machine, then when
an administrator adds people, and finally when each developer creates a
personal workspace for a project.

> **Pre-release documentation:** Soda OS has no public release, downloadable
> installer, or production update channel yet. This handbook explains the
> accepted product behavior and clearly identifies what the current pre-release
> implementation has and has not demonstrated.

## Product contract

### The mental model

Soda OS is an opinionated Fedora system built as a **bootc** image. Bootc is the
native image-based mechanism used to replace the operating-system deployment;
it does not replace the machine's accounts, homes, repositories, or other
machine-specific data.

The machine stays private behind a **Tailnet**: the private network created by
Tailscale for an organization or group of devices. Developers reach the Soda
machine over that network with ordinary OpenSSH rather than through a custom
Soda connection service.

Each person has two kinds of Linux identity:

- A **primary account** represents the human. It is used to sign in to Cockpit,
  discover projects, set up workspaces, and—when supported—sign in to Forgejo.
- A **workspace account** represents one person working on one project. It has
  its own Linux user ID, private home, complete Git clone, dependencies, data,
  caches, and processes.

The **project catalog** is the short, machine-wide list of repositories offered
to developers. It is not a permission database or a record of Git activity.
For each entry, a developer can select **Set up for me** to create their own
workspace.

Soda includes two familiar browser tools:

- **Cockpit** is Fedora's web interface for administering a Linux machine.
  Soda uses stock Cockpit with its own branding and one focused **Projects**
  page.
- **Forgejo** is the bundled Git hosting and collaboration service. Teams may
  also catalog repositories hosted by an external Git provider.

### Who does what

The owner or technical decision-maker chooses the machine, controls its private
network, and decides who may administer it.

An administrator installs Soda OS, manages Linux accounts and administrator
membership, operates OS updates, and uses the supported Soda-aware action when
removing a person and all of that person's local workspaces.

A developer signs in with a primary account to discover projects, then works
through a separate workspace account for each selected project. Developers
collaborate through Git branches, pushes, reviews, and the canonical Git host;
they do not share one writable checkout.

### From installation to development

The intended journey is:

1. Install an image that matches the machine's architecture.
2. Create the first Linux administrator and enroll the machine in its Tailnet.
3. Use stock Cockpit and Linux to add other primary accounts.
4. Add an existing repository in **Projects**, or create an empty repository
   in the bundled Forgejo service.
5. Select **Set up for me** to receive a workspace account and complete clone.
6. Connect directly to that workspace with SSH-capable terminals, editors, and
   development tools.

Read [Installation model](installation-model.md) for the owner journey,
[Accounts and workspaces](accounts-and-workspaces.md) for the identity model,
or [Projects and Git](projects-and-git.md) for the daily setup flow.

### Clear ownership and data boundaries

Linux owns accounts, permissions, homes, and processes. Tailscale owns private
reachability. OpenSSH owns remote sessions. Cockpit owns general machine
administration. Forgejo or the external canonical Git host owns repositories,
access, and collaboration. Bootc owns operating-system deployments.

Soda joins those systems into one workflow. It owns the installable
composition, the project catalog, the workspace convention, and the focused
Projects experience. It does not replace them with a second set of Soda-owned
accounts, permissions, sessions, or deployment state.

Removing a project from Soda permanently deletes every local workspace for
that project, including homes, clones, dependencies, project-local data, and
uncommitted or unpushed work. It does not delete the canonical Git repository.
See [Administration](administration.md) before using any removal action.

## Current implementation

Soda OS is pre-release. The current Fedora 44 bootc image definitions cover
x86-64 and AArch64 as equal product targets, but there is no public release or
production update channel to install from.

One fresh installation on native x86-64 has exercised the complete installed
path: protected installer input, the initial Linux and Forgejo administrator,
Tailnet enrollment, stock Cockpit, Projects, workspace setup, direct SSH, the
installed development toolset, rootless Podman, and the exact installed image.
Native x86-64 image selection to an earlier image and forward again has also
preserved current mutable state.

Matching-native AArch64 construction, installation, and installed-product
evidence for the same current path are still pending. Evidence from x86-64 does
not qualify AArch64.

Later-created primary users cannot currently use the intended Forgejo PAM
login. PAM is the standard Linux authentication path that Forgejo is intended
to use; enabling the current pinned Forgejo process to verify Linux passwords
still requires an unresolved privilege decision. The installer-created first
Forgejo administrator does work.

Final installed coverage for the complete multi-user destructive and failure
scenarios is incomplete. Individual catalog, workspace, credential, and
deletion rules have code-level coverage, but that is not a substitute for the
final installed-system evidence.

Soda ships no runtime daemon, general control CLI, local control socket, or API.
Stock Cockpit and ordinary Linux tools expose native host status.
