# Connect and develop

Use LAN or Tailscale, ordinary SSH, mise, and normal development-server links.

The Soda machine runs development work while your laptop, editor, terminal, or
browser remains the interface.

## Product contract

### Reach the machine

On a trusted local network, use the Soda machine's LAN name or address. SSH,
Cockpit, Forgejo, and project development servers are directly reachable.

In a cloud environment, join the machine's Tailnet and use its Tailscale name
or address. SSH, Cockpit, and Forgejo are never public Internet services.

### Connect directly to a workspace

Projects shows the workspace username and host. Connect with ordinary OpenSSH:

```text
ssh <workspace-username>@<soda-host>
ssh <workspace-username>@<soda-host> <command>
scp <file> <workspace-username>@<soda-host>:<destination>
sftp <workspace-username>@<soda-host>
```

There is no Soda SSH gateway, project selector, forced command, or synthetic
home. SSH-capable editors and agents use the same host and workspace username.

The copied public keys authorize inbound workspace access. They do not provide
an outbound private key. Git uses SSH, and any additional Git-host
authentication remains private to the workspace.

Tea and GitHub CLI are installed in every workspace. Authenticate them manually
and separately inside that workspace.

### Install development tools

Use `mise` to select tools for **my workspace** or **this project**. Project-
shared tools are stored once and reused by its workspaces; installed
dependencies and mutable state remain workspace-private.

Coding assistants are personal to a workspace. Select and authenticate them
there rather than copying credentials from another account.

### Share development-server links

Projects choose non-conflicting host ports. Send a teammate the normal server
URL using the Soda LAN or Tailscale address; ordinary links and hot reload work
without a Soda Share button or server registry.

Soda does not track ports or processes, proxy project traffic, or manage
containers. Projects may use ordinary tools such as rootless Podman when useful.
