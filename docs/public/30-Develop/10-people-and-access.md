# Add people and manage access

Create one primary Linux identity per teammate, register their Forgejo SSH access, and grant administrator capability only through `wheel`.

## Understand the identities

- A **primary Linux account** is the person's stable identity and owns their
  password and public SSH keys.
- Membership in **`wheel`** grants administrator capability.
- A **Forgejo account** is the same person's repository identity.
- A **workspace account** is the person's development identity for one project.

Development happens in workspace accounts, not in the primary home.

## Prerequisites

- Sign in to Cockpit as a primary account in `wheel`.
- Obtain the new person's chosen username, initial password, and SSH public
  key through a trusted channel.
- Confirm that the public key belongs to that person. Never request or accept a
  private key.

## Add a person

1. Open Cockpit at `https://SODA_HOST:9090`.
2. Open **Projects** and select **Add person**.
3. Enter the username.
4. Enter and confirm the initial password.
5. Paste the person's SSH public key.
6. Review the result and submit the operation once.

Soda creates an ordinary primary Linux account, a corresponding non-admin
Forgejo account, and registers the public key with Forgejo. The person is not
added to `wheel`.

Give the initial password to the person through a separate trusted channel.
They should change it after their first login.

## Promote an administrator

Use stock Cockpit rather than a Soda role page:

1. Open **Accounts** in Cockpit.
2. Select the primary account.
3. Grant administrator capability, which adds the account to `wheel`.
4. Ask the person to start a new login session before relying on the new
   capability.

Promotion changes Linux administration capability. It does not make the person
a Forgejo site administrator.

## Maintain passwords and keys

Use Cockpit **Accounts** or native Linux tools for primary-account passwords.
Use standard `~/.ssh/authorized_keys` files for Linux SSH access and Forgejo's
native SSH-key interface for repository access. The [Forgejo user
guide](https://forgejo.org/docs/latest/user/) covers its account and repository
features.

Workspace setup copies the person's current public authorized keys once. A
later primary-key change does not silently modify existing workspaces; update
those standard files explicitly where needed.

Soda never copies private keys, Tea configuration, GitHub CLI configuration, or
tokens into another account.

## Verify the result

Ask the person to:

1. Sign in to Cockpit with the primary Linux username and password.
2. Sign in to Forgejo with the corresponding account.
3. Confirm the registered public key in Forgejo.
4. Open **Projects** and create their first workspace.

## Expected result

The teammate has one ordinary primary Linux account, one corresponding
non-administrator Forgejo account, and the same public key registered for Linux
and Forgejo access. They have no workspace until they select **Set up for me**.

## If adding a person fails

Soda stops at the failing boundary and reports which parts succeeded. Do not
assume a hidden rollback occurred. If the Linux account or Forgejo account now
exists, inspect it through Cockpit or Forgejo before retrying the reported
remaining step.

## Remove a person

Person removal is destructive and administrator-only. Read [Data safety and
removal](../40-Operate-Soda-OS/30-data-safety-and-removal.md) before using it.
The operation deletes the person's workspaces first, Forgejo account second,
and primary Linux account last.

## Next step

Read [Projects and workspaces](20-projects-and-workspaces.md).
