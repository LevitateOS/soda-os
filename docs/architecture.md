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
authentication and password changes.

The daemon is the sole writer for Soda's SQLite state and system project
resources. It is not exposed over TCP. Project service accounts own project
repositories and processes. A registered device-key fingerprint identifies the
person, while the SSH project account selects the project. The gateway enters
that person's personal workspace while retaining the project Unix UID.

## Identity model

- A person is a normal Linux account plus Soda metadata and an `admin` or
  `developer` role. Each person can register multiple named public SSH device
  keys; Soda stores no private keys.
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
It intentionally requires a fresh database. Project repositories live under
`/srv/soda/projects` and publisher-resolved tools are cached under
`/opt/soda/toolchains`.

The four initial profiles are Web (Node.js and Bun), Python (Python and uv),
Rust (rustc and Cargo), and Go. A project records exact versions after the first
successful resolution; retries reuse that resolution instead of silently
moving the project to a newer stable release.

## Deliberate scaffold limits

There are no deletion or update endpoints, browser IDE, web terminal,
containers, profile switching, or Internet-facing management API. Trusted-LAN
deployment does not remove individual PAM authentication or SSH attribution.
