# Connect and develop

Use ordinary SSH and your preferred editor inside a private project workspace, with mise managing its tools.

## Connect as the workspace account

Complete [workspace setup](../30-Use-Soda/20-projects-and-workspaces.md), then
copy the workspace username, SSH command, and clone path from Projects.
The host follows your Cockpit browser address; use its trusted LAN or Tailnet
route from your client.

```sh
ssh WORKSPACE_USER@SODA_HOST
```

Use the personal private key corresponding to the public key copied at setup.
Validate the host fingerprint through [First connection](../20-Deploy/30-first-connection.md#verify-and-test-ssh).
Inside the session, `whoami` should show the workspace username and `$HOME` its
private home. Change to the supplied path beneath `$HOME/Projects/REPOSITORY`.

SSH is not a Soda command tunnel. Remote commands and file transfers work normally:

```sh
ssh WORKSPACE_USER@SODA_HOST 'whoami; printf "%s\n" "$HOME"'
scp ./local-file WORKSPACE_USER@SODA_HOST:
sftp WORKSPACE_USER@SODA_HOST
```

## Open an editor

For VS Code, install Microsoft's **Remote - SSH** extension on the client, connect
to `WORKSPACE_USER@SODA_HOST`, and open the clone path. Follow its
[Remote SSH guide](https://code.visualstudio.com/docs/remote/ssh) for client-key
selection, remote extensions, and forwarded ports. Other SSH-capable editors
use the same native account and path. Terminal editors run directly on Soda.

An optional entry in your client's `~/.ssh/config` makes repeated connections
clearer; choose one alias per workspace:

```sshconfig
Host soda-example
    HostName SODA_HOST
    User WORKSPACE_USER
    IdentityFile ~/.ssh/YOUR_PERSONAL_PRIVATE_KEY
    IdentitiesOnly yes
```

Keep editor servers and extensions in the workspace home. Shared compute does
not require shared writable source files.

## Use ordinary Git

Inspect the complete clone, create branches, commit, push, and review through
your authoritative Git host:

```sh
git status
git remote -v
git switch -c my-change
```

Set your native Git author identity for the workspace or repository when needed.
The workspace's own outbound SSH key authenticates Git; the client key used
for inbound SSH is a different key. On a host-key prompt, verify the Git host's
published/admin-provided fingerprint before accepting it.

Commit and push work that should survive workspace deletion. A commit existing
only in the local clone is not a remote backup.

## Manage development tools

mise is the development-tool manager. Read the repository's instructions and
review its configuration before trusting it:

```sh
mise trust
mise install
mise exec -- TOOL_COMMAND
```

For a project you own, `mise use TOOL@VERSION` records your selected version in
native configuration. Commit the intended configuration changes so teammates
can reproduce them. Use [mise documentation](https://mise.jdx.dev/) for supported
backends, tasks, environment variables, and version management.

Dependencies, caches, runtimes, and assistant installations belong to the workspace.
Authenticate assistants there manually; do not share another person's tokens
or rely on credentials from your primary home.

## Sign in to Forgejo and GitHub CLIs

Tea and GitHub CLI are available in every workspace. Their sessions are separate
from browser authentication and from every other workspace.

- For Forgejo, run `tea logins add` interactively, verify with `tea whoami`, and follow
  [Tea and Forgejo CLI guidance](https://docs.codeberg.org/git/clone-commit-via-cli/).
  Use the reachable Forgejo URL and your own token through a protected prompt.
- For GitHub, run `gh auth login --git-protocol ssh`, verify with `gh auth status`, and follow
  [GitHub CLI authentication](https://cli.github.com/manual/gh_auth_login).

Use native logout/revocation when access should end. Never add tokens to the
catalog, repository, screenshots, or command arguments recorded in shared logs.

## Development services and ports

Workspaces share the host network. Choose a free port and coordinate long-running
services with the team. For local-only access, bind the service to loopback and
forward it over your workspace SSH connection:

```sh
ssh -N -L 8080:127.0.0.1:8080 WORKSPACE_USER@SODA_HOST
```

Open `http://127.0.0.1:8080` on your client; substitute the actual free client and
server ports. Editors can manage equivalent native forwarding.

For direct team access, bind to the intended reachable interface and ask an
administrator to allow only the selected port through the host firewall.
Cloud access remains Tailnet-only. See [the service reference](../50-Operate/20-administration.md#service-endpoints).
Podman is available if a project wants containers; you own their configuration,
storage, and lifecycle.

Before stopping processes, removing a workspace, or restarting the server,
preserve local data and coordinate with users of its services.
