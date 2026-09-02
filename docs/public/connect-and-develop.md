The Soda machine is where development runs; a laptop or other lightweight
device is the interface. Developers connect with the same SSH-capable
terminals, editors, file-transfer tools, and agents they already use.

You encounter this workflow after an administrator has enrolled the machine in
the private network and after **Set up for me** has created your project
workspace: a dedicated Linux account, home, and clone for you and that project.

## Product contract

### Reach Soda through the Tailnet

A **Tailnet** is the private network created by Tailscale for an organization
or group of devices. Soda OS assumes a trusted team reaches OpenSSH,
**Cockpit** (Fedora's browser administration interface), **Forgejo** (the
bundled Git hosting service), and other managed services through that network.
Those services are not designed for direct exposure to the public Internet.

The owner controls Tailnet membership and policy. An administrator keeps the
Soda machine enrolled. A developer connects from another authorized Tailnet
device using the machine's Tailnet name or address.

### Connect to the workspace, not through a gateway

After setup, the Projects page shows a command in this form:

```text
ssh <workspace-username>@<soda-tailnet-host>
```

That is an ordinary OpenSSH login to a real Linux account. Soda adds no project
selector, forced command, synthetic home, or SSH gateway. The same identity
works with familiar SSH behavior, including:

```text
ssh <workspace-username>@<soda-tailnet-host> <command>
scp <file> <workspace-username>@<soda-tailnet-host>:<destination>
sftp <workspace-username>@<soda-tailnet-host>
```

SSH-capable editors and coding agents use the same host and workspace username.
Interactive shells, non-interactive commands, editor processes, automation,
SCP, and SFTP run as the workspace's actual Linux user in its real home.

The SSH public keys copied during setup authorize access to the Soda
workspace. Outbound Git authentication is separate. A key that lets you enter
the workspace does not automatically give that workspace access to GitHub,
Forgejo, or another repository host. Use an ordinary Git method that suits the
host and project; Soda does not choose one or retain credentials.

### Use the installed tools or add project-local ones

Soda's image includes a broad reviewed set of language runtimes, compilers,
build systems, Git and SSH clients, container tools, data and network
utilities, archive tools, and terminal editors. They are normal commands on
`PATH`, available to primary and workspace accounts.

Project-specific packages, virtual environments, language caches, and
dependencies belong in the workspace home or repository. Soda does not have a
runtime toolchain downloader, version profile, or readiness database.

### Coordinate ports on the shared host

Workspace accounts separate files and processes, not the network stack. Two
development servers cannot listen on the same host address and port at the
same time. The team or project must choose non-conflicting ports.

Rootless Podman, Buildah, and Skopeo are available as ordinary tools. Podman is
optional; Soda does not use it to create workspaces and does not manage
containers or project networks.

See [Accounts and workspaces](accounts-and-workspaces.md) for the identity and
key model or [Projects and Git](projects-and-git.md) for initial setup.

## Current implementation

The current fixed ingress rules allow managed TCP services only from the local
machine and the Tailscale interface: OpenSSH on port 22, stock Cockpit on port
9090, and Forgejo on port 30000. Other interfaces are rejected for those
ports. Forgejo advertises SSH clone URLs through OpenSSH on port 22 rather than
running a second embedded SSH server.

The installed command list currently includes Go, Python, uv, Rust, Node.js,
Bun, C and C++ tools, Git, Git LFS, GitHub CLI, OpenSSH tools, Podman, Buildah,
Skopeo, SQLite, common data, network, and archive utilities, and terminal
editors. These are immutable image contents, not a promise of on-demand latest
versions.

One native x86-64 installation has exercised a direct attributed workspace
command, the installed command set, and rootless Podman. Earlier focused
installed evidence also covered an interactive shell, SCP, SFTP, and password
rejection. The consolidated installed transport scenarios and full
multi-user acceptance are not yet complete.

The same installed-product path has not yet been verified on matching-native
AArch64 hardware. There is also no public Soda OS release to connect to unless
an owner is working with locally built pre-release artifacts.
