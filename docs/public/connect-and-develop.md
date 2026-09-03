After selecting **Set up for me**, work directly in the resulting workspace
with ordinary OpenSSH clients. The remote shell, editor server, commands,
transfers, development agents, and project services run as the workspace's real
Linux user on the Soda machine.

## Before you connect

You need:

- a client device authorized on the Soda machine's Tailnet;
- the private key matching a public key copied into the workspace;
- the workspace username and SSH command shown on **Projects**; and
- Tailnet policy that permits SSH access to the machine.

The workspace has no login password. Public-key authentication is the normal
direct access path.

## Open a workspace shell

Use the command shown for the workspace:

```sh
ssh <workspace-user>@<soda-hostname>
```

Confirm the remote identity and enter the clone:

```sh
id
cd "$HOME/Projects/<repository>"
git status
```

Commands run directly as the workspace user. There is no forced Soda command,
synthetic home, or intermediate session service.

## Run commands and automation

Ordinary non-interactive SSH behavior remains available:

```sh
ssh <workspace-user>@<soda-hostname> 'cd "$HOME/Projects/<repository>" && make test'
```

Scripts and automation can use the same OpenSSH options, host aliases, agents,
and key selection used with other Linux servers. Process ownership on the Soda
machine remains attributable to the workspace Linux user.

## Transfer files

Use SCP for direct copies:

```sh
scp ./notes.txt <workspace-user>@<soda-hostname>:Projects/<repository>/
```

Use SFTP for interactive or graphical file transfer:

```sh
sftp <workspace-user>@<soda-hostname>
```

Files created through either protocol receive ordinary Linux ownership and
permissions inside the workspace.

## Connect an editor or coding agent

Configure the editor's standard SSH remote feature with the workspace username
and Soda Tailnet hostname. Open the repository directory under
`$HOME/Projects/<repository>` after the connection succeeds.

Editor helpers, language servers, terminals, debuggers, and coding agents run
inside the workspace. Their files, caches, sockets, and processes belong to
that workspace account rather than to the primary account or another
developer's workspace.

## Keep inbound SSH and outbound Git separate

The authorized key copied during setup permits inbound SSH to the workspace.
It does not automatically authenticate outbound Git commands.

For `git fetch`, `git pull`, and `git push`, use the canonical host's ordinary
authentication:

- forward an SSH agent when appropriate;
- use the private Tea configuration copied for the same human; or
- configure a provider credential privately in the workspace.

Soda does not retain or synchronize Git credentials. Repository access and
collaboration remain owned by Forgejo or the external Git provider.

## Use the installed development tools

Soda includes a broad reviewed collection of language runtimes and development
tools in the operating-system image. Use ordinary project conventions for
additional packages, version-specific dependencies, virtual environments, and
user-local configuration.

Each workspace has a separate home, so one project's dependencies and caches
do not overwrite another project's user-local state.

Rootless Podman is available when a project benefits from containers. It is an
optional developer tool, not the mechanism that creates or owns Soda
workspaces.

## Choose project ports deliberately

Workspace accounts share the host network. Projects choose non-conflicting
ports for development servers, databases, and other listeners. When a bind
fails with `address already in use`, inspect the native host state:

```sh
ss -ltnup
```

Coordinate long-lived ports with the trusted team. Soda does not allocate,
proxy, or namespace project ports.

For the ownership and trust boundary behind this workflow, read
[Product model](product-model.md). For host-level work, use
[Administration](administration.md).
