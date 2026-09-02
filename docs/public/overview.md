Soda OS turns a powerful Linux machine into a shared remote development
computer. A lightweight laptop stays the interface while builds, tests,
language tools, development servers, databases, agents, and project processes
run on the Soda machine.

> **Pre-release documentation:** Soda OS does not currently have a public tag
> or downloadable release. These pages explain the accepted product model and
> identify what the present source and product evidence have demonstrated.

## Product contract

Soda OS is an opinionated Fedora bootc appliance for private remote
development. It combines Linux, OpenSSH, Tailscale, stock Cockpit, Git, bundled
Forgejo, optional Podman, and a broad development toolset without rebuilding
those systems behind a Soda control plane.

The business or technical owner chooses and administers the machine. Developers
connect from SSH-capable editors and terminals. Each person has a primary Linux
account, and each person-project pair can receive an independent workspace
account with its own home and complete Git clone.

Soda owns the installable composition, project catalog, workspace convention,
focused Cockpit Projects page, and the narrow synchronous operations needed to
join those pieces. Linux owns accounts and processes. The canonical Git host
owns repositories and collaboration. Tailscale owns private reachability.
OpenSSH owns remote sessions. Cockpit owns general host administration, and
bootc owns image deployment.

Continue with the [installation model](installation-model.md), or go directly
to [accounts and workspaces](accounts-and-workspaces.md) to understand daily
development.

## Current implementation

The source currently composes Fedora 44 bootc images for equal x86-64 and
AArch64 product targets. The protected stock-Anaconda installation path,
initial administrator, Tailscale enrollment, Cockpit Projects workflow, direct
workspace SSH, immutable toolset, and account-preserving image selection have
been exercised together on native x86-64 hardware.

Matching-native AArch64 construction and installed-system evidence still need
to repeat the current protected installer and complete product path. Later
primary users cannot yet sign in to Forgejo through PAM because enabling the
pinned Forgejo process to verify Linux passwords requires an unresolved
privilege decision.

Final automation for multi-user destructive scenarios is incomplete. A
temporary health-only `sodad` and `sodactl health` surface also remains until
the final control-plane deletion milestone. It does not participate in project,
workspace, authentication, telemetry, toolchain, or update behavior.

There is no published installer or production update channel today. The
repository can build local unsigned candidate artifacts, but those maintainer
procedures are deliberately outside this conceptual handbook.
