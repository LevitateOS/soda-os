# People and access

Create primary Linux accounts, establish individual access, and keep workspace, Git-host, and administrator permissions distinct.

## Add a person

As a Linux administrator in **Cockpit → Accounts**:

1. Create a normal primary Linux user with a stable username. Do not use a
   workspace username or create development accounts manually.
2. Set a Linux password using Cockpit's protected controls. Grant administrator
   membership only if this person should administer the machine.
3. Add their personal public SSH key to **Authorized public SSH keys**. They keep
   the private key on their own client.
4. Give them the server's approved trusted-LAN or Tailnet address and verified
   host/certificate identity. For cloud access, grant Tailnet access through
   native Tailscale administration.

Native Linux account tools are also supported. A primary account needs a usable
password for console, Cockpit, Forgejo PAM, and password-based sudo; an SSH key
alone is not that password. Soda uses normal accounts with real homes. Deliver
an initial password through a separate trusted channel and have the person
change it after first login.

The person follows [First connection](../20-Deploy/30-first-connection.md), signs
into [Forgejo](../30-Use-Soda/30-forgejo.md) with their Linux credentials, and gets
an ordinary Forgejo profile. Login order and Linux `wheel` membership do not
change that role. An existing Forgejo administrator can explicitly promote them
if needed; repository access remains native to the Git host.

## Create only the workspaces they need

The new person opens Projects and selects **Set up for me** for each relevant
project. Setup copies their current inbound public keys once and creates a
private outbound Git key. They register that key at the Git host and retry setup
to complete the clone. See [the complete workflow](../30-Use-Soda/20-projects-and-workspaces.md).

Catalog access is shared among primary humans, but it does not grant permission
to clone every repository. Workspaces never create Forgejo identities and cannot
log into the Cockpit Projects page as primary humans.

## Change a password, key, or role

- **Password:** change it through native Linux/Cockpit administration. PAM uses
  the current Linux password; a separate CLI-created Forgejo administrator has
  its own Forgejo password.
- **Inbound SSH key:** update the primary account's authorized keys for future
  workspaces and update existing workspace authorized keys separately. Test a new
  key before revoking the old one; there is no ongoing key synchronization.
- **Outbound Git key/token:** revoke or replace it through the Git host and the
  relevant workspace's native configuration. Linux inbound-key removal does not
  revoke Git credentials.
- **Linux administrator:** edit native `wheel` membership. This changes host
  administration, not Forgejo site roles or repository access. Start a fresh
  login session before relying on changed group membership.
- **Forgejo role:** use native Forgejo user administration. It does not grant sudo.
- **Tailnet access:** use Tailscale's policy/device administration. It changes
  reachability, not Linux or Git-host account ownership.

Keep primary usernames stable: they are part of the workspace-account convention.
Native renaming is not a coordinated Soda identity migration. Plan with the
administrator before changing identity or home ownership.

## Remove access or delete a person

Coordinate access revocation across Linux accounts/keys, active processes,
Forgejo sessions/tokens/keys, other Git hosts, and Tailscale. Treat these as
separate native owners rather than assuming one deletion revokes everything.

For destructive local removal, use **Projects → People → Remove local person**
with administrator access. It removes workspaces before the primary account;
stock Accounts or command-line deletion does not cascade. Canonical repositories
and the Forgejo account survive. Review
[Data safety and removal](40-data-safety-and-removal.md#before-removing-a-person)
before confirming.
