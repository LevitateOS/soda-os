# Architecture

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
resources. It is not exposed over TCP. Project service accounts own project
repositories and processes. A registered device-key fingerprint identifies the
person, while the SSH project account selects the project. The gateway enters
that person's personal workspace while retaining the project Unix UID.

## Identity model

- A person is a normal Linux account plus Soda metadata and an `admin` or
  `developer` application role. These roles do not create parallel Linux role
  groups. Each person can register multiple named public SSH device keys; Soda
  stores no private keys.
- A project is a locked `soda-p-<slug>` service account whose home is
  `/srv/soda/projects/<slug>`.
- A membership connects a person to a project and owns exactly one personal
  workspace at `/srv/soda/projects/<slug>/worktrees/<username>`.
- Project SSH uses the project account name, while the forced-command gateway
  identifies the human from the authorized key and enters that person's
  workspace under the shared project UID.
- Each project member receives an isolated session home under
  `/srv/soda/projects/<slug>/.soda/people/<username>/home`, including separate
  XDG directories and a `~/workspace` link to the checkout.

People therefore remain the authentication and attribution boundary, while the
project account remains the filesystem and process ownership boundary. Two
people collaborate without sharing credentials and without giving their normal
Linux accounts ownership of project files.

## Runtime state

`sodad` exposes gRPC only through `/run/soda/sodad.sock`. Schema version 2
stores people, SSH device keys, projects, memberships, worktrees, development
environment resolutions, and project-setup jobs in `/var/lib/soda/soda.db`.
It intentionally requires a fresh database. Cockpit certificates also live in
`/var/lib/soda/certs`.

Mutable project repositories and toolchain caches physically live at
`/var/lib/soda/projects` and `/var/lib/soda/toolchains`. Image-owned systemd
bind mounts retain the stable session paths `/srv/soda/projects` and
`/opt/soda/toolchains`; the forced SSH gateway and project account homes keep
using those visible paths. `tmpfiles.d` creates the state and mount-point
directories at boot rather than shipping mutable Soda state in the image.
Soda service logs are written below `/var/log/soda`.

## Sibling platforms

AArch64 and x86-64 are equal Soda OS sibling architectures. Shared Go code owns
the product and runtime behavior; platform TOML files select only the upstream
base, OCI identity, package and tool locks, installer/UEFI inputs, artifact
names, discovery channel, and test harness that genuinely differ. Neither
platform is a default, compatibility path, or experimental target.

Each sibling publishes a single-platform exact-digest release. Discovery tags
(`current-aarch64` and `current-x86_64`) and release-record filenames remain
separate so publication order cannot replace the other architecture. Both use
the same Cosign trust contract, non-disruptive staging, explicit reboot
activation, rollback visibility, persistent-state policy, and equivalent
acceptance gates. Soda does not use a multi-platform discovery index.

Soda-created Linux users and their PAM passwords remain system account state;
SSH host keys remain under `/etc/ssh`; and per-project forced-command key files
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
operator explicitly imports it into Soda.
