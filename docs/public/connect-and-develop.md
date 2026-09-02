The Soda machine is the development system. Your laptop or other lightweight
device is the interface used to reach it with ordinary SSH-capable tools.

## Product contract

Soda OS assumes a trusted team connected through a private Tailnet. It does not
design OpenSSH, Cockpit, Forgejo, or project services for direct public-Internet
exposure.

Developers connect directly to their derived workspace account with OpenSSH.
Interactive shells, remote commands, automation, SCP, and SFTP all run as that
workspace's real Linux UID in its real home. Soda adds no selector, forced
command, synthetic home, or SSH gateway.

SSH-capable editors and agents work through the same standard connection. The
image supplies a broad reviewed set of compilers, runtimes, build tools, Git and
SSH clients, container tools, command-line utilities, archives, and editors.
Project-specific packages, caches, virtual environments, and dependencies stay
in ordinary user-owned homes and repositories.

Projects share the host network and select non-conflicting ports themselves.
Rootless Podman is available as an optional project tool, not a Soda-managed
container or isolation system.

## Current implementation

Native nftables permits managed TCP ports only from loopback and `tailscale0`:
OpenSSH on 22, stock Cockpit on 9090, and Forgejo on 30000. Forgejo advertises
SSH clone URLs through OpenSSH on port 22; its embedded SSH server is disabled.

After setup, the Projects page shows a command in this form:

```text
ssh <workspace-username>@<soda-tailnet-host>
```

The installed command contract currently includes Go, Python, Rust, Node.js,
Bun, C and C++ tooling, Git, GitHub CLI, OpenSSH tools, Podman, Buildah, Skopeo,
SQLite, common data and archive utilities, editors, and supporting build tools.
These commands are immutable image content available through normal `PATH`;
there is no Soda runtime toolchain downloader or profile manager.

Native x86-64 installation evidence has exercised direct workspace commands,
the immutable toolset, and rootless Podman. Earlier focused evidence also
covered shell, SCP, SFTP, and password rejection. The current installed-product
path still needs matching-native AArch64 repetition and the final consolidated
acceptance workflow.
