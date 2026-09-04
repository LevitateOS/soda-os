# Soda OS base principles

This document states the product purpose and ownership philosophy in human
terms. The [architectural reset](architecture-reset.md) records the exact
accepted architecture and governing constraints. The linked GitHub issues own
bounded implementation, deletion, and verification work. These layers should
clarify one another rather than become competing specifications.

## What I am trying to build

I want Soda OS to turn a powerful Linux machine into a standardized remote
development appliance.

The machine may be a powerful computer in my home or a machine hosted in the
cloud. My MacBook Air or another lightweight computer acts as the thin client.
On a trusted local network, development tools connect directly over the LAN.
In the cloud, they connect privately through Tailscale. Neither path exposes
Soda services to the public Internet.

The Soda machine runs the expensive work:

- compilation;
- indexing;
- tests;
- development servers;
- databases;
- agents;
- language tooling;
- containers when a project uses them;
- other project processes.

The client remains the interface. The powerful remote machine is the actual
development system.

Soda should provide this without requiring public DNS. Tailscale provides
private cloud reachability, while ordinary LAN routing provides local access.
OpenSSH provides remote development access in both cases.

## The operating-system foundation

Soda OS should be an opinionated Fedora bootc appliance.

The operating-system base is image-based and updated through bootc. Soda selects
and configures the packages needed to provide a useful remote development
machine, including OpenSSH, Tailscale, Cockpit, Forgejo, Git, `mise`, Tea, and
GitHub CLI.

Soda does not replace Fedora, Linux administration, OpenSSH, Tailscale, Cockpit,
Forgejo, Git, Podman, or bootc. Its value is combining them into one coherent,
installable system with a repeatable workflow.

The intended installation architecture is one complete journey with no
separate Soda-owned post-install setup. The current release uses **Soda Setup**
after the installed system boots as a temporary workaround shared by the
machine console and Cockpit. Preserve that usable release journey until it is
replaced; the long-term goal does not authorize hiding Soda Setup inside a
renamed custom installer or inventing another onboarding system.

Linux administrators manage image updates through native bootc operations.
The supported fallback to an earlier image must preserve current Linux account
and workspace state. Soda does not add a runtime update service or parallel
deployment model merely to provide that outcome.

Development-tool installation, versions, and project toolchain configuration
belong to `mise`. Soda ships it for people to invoke and configure directly in
their workspaces. Project configuration is shared through the repository's
native workflow. Soda does not add tool selections, an installation wrapper,
shared tool storage, lifecycle state, or a competing package manager,
downloader, cache, profile system, or version database.

## Human and workspace accounts

Each person has one primary Linux account.

That primary account represents the human for:

- Cockpit login;
- Forgejo login through PAM;
- Linux administrator status;
- project discovery and setup.

Development work runs in derived workspaces rather than in the primary account.

For every project that a person sets up, Soda creates one derived Linux
workspace account for that person-project pair.

For example:

    alice                  primary human account
    alice × website        Alice's website workspace
    alice × api            Alice's API workspace

    bob                    primary human account
    bob × website          Bob's website workspace

Each derived workspace account has its own:

- Linux UID;
- private home;
- Git checkout;
- user-local dependencies;
- caches;
- project data;
- processes.

This isolation exists to prevent ordinary development conflicts. Dependencies,
working files, caches, databases, and processes for one workspace should not
be mixed with another workspace.

This is not a hostile-tenant security model. Soda does not assume that members
of the same development team are trying to attack each other.

Separate Linux accounts do not create separate host port spaces. Projects
choose non-conflicting ports themselves. A project may use rootless Podman or
another ordinary tool when it wants container or network isolation, but Podman
is optional and is not the foundation of Soda's workspace model.

## Projects and collaboration

A Soda project is shared through its canonical Git repository, not through one
shared writable working directory.

Each person receives an independent workspace account and clone. People
collaborate through:

- Git commits and branches;
- pushes and fetches;
- pull requests or merge requests;
- reviews;
- issues and releases where the Git host provides them.

The canonical Git host owns repository access and collaboration.

For a bundled repository, Forgejo owns its repositories, users, permissions,
teams, reviews, issues, and releases.

For an externally hosted repository, the external provider owns those things.
Soda does not copy provider membership, infer permissions, or create an
external-provider management layer.

## The project catalog

Soda maintains one small appliance-wide catalog of projects that are available
for development on that machine.

Every primary human account can discover the catalogued projects.

Through the Soda Projects page, a person can:

- add an existing Git repository URL;
- select **Set up for me**;
- edit a catalog entry's display information and additional metadata;
- remove their own workspace.

Only an administrator removes an entire project from Soda. The catalog does
not have an assumed closed metadata field list; new user-visible project facts
remain explicit product decisions.

Repositories are created through Forgejo or the external authoritative Git host
and then added to Projects with their SSH clone URL. Projects does not create
repositories. A project's ID and canonical URL do not change in place. An
administrator replaces an incorrect URL by removing the project and its local
workspaces, then adding it again; the Git host's repository remains.

Selecting **Set up for me** prepares that person's derived workspace account and
workspace-private outbound Git key. If the authoritative Git host has not
authorized the public key, Projects reports it for the person to register
natively before retrying. Successful setup leaves a complete checkout at
`$HOME/Projects/<repository>` that can be opened directly through SSH. Projects
reports whether the derived Linux workspace account exists, not a synthetic
readiness state, so a retained account is visible while clone completion is
retried. Projects accepts no Forgejo password and registers no workspace key.

Project listing and workspace setup do not depend on Tailscale. The Cockpit
page uses the current browser hostname for SSH guidance instead of making Soda
choose between an approved LAN and Tailnet path.

The catalog records only the minimum Soda-specific association needed to offer
the project through this appliance. It does not become a database of:

- collaborators;
- repository permissions;
- Linux users;
- Git credentials;
- clones;
- branches;
- processes;
- containers;
- ports;
- provisioning jobs;
- retries;
- runtime status.

Repository authorization remains with the canonical Git host.

## Destructive local removal

Removing a project from Soda removes its catalog entry and all Soda-managed
local workspace accounts and data for that project.

This may permanently destroy:

- workspace homes;
- local clones;
- dependencies;
- project-local data;
- uncommitted or unpushed work.

It does not delete the canonical Git repository.

The trusted development team is responsible for coordinating project removal
and pushing anything that must be preserved. Soda does not add approval,
ownership-transfer, archive, rollback, or recovery workflows.

The supported administrator-only human deletion action removes that person's
workspaces first, the Forgejo account second, and the primary Linux account
last. If any step fails, Soda reports exactly what succeeded and remains so an
administrator can retry. There is no rollback. Generic Cockpit or command-line
account deletion remains an ordinary non-cascading Linux action.

## Forgejo

Soda includes Forgejo as its built-in Git forge.

Soda Setup currently creates the initial same-named Linux and Forgejo
administrator accounts, installs that administrator's public SSH key in Linux,
and registers it with Forgejo.

Later primary accounts are created through stock Cockpit or Linux. A person's
first normal Forgejo sign-in creates the corresponding profile through PAM, and
the person manages public keys through Forgejo's native interface. Soda does not
pre-provision later Forgejo accounts or keys. Git uses SSH.

Derived workspace accounts are Linux development identities only. They are not
Forgejo users.

Linux and Forgejo remain independent after account creation:

- Linux owns Linux passwords, UIDs, homes, and `wheel` membership;
- Forgejo owns Forgejo profiles, roles, keys, repositories, and permissions;
- Soda does not synchronize roles or repair differences between them.

Tea and GitHub CLI are available in every workspace. A person authenticates
each client manually and separately in that workspace. Soda copies no Tea
configuration, token, private key, or GitHub credential.

## Cockpit

Soda uses stock Cockpit from its Fedora base.

Cockpit provides:

- host administration;
- Linux account administration;
- services, storage, networking, and logs;
- ordinary Linux authentication and privilege elevation.

Soda adds branding and one focused Projects page for the project catalog and
workspace workflow.

Soda does not provide a second web server, authentication system, session
service, TLS implementation, generic administration dashboard, or replacement
for Cockpit.

## What Soda uniquely owns

Soda owns:

- the branded Fedora bootc image;
- one-ISO installation, with Soda Setup retained as the current temporary
  post-install workaround until one complete installation journey replaces it;
- Tailscale and LAN/OpenSSH access composition;
- package and service configuration;
- the project catalog;
- the primary-human-to-workspace-account convention;
- the focused Cockpit Projects page;
- the narrow synchronous operations needed for catalog mutation, workspace
  setup and removal, and supported cascading human deletion;
- connection guidance for SSH-capable development tools;
- product-level installation and acceptance tests.

These are the parts that turn a collection of existing Linux tools into a
coherent remote development appliance.

## What Soda must not become

Soda must not create:

- a parallel person database;
- a parallel administrator-role system;
- a repository-permission database;
- a Forgejo identity mirror;
- a Git credential vault or broker;
- a custom SSH gateway;
- a container inventory or orchestration system;
- a language-version distribution service;
- a bootc deployment model;
- a generic workflow engine;
- durable provisioning jobs;
- retry queues;
- compensation transactions;
- reconciliation or anti-drift machinery;
- a general-purpose privileged daemon merely to call existing commands.

Existing systems remain authoritative for the facts they naturally own.

When Soda needs to join several existing operations into one product workflow,
it should use configuration, packaging, standard commands, or the narrowest
synchronous adapter that provides the required outcome.

A discrepancy with an upstream system permits Soda to bridge that exact
discrepancy. It does not give Soda ownership of the surrounding subsystem.

## Concise definition

> Soda OS is an opinionated Fedora bootc appliance that turns a powerful
> private machine into a multi-user remote development system. Lightweight
> clients connect over a trusted LAN or through Tailscale and OpenSSH, each
> person-project pair gets
> an independent Linux workspace account, Forgejo or the external canonical
> Git host provides collaboration, and Cockpit provides administration and
> project onboarding.
