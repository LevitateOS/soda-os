# Architecture

> [!IMPORTANT]
> This document describes the pre-reset implementation currently present in the
> repository. It is retained as implementation evidence, not as the target
> architecture or permanent product contract. See the
> [architectural reset](architecture-reset.md) for the accepted product direction,
> target ownership boundaries, multi-user model, open decisions, and review
> issues.

Soda OS keeps four ownership boundaries:

1. Fedora bootc owns the base operating system and standard administration
   facilities.
2. TOML specifications own Soda identity, paths, and toolchain source policy.
3. Go owns privileged orchestration, persistence, project sessions, and image
   build sequencing.
4. The Go/HTMX cockpit owns the human-facing management website and calls the
   Go daemon over a local gRPC Unix socket.

The network-facing `soda-cockpit` process runs as an unprivileged service.
Linux password checks are delegated over `/run/soda/pam.sock` to the small
root `soda-authd` helper; only members of the local `soda-api` group can reach
that socket. The helper has no network listener and performs only PAM
authentication and password changes.

The daemon is the sole writer for Soda's SQLite state and system project
resources. It is not exposed over TCP. Project service accounts own shared bare
repositories and unattended built-in repository setup. SSH authenticates a
person's Linux account with a registered device key; an optional untrusted
`SODA_PROJECT` selector chooses a project, and the gateway validates membership
before entering that person's private workspace under their own Unix UID.

Forgejo provides Soda's bundled Git repositories. Soda remains the source of
truth for people, roles, memberships, SSH device keys, and projects; the daemon
projects those records into Forgejo and stores only their remote identifiers.
Creating a repository on this Soda server and connecting an external repository
both continue through the same project, membership, and workspace lifecycle.

Forgejo starts alongside the daemon but is not a daemon prerequisite. If it is
unavailable at boot, Soda still reconciles project SSH access and keeps existing
worktrees and external repositories usable. The daemon logs the failed Forgejo
reconciliation, and a later successful restart retries it.

## Identity model

- A person is a normal Linux account plus Soda metadata and an `admin` or
  `developer` application role. These roles do not create parallel Linux role
  groups. Every person belongs to `soda-people`, can register multiple named
  public SSH device keys for inbound access, and receives one server-generated
  Ed25519 key for outbound Git. Soda persists only that Git key's public key and
  fingerprint; its protected private key remains in the person's home.
- A project has a `soda-p-<slug>` service account whose home is
  `/srv/soda/projects/<slug>`.
- A membership connects a person to a project and owns exactly one personal
  workspace at `/srv/soda/projects/<slug>/worktrees/<username>`.
- Project SSH uses the person's username and sends `SODA_PROJECT=<slug>`. The
  forced-command gateway derives the person from the authenticated Unix account,
  validates the selector and membership, and enters the person's workspace.
- Each project member receives an isolated session home under
  `/srv/soda/projects/<slug>/.soda/people/<username>/home`, including separate
  XDG directories and a `~/workspace` link to the checkout.

People are the authentication, attribution, workspace, and process ownership
boundary. The project service account and its primary group own the shared bare
repository; members join that group, while personal worktrees and session homes
remain inaccessible to other members.

## Runtime state

`sodad` exposes gRPC only through `/run/soda/sodad.sock`. Schema version 4
stores people, their public Git identities, SSH device keys, projects including
an external repository's bootstrap person, memberships, worktrees, toolchain
installations and resolutions, provisioning jobs, and built-in Git mappings in
`/var/lib/soda/soda.db`. It intentionally requires a fresh database. Cockpit
certificates also live in `/var/lib/soda/certs`.

Mutable project repositories and toolchain caches physically live at
`/var/lib/soda/projects` and `/var/lib/soda/toolchains`. Image-owned systemd
bind mounts retain the stable session paths `/srv/soda/projects` and
`/opt/soda/toolchains`; the forced SSH gateway and project resources keep using
those visible paths. `tmpfiles.d` creates the state and mount-point
directories at boot rather than shipping mutable Soda state in the image.
Soda service logs are written below `/var/log/soda`.

## Sibling platforms

AArch64 and x86-64 are equal Soda OS sibling architectures. Shared Go code owns
the product and runtime behavior; platform TOML files select only the upstream
base, OCI identity, package and tool locks, installer/UEFI inputs, artifact
names, and test harness that genuinely differ. Neither
platform is a default, compatibility path, or experimental target.

Each sibling produces a single-platform exact-digest artifact. One paired
release index names both sibling images and their architecture-specific ISO and
record assets, so publication cannot advance one architecture alone. Both use
the same non-disruptive staging, explicit reboot activation, rollback
visibility, persistent-state policy, and equivalent acceptance gates.

Soda-created Linux users and their PAM passwords remain system account state;
SSH host keys remain under `/etc/ssh`; and per-person authorized-key files
remain root-owned under `/etc/soda/authorized_keys`. Those paths, the SQLite
database, certificates, logs, projects, and toolchains are persistent host
state, not container state.

The daemon samples host health for the status page. Project and account pages
show current state when requested; the daemon does not poll workspace Git state
or SSH presence and does not maintain a global browser event stream.

The four initial profiles are Web (Node.js and Bun), Python (Python and uv),
Rust (rustc and Cargo), and Go. A project records exact versions after the first
successful resolution; retries reuse that resolution instead of silently
moving the project to a newer stable release.

## Deliberate scaffold limits

There are no project or person deletion or update endpoints, browser IDE, web
terminal, containers, profile switching, or Internet-facing management API.
Trusted-LAN deployment does not remove individual PAM authentication or SSH
attribution. A host administrator remains an ordinary Linux account until an
operator explicitly imports it into Soda. External Git uses SSH URLs only; Soda
does not store HTTPS credentials, broker provider tokens, or export private Git
keys.
