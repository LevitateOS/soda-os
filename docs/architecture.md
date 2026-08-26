# Architecture

Soda OS keeps four ownership boundaries:

1. Rocky Linux owns the base operating system and standard administration
   facilities.
2. TOML specifications own Soda identity, paths, and toolchain source policy.
3. Rust owns privileged orchestration, persistence, project sessions, and image
   build sequencing.
4. The Go/HTMX cockpit owns the human-facing management website and calls the
   Rust daemon over a local Unix socket.

The daemon is the sole writer for Soda's SQLite state and system project
resources. It is not exposed over TCP. Project service accounts own project
repositories and processes. Individual human SSH keys select actor-specific Git
worktrees while retaining the project Unix UID.
