# Make the first connection

Connect to the installed server, establish trusted SSH and Cockpit access, and prepare your primary account for workspaces.

Log in normally at the console with the account created by Anaconda or cloud-init.
The interactive welcome shows the native hostname, Cockpit/Forgejo URLs, your
SSH command, and Tailscale status. It appears on each interactive login, not
as a setup wizard to dismiss.

## Choose a reachable address

- **Trusted LAN:** use the LAN address shown by the server. SSH and Cockpit have
  firewall allowances; administrators open Forgejo and development ports later.
- **Cloud:** first complete [native Tailscale sign-in from the console](../30-Use-Soda/40-tailscale.md#initial-cloud-connection).
  Join your client to the allowed Tailnet, then use the server's Tailnet address.
  Do not try its public IP or open public SSH/Cockpit ingress.

Below, `SODA_HOST` means that reachable hostname or address, and `PRIMARY_USER`
means your primary Linux username. If a hostname does not resolve, use the
corresponding IP and check DNS; do not substitute an unrelated machine.

## Open Cockpit and verify its identity

Open `https://SODA_HOST:9090` in your browser. Cockpit is the Soda OS dashboard
for host administration and Projects. Sign in with your **primary Linux username
and password**, not a workspace account or Git-host token.

A newly installed Cockpit can use a self-signed certificate, so your browser
may not already trust it. Check the URL and certificate against the intended
machine using the installation console or your administrator before accepting
that server's exception. Do not disable browser TLS verification globally.
Administrators can install a trusted certificate through
[Cockpit's native certificate configuration](https://cockpit-project.org/guide/latest/https.html).
An unexpected certificate change on an existing server needs investigation.

Enable **Administrative access** only when performing administrator tasks.
See [the Cockpit guide](../30-Use-Soda/10-cockpit.md) for its pages and privileges.

## Add your personal SSH public key

On your client, use an existing personal key or create one with OpenSSH:

```sh
ssh-keygen -t ed25519
```

Use a passphrase and choose a new filename if a key already exists; do not
overwrite it. Keep the private file on your client. Copy only the `.pub` file's
contents, which start with a public key type such as `ssh-ed25519`.

In **Cockpit → Accounts → your account → Authorized public SSH keys**, add that
public key. Cloud-init may already have installed it; check before adding a
second copy. This is the key that permits inbound SSH and is copied once when
you create a workspace. It is different from the workspace's outbound Git key.

## Verify and test SSH

At the Soda console, obtain the SSH host key fingerprint:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

From your client, connect with the corresponding personal private key:

```sh
ssh -i /path/to/personal_private_key PRIMARY_USER@SODA_HOST
```

Compare the first-connection fingerprint with the console result. Accept only
a matching identity; an administrator can provide the appropriate fingerprint
if the negotiated host-key type differs. Never use disabled host-key checking
as a connection fix. A later host-key change needs explanation, such as a
verified reinstall, before updating your client's known-host entry.

After login, `whoami` should show your primary username. Ordinary SSH commands,
SCP, and SFTP remain available without interactive welcome output.

## Open Forgejo and create your workspace

For LAN access, an administrator allows Forgejo HTTP as described in the
[service reference](../50-Operate/20-administration.md#service-endpoints).
Open the Forgejo URL and [sign in with your Linux credentials](../30-Use-Soda/30-forgejo.md).
Your first PAM login creates an ordinary account; it does not grant site
administration, even if you are a Linux administrator.

Then open [Projects](../30-Use-Soda/20-projects-and-workspaces.md), add the repository
if needed, and select **Set up for me**. Use the resulting **workspace** username
for development, not the primary SSH session you just tested.

If a connection fails, check the selected route, username, credentials, and
[the owning service](../50-Operate/20-administration.md#troubleshooting).
