# Architecture

Soda OS keeps four ownership boundaries:

1. Rocky Linux owns the base operating system and standard administration
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
authentication.

The daemon is the sole writer for Soda's SQLite state and system project
resources. It is not exposed over TCP. Project service accounts own project
repositories and processes. Individual human SSH keys select actor-specific Git
worktrees while retaining the project Unix UID.

## Identity model

- A person is a normal Linux account plus Soda metadata, an SSH public key, and
  an `admin` or `developer` role.
- A project is a locked `soda-p-<slug>` service account whose home is
  `/srv/soda/projects/<slug>`.
- A membership connects a person to a project. Its default worktree is
  `/srv/soda/projects/<slug>/worktrees/<username>`.
- Project SSH uses the project account name, while the forced-command gateway
  identifies the human from the authorized key and enters that person's
  worktree under the shared project UID.

People therefore remain the authentication and attribution boundary, while the
project account remains the filesystem and process ownership boundary. Two
people collaborate without sharing credentials and without giving their normal
Linux accounts ownership of project files.

## Runtime state

`sodad` exposes gRPC only through `/run/soda/sodad.sock`. It stores people,
projects, memberships, worktrees, toolchain resolutions, and provisioning jobs
in `/var/lib/soda/soda.db`. Project repositories live under `/srv/soda/projects`
and publisher-resolved toolchains are cached under `/opt/soda/toolchains`.

The four initial profiles are Web (Node.js and Bun), Python (Python and uv),
Rust (rustc and Cargo), and Go. A project records exact versions after the first
successful resolution; retries reuse that resolution instead of silently
moving the project to a newer stable release.

## Deliberate scaffold limits

There are no deletion or update endpoints, browser IDE, web terminal,
containers, profile switching, or Internet-facing management API. Trusted-LAN
deployment does not remove individual PAM authentication or SSH attribution.
