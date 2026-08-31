# Soda OS architectural reset

**Status:** Product direction accepted; implementation decisions remain under
review in the linked issues.

**Recorded:** 2026-08-31

## Summary

Soda OS needs an architectural reset.

The product is an opinionated Fedora bootc appliance for private remote
development. A lightweight client connects over Tailscale and SSH to a more
powerful Soda machine. Standard clients such as Codex, Claude, Zed, VS Code,
and OpenSSH run their remote workloads there. Forgejo provides Git
collaboration. Cockpit provides system administration.

Soda's value is the installable, repeatable composition of those existing
systems and the smallest workflow that joins them. Soda is not intended to be
a parallel implementation of Linux identity, OpenSSH sessions, Git
collaboration, Cockpit administration, language toolchain distribution, or
bootc deployment management.

The current implementation crossed that boundary. Small product discrepancies
were treated as reasons for Soda to own surrounding subsystems. Custom helpers
acquired custom state; custom state required synchronization; synchronization
required provisioning, rollback, retry, reconciliation, RPC, UI mapping, and
tests. Those layers then made the accidental architecture appear permanent.

The reset changes the burden of proof: an existing system remains authoritative
unless a concrete Soda product fact cannot be represented there. A discrepancy
justifies bridging that discrepancy. It does not transfer ownership of the
surrounding subsystem to Soda.

## Original product intent

Soda OS standardizes a way of working:

1. Install a Fedora bootc development appliance on a powerful remote machine.
2. Join it to a private Tailnet instead of exposing it to the public Internet.
3. Administer the machine with normal Linux facilities and Cockpit.
4. Connect from a lightweight client with an SSH-capable development tool.
5. Create a project from a new local Forgejo repository or an existing Git
   repository.
6. Give each collaborator an attributed, conflict-free workspace.
7. Collaborate through Git branches, pushes, and Forgejo review.

The MacBook, laptop, or other client holds the human's local private material
and user interface. Compilation, indexing, tests, agents, and development
processes run on the Soda machine.

Soda should make this workflow understandable and repeatable. It should not
replace the tools that already implement it.

## How the architecture got out of hand

The repository exhibits a repeatable snowball pattern. This describes the
architectural mechanism visible in the code; it does not require assigning a
single chronology or intent to every change.

1. An existing Linux or upstream behavior did not exactly match one Soda
   workflow.
2. Soda added a custom helper for the discrepancy.
3. The helper needed metadata and Soda persisted a parallel representation.
4. The parallel representation was declared authoritative.
5. Linux, Git, Forgejo, or bootc could now disagree with Soda state.
6. The disagreement required preflights, compensation, rollback, retry, and
   startup reconciliation.
7. The browser needed access to the new state, producing a daemon, protobuf and
   gRPC contracts, client adapters, and presentation models.
8. Tests preserved the internal machinery as if it were an independent product
   requirement.

This is visible across the current implementation:

- Soda stores people and application roles while Linux already owns accounts,
  UIDs, passwords, groups, and administrator status.
- Soda stores projects, memberships, and worktrees while Unix groups, the
  filesystem, Git, and Forgejo already persist most of those facts.
- `soda-cockpit` is a standalone web application that owns TLS, PAM bridging,
  sessions, navigation, and host pages instead of being a Cockpit package.
- Soda samples generic Linux host telemetry already available through Cockpit,
  systemd, D-Bus, and ordinary Linux interfaces.
- Soda implements release discovery, downloading, checksums, archive
  extraction, and installation for several language ecosystems.
- Soda translates and validates a parallel representation of bootc update
  state.
- Soda projects its people, keys, repositories, and collaborators into Forgejo
  and stores remote mapping records, creating reconciliation work.

The problem is not that Go invokes standard commands. A small, testable adapter
can be appropriate. The problem is that adapters became alternative owners of
the state and behavior around those commands.

## Reset principles

### Existing systems remain authoritative

Use the system that already owns each fact:

| Responsibility | Authoritative owner |
| --- | --- |
| Human identity and process attribution | Linux UID and account |
| Administrator status | Linux administrator membership, currently `wheel` |
| Project filesystem access | Unix groups and filesystem permissions |
| Remote execution | OpenSSH |
| Repository state and history | Git |
| Git users, teams, repositories, and review | Forgejo |
| Host administration and generic telemetry | Cockpit and system services |
| OS deployment state and activation | bootc |
| Language installation | A reviewed existing toolchain boundary |
| Appliance image and product conventions | Soda OS |

Soda may coordinate a workflow across owners. It should not copy their state
merely to make queries or UI rendering convenient.

### Custom code stays at the edge

For every proposed Soda component, answer:

1. What exact user outcome requires it?
2. Which existing system already owns most of that outcome?
3. What precise gap remains?
4. Can configuration, packaging, or a short adapter bridge the gap?
5. Does Soda genuinely own a durable fact that must be persisted?
6. What code and state disappear when the existing owner remains authoritative?

If the retained component cannot answer those questions, it should be deleted
rather than reorganized.

### Tests prove behavior; they do not create product authority

Current tests are evidence of the current implementation. They are not a reason
to preserve obsolete schemas, RPCs, reconciliation, or internal APIs after the
ownership decision changes.

### Delete vertically

The reset is not a package-shuffling exercise. Implementation should remove one
unnecessary ownership slice at a time while leaving an understandable working
product after each step. Do not replace the current control plane with another
generic framework, compatibility layer, or workflow engine.

## Target multi-user model

The reset separates four concerns that the current implementation partially
combines: human identity, project authorization, workspace conflict isolation,
and credential isolation.

### One Linux user per human

Alice works as Linux user `alice`; Bob works as `bob`. Their UID is the process,
file-ownership, audit, and SSH attribution boundary. They do not normally log in
as a shared project account.

Linux administrator status is also authoritative. A Linux administrator is a
Soda administrator without registration, import, or a parallel Soda role.

### One Unix group per project

A project group grants Alice and Bob access to the project. Forgejo's matching
team or repository membership grants Git collaboration access. The review must
decide whether a project service account remains necessary for any concrete
automation; it is not required merely to represent the project.

### One person-owned checkout per person and project

Alice and Bob share a repository and its history, but they should not write to
the same working directory. A target layout may resemble:

```text
/srv/soda/projects/example/
├── alice/
│   └── repository/
└── bob/
    └── repository/
```

Each checkout is owned by the corresponding human. Project collaborators may
receive read access to one another's project files for direct inspection, but
they should not receive write access to another person's checkout by default.
They exchange changes through Git and review them through Forgejo.

Git worktrees remain useful within one person's clone when that person or their
agents need several concurrent branches. A Git common directory shared for
write access across different Unix users is not the intended security boundary:
it also shares refs, configuration, locks, hooks, and administrative metadata.

### Credentials belong to the human, not the project

Alice must not be able to use Bob's Git credentials, and Bob must not be able to
use Alice's. Project directories therefore contain no shared human credentials.
Credential isolation follows the Linux UID through a private home, a per-user
agent socket, or another reviewed per-user mechanism.

The exact outbound Git credential method remains open. The review must compare
at least:

- SSH agent forwarding, which keeps private keys on the client; and
- a protected per-person key on the Soda machine, which is simpler for clients
  that do not support forwarding consistently.

Root remains inherently trusted. Protecting credentials from the machine
administrator would require a different trust and isolation model and is not
implied by ordinary multi-user Unix separation.

### Collaboration happens through Forgejo

Alice and Bob see each other's work by fetching branches, inspecting readable
workspaces when useful, and reviewing commits or pull requests in Forgejo.
Forgejo, not a parallel Soda model, owns Git-specific users, keys, teams,
repositories, collaborators, issues, pull requests, and releases.

## Target product workflow

The intended install-to-development path is:

1. Install the architecture-matched Soda bootc image.
2. Join the machine to the owner's Tailnet.
3. Open standard Cockpit through the Tailnet.
4. Create or import Linux people using supported system facilities.
5. Create a project from a new Forgejo repository or an existing Git URL.
6. Create or select the project Unix group and Forgejo team.
7. Add collaborators to those existing authorization boundaries.
8. Create one person-owned checkout for each collaborator.
9. Show ready-to-use SSH connection guidance for Codex, Claude, Zed, VS Code,
   and normal OpenSSH clients.
10. Let Git and Forgejo own branches, sharing, review, and history.

Soda owns the branded installation, package and configuration composition,
project conventions, and the smallest glue needed to make this flow reliable.

## Decisions already made

- Soda is a remote-development appliance for powerful shared machines and
  lightweight clients.
- Access is private through Tailscale; Soda is not designed for public-Internet
  exposure.
- SSH is the primary development and administration path for remote tools.
- AArch64 and x86-64 remain equal sibling architectures.
- Forgejo remains the bundled internal Git service, while external repositories
  remain supported.
- One Linux user represents one human.
- Linux administrator status is authoritative for Soda administration.
- Each collaborator needs a conflict-free, attributed project checkout.
- Human Git credentials are isolated per Linux user and never shared through a
  project account or project directory.
- Existing systems remain authoritative unless a concrete Soda-only fact proves
  otherwise.
- The current implementation is not a compatibility contract. Pre-release
  scaffold state receives no migration machinery unless explicitly required.

## Decisions still open

- The exact person-owned workspace layout and read permissions.
- SSH agent forwarding versus protected server-side per-person Git keys.
- Whether any project service account remains necessary.
- The smallest project-selection SSH helper, if one remains necessary.
- The exact Cockpit package and privilege boundary.
- Whether Soda retains any independent durable runtime database.
- Whether a long-running `sodad`, protobuf, or gRPC boundary survives.
- The existing toolchain manager or package boundary to adopt.
- The minimum Soda-specific update policy and UI above bootc.

These choices must be resolved from product behavior and verified upstream
capabilities, not from the shape of the current code.

## Review issues

The reset is tracked as separate ownership reviews so each decision can be made
and implemented deliberately:

- [#32: replace the standalone dashboard with Cockpit composition](https://github.com/LevitateOS/soda-os/issues/32)
- [#33: make Linux identity and administrator status authoritative](https://github.com/LevitateOS/soda-os/issues/33)
- [#34: remove duplicate Linux host telemetry](https://github.com/LevitateOS/soda-os/issues/34)
- [#35: make Unix, Git, and Forgejo own project state](https://github.com/LevitateOS/soda-os/issues/35)
- [#36: reduce project sessions to minimal OpenSSH glue](https://github.com/LevitateOS/soda-os/issues/36)
- [#37: make Forgejo authoritative for Git collaboration](https://github.com/LevitateOS/soda-os/issues/37)
- [#38: reduce Soda update code to bootc policy](https://github.com/LevitateOS/soda-os/issues/38)
- [#24: replace custom toolchain distribution machinery](https://github.com/LevitateOS/soda-os/issues/24)
- [#39: justify or remove SQLite, gRPC, and `sodad`](https://github.com/LevitateOS/soda-os/issues/39)

Concrete correctness fixes may proceed independently. They must not be used to
polish an ownership boundary that the corresponding architecture review has not
yet justified.

## Non-goals of the reset

- Rebuilding Linux, Cockpit, OpenSSH, Git, Forgejo, bootc, or an existing
  toolchain manager behind a different Soda API.
- Adding a browser IDE or replacing SSH-capable development clients.
- Adding public-Internet exposure, enterprise identity, generic orchestration,
  workflow engines, event buses, or background policy systems.
- Promising container or VM isolation that has not been selected as a product
  requirement.
- Removing established user outcomes merely to reduce a line count.

The objective is a smaller and more legible product because the correct systems
own their natural responsibilities, not because smallness is itself the product.
