# Connect and develop

Enter your workspace through ordinary OpenSSH and use native Git-host, tool, editor, assistant, port, and container workflows.

## Prerequisites

- Create a workspace through Cockpit **Projects**.
- Record its derived username and the Soda host address.
- Keep the matching private SSH key on your client device.
- Obtain access to the project's authoritative Git repository.

## Connect with OpenSSH

Start an interactive shell:

```sh
ssh WORKSPACE_USER@SODA_HOST
```

Run one command without an interactive shell:

```sh
ssh WORKSPACE_USER@SODA_HOST 'cd "$HOME/Projects/REPOSITORY" && git status'
```

Copy files with SCP or open an SFTP session:

```sh
scp ./local-file WORKSPACE_USER@SODA_HOST:~/Projects/REPOSITORY/
sftp WORKSPACE_USER@SODA_HOST
```

These are normal OpenSSH operations. Consult the upstream manuals for
[SSH](https://man.openbsd.org/ssh.1), [SCP](https://man.openbsd.org/scp.1), and
[SFTP](https://man.openbsd.org/sftp.1).

SSH-capable editors connect with the same host, derived username, and key. Do
not connect an editor as the primary account for development.

## Work with Git over SSH

The repository is under:

```text
$HOME/Projects/REPOSITORY
```

Check its credential-free SSH remote before working:

```sh
cd "$HOME/Projects/REPOSITORY"
git remote -v
git status
```

Use the Git host's native access controls and ordinary Git commands. See
[Git clone and SSH URL documentation](https://git-scm.com/docs/git-clone) and
the [Forgejo user guide](https://forgejo.org/docs/latest/user/).

## Sign in to Forgejo and GitHub CLIs

Authentication is private to this workspace. Soda does not copy a login from
the primary account or another workspace.

For bundled Forgejo, start Tea's interactive login and then verify it:

```sh
tea logins add
tea whoami
```

Use the Soda Forgejo URL and your personal Forgejo account when prompted. See
[Codeberg's Tea and Forgejo CLI guidance](https://docs.codeberg.org/git/clone-commit-via-cli/).

For GitHub CLI:

```sh
gh auth login --git-protocol ssh
gh auth status
```

Follow the interactive device flow documented by [GitHub CLI
authentication](https://cli.github.com/manual/gh_auth_login).

Repeat these logins separately in every workspace that needs them. Never copy
another account's CLI configuration or token.

## Manage development tools

Soda supplies the operating-system capabilities needed to host workspaces.
`mise` owns development-tool discovery, downloads, versions, activation, and
project configuration.

Before adding a tool, change to the repository, decide whether the choice is
personal or shared by the project, and review existing project configuration
before trusting or executing it.

When Projects offers initial tool choices, select as many as the work requires.
Those choices are conveniences, not a closed list.

- Choose **my workspace** for a personal tool or version that should affect
  only your derived account.
- Choose **this project** for a tool and version the team should share across
  workspaces for this project.

Any project user may add shared project tools. Soda does not add a separate
approval or membership system around `mise`.

Inspect the active configuration from the repository:

```sh
cd "$HOME/Projects/REPOSITORY"
mise config ls
mise ls
```

Review an existing `mise.toml` before trusting and activating it. It is project
code and may define environment behavior or tasks.

To add a shared project tool, use `mise use` and commit the resulting project
configuration:

```sh
cd "$HOME/Projects/REPOSITORY"
mise use TOOL@VERSION
git diff -- mise.toml
git add mise.toml
git commit
```

Teammates activate the configured tools from their own workspaces:

```sh
mise install
```

Follow [mise getting started](https://mise.jdx.dev/getting-started.html) for
supported tool names, versions, configuration, activation, and trust behavior.

For a personal tool, use the personal scope offered by Projects or a
`mise.local.toml` ignored by Git. Check the affected files before committing so
a personal choice does not accidentally become project policy.

For a shared project tool, Soda gives `mise` one project-scoped storage
location that every workspace for the project can use. The selected tool is
stored once, while `mise` and the relevant package ecosystem own its layout,
download cache, locking, and concurrency. Project dependencies, virtual
environments, build output, and other mutable development state remain private
to each workspace. Soda does not define a cache format or run a cache service
or dependency downloader.

## Use a coding assistant

Choose assistants during workspace creation or install them later with the
workspace's tool workflow. Sign in separately inside this workspace. Assistant
configuration and credentials remain personal to that workspace. Do not store
assistant credentials in shared project configuration or copy them between
workspace homes.

## Share a development server

Choose a project port that does not conflict with another service, then bind
the development server so it accepts connections from the trusted network. For
example, follow the framework's documented host-binding option instead of
assuming its default loopback binding is reachable.

Send a teammate the normal development URL using the Soda LAN or Tailscale host
name and the selected port. WebSockets and hot reload use that same route. Soda
does not require a Share action or track project ports and processes.

Cloud development URLs are reachable through Tailscale only. LAN installations
may use either the LAN or Tailscale address.

## Use rootless Podman when needed

Run Podman as the workspace user so its containers and storage remain owned by
that Linux identity:

```sh
podman info
podman run --rm docker.io/library/alpine:latest echo ok
```

See the [Podman documentation](https://docs.podman.io/en/stable/markdown/podman.1.html).
Containers are optional development tools, not Soda's workspace isolation
mechanism.

## Expected result

Commands, files, Git activity, dependencies, assistants, processes, and
containers run as the derived workspace UID in its private home. Shared tool
intent stays in project configuration, while personal tools and mutable state
remain private.

## If something fails

Diagnose the native owner: OpenSSH for login or transfer errors, the Git host
for repository authorization, `mise` for tool installation, the framework for
development-server binding, and Podman for rootless-container errors.

For a tool problem, run `mise doctor` and `mise config ls`, then follow the
diagnostic guidance for the tool backend. Do not work around a shared
permission problem by copying another user's installation or credentials.

## Next step

If this machine will execute provider jobs locally, read [CI
runners](40-ci-runners.md). Otherwise continue with
[Administration](../40-Operate-Soda-OS/10-administration.md).
