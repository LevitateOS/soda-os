Use **Add person…** to give a new developer a complete Soda OS identity. The
operation creates the person's primary Linux account, SSH access, native
Forgejo identity, and private Tea configuration together.

Only an administrator—a primary Linux account in `wheel`—can add or remove a
person. The new person begins as a developer without Linux administrator
access.

## Before you begin

Collect these values from the person:

- a stable lowercase Linux username;
- a temporary initial password; and
- one valid OpenSSH public key.

The username becomes the person's durable Linux and Forgejo name and is used to
derive their workspace associations. Choose it carefully; renaming a primary
account with existing workspaces is not part of the Soda workflow.

The public key normally begins with a type such as `ssh-ed25519` and ends with
an optional comment. Ask for the public `.pub` file, never the private key.

## Add the person

1. Sign in to Cockpit as an administrator.
2. Open **Projects**.
3. Find the **People** panel.
4. Select **Add person…**.
5. Enter **Username**.
6. Enter and confirm **Initial password**.
7. Paste the person's **SSH public key**.
8. Select **Add person**.

The password is used only to create the Linux account and its native Forgejo
login. Soda does not retain a separate copy.

## Expected result

A successful operation leaves:

- an ordinary primary Linux account with a private home;
- the supplied public key in the account's standard
  `~/.ssh/authorized_keys`;
- a same-named ordinary Forgejo user;
- a private Tea login owned by that person; and
- no `wheel` membership unless an administrator grants it separately.

**Tea** is Forgejo's command-line client. Soda places that person's private Tea
configuration in their primary home so future workspace setup can copy it
once. It is user-owned configuration, not a shared Soda credential.

The person can now sign in to Cockpit, use Forgejo, open **Projects**, and
select **Set up for me** for any repository they can access.

## Grant administrator access

Grant administrator access only when the person must operate the host:

1. Open Cockpit's stock **Accounts** page.
2. Select the primary Linux account.
3. grant administrator access by adding it to `wheel`.
4. Ask the person to begin a new session before relying on the changed group.

Linux `wheel` membership controls Soda administrator access. It does not make
the person a Forgejo site administrator. Manage Forgejo roles and repository
permissions in Forgejo.

Removing someone from `wheel` removes Linux administrator access without
deleting their account, workspaces, or Forgejo identity.

## Manage SSH access

The primary account's standard `authorized_keys` is the source used when a new
workspace is created. Soda copies the valid public keys into that workspace
once. Later key changes in the primary account do not rewrite existing
workspace accounts.

Use ordinary Linux and OpenSSH tools to add, rotate, or remove keys in an
existing primary or workspace account. Removing a key affects new SSH
connections but does not delete files or terminate unrelated processes.

## Keep account ownership clear

Linux remains authoritative for the primary account, password, groups, home,
and processes. Forgejo remains authoritative for its same-named user,
repository access, and tokens. Tea remains the user's private Forgejo CLI
configuration.

Use **Add person…** for complete Soda onboarding rather than creating only a
Linux account. Use stock Cockpit for ordinary password and `wheel` management
after onboarding.

Continue with [Projects and workspaces](projects-and-workspaces.md). Before
deleting anyone, read [Data safety and removal](data-safety-and-removal.md).
