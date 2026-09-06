# Forgejo: built-in Git

Use Forgejo for repositories, SSH keys, permissions, issues, reviews, and collaboration on your Soda server.

## Sign in

Open the Forgejo URL from the Soda welcome message, normally
`http://SODA_HOST:30000`. For trusted LAN access, an administrator first allows
that port through [Networking → Firewall](../50-Operate/20-administration.md#service-endpoints).
Cloud access uses the Tailnet address, never public service ingress.

Sign in with your **primary Linux username and password**. PAM authenticates
your Linux credentials and Forgejo creates an ordinary profile on first login,
including for the first human and Linux administrators. No first-owner signup
or administrator prerequisite is needed. Workspace usernames are Linux-only
and cannot be used as Forgejo identities.

Forgejo keeps its own profile, keys, permissions, and roles. Changing Linux
`wheel` membership does not grant a Forgejo role. A separately CLI-created
administrator uses its separate Forgejo password, as described below.

## Create a repository

1. Sign in and use Forgejo's native **New repository** action.
2. Select the owner, name, visibility, and initialization options appropriate
   for the project; create it and review access settings.
3. Copy the repository's **SSH** clone URL.
4. In Cockpit **Projects**, select **Add repository** and enter that URL.
5. Select **Set up for me** when you want your personal workspace.

Creating a repository and adding it to Soda are separate operations. For a
repository already at an external Git host, keep using that host's native
permissions and collaboration and add its SSH URL directly to Projects.

Forgejo owns native organizations/teams, issues, pull requests, releases, and
repository deletion. Follow the [Forgejo user guide](https://forgejo.org/docs/latest/user/)
for those workflows. Adding a Soda catalog entry does not grant Git access.

## Register a workspace key

When Projects reports that Git access is needed:

1. Copy the **workspace public key** shown by Projects.
2. Open Forgejo's user settings and **SSH / GPG Keys**.
3. Add the SSH public key with a label identifying the Soda workspace.
4. Return to Projects and select **Retry setup**.

The key must belong to an account with access to the repository. Each workspace
has its own outbound private key, which stays there. Register personal client
keys separately if you also clone directly from your client. Removing an inbound
key from Linux does not revoke a Git key registered in Forgejo.

Tea is available inside workspaces; [authenticate it manually there](../40-Develop/10-connect-and-develop.md#sign-in-to-forgejo-and-github-clis).
Browser login and Tea login are not copied between workspaces.

## Create a Forgejo administrator

Site administration is separate from normal repository use. If no usable
Forgejo administrator exists, a **Linux administrator** can create a new one
with Forgejo's native CLI. This creates no Linux account and does not promote
an existing PAM user.

On the Soda server, in a private interactive terminal, replace `forgejo-admin`
with an unused Forgejo username distinct from people's Linux usernames, and
replace the email:

```sh
sudo -H -u git /usr/bin/forgejo \
  --work-path /var/lib/forgejo --config /etc/forgejo/app.ini \
  admin user create \
  --username forgejo-admin --email you@example.com --admin \
  --random-password --random-password-length 24 --must-change-password=true
```

**The command prints the generated password.** Store it securely. Do not run it
through recorded/shared terminal sessions, CI logs, or shell tracing. Forgejo
generates the password rather than receiving it in command arguments.

Sign in to Forgejo with that new username/password, complete the required password
change, and open **Site Administration** from the account menu. No database reset
or service restart is needed. To inspect native administrators:

```sh
sudo -H -u git /usr/bin/forgejo \
  --work-path /var/lib/forgejo --config /etc/forgejo/app.ini \
  admin user list --admin
```

### Promote a PAM user

An existing Forgejo administrator opens **Site Administration → User Accounts**,
edits the intended user, enables **Administrator**, and saves. The user keeps
Linux/PAM authentication. Do not delete/recreate an account to promote it; the
bundled native CLI creation command is not an existing-user promotion command.
Forgejo's safeguards for changing existing administrators still apply.

### Registration and configuration

New configurations set `[service] DISABLE_REGISTRATION = true`. Ordinary PAM
account creation and explicit CLI administrator creation still work. Updates
preserve existing configuration and roles. To change an operator's registration
policy, review `/etc/forgejo/app.ini`, edit that setting in its existing section,
and restart `forgejo.service` through native administration.

If an operator enables browser registration on an empty instance, native Forgejo
can grant its first registered user administration. Soda does not replace that
upstream behavior. Existing ordinary accounts do not require a database reset
before explicit administrator creation.

## Keep data and access separate

Soda workspace, project, and person removal preserve the canonical repository.
Person removal also preserves the Forgejo account. To revoke remaining Forgejo
sessions, tokens, keys, or delete its account, use native Forgejo administration
separately; Linux account deletion is not complete Git-host access revocation.

Back up Forgejo configuration, repositories, and application data together using
[Backups and restoration](../50-Operate/30-backups-and-restoration.md).
For local CI execution, see [Runners](50-ci-runners.md).
