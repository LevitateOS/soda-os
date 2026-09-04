# Add people and manage access

Create one primary Linux identity per teammate, let Forgejo create its own
identity at first normal login, and grant administrator capability only through
`wheel`.

## Understand the identities

- A **primary Linux account** is the person's stable identity and owns their
  password and public SSH keys.
- Membership in **`wheel`** grants administrator capability.
- A **Forgejo account** is the same person's repository identity.
- A **workspace account** is the person's development identity for one project.

Development happens in workspace accounts, not in the primary home.

## Prerequisites

- Sign in to Cockpit as a primary account in `wheel`.
- Obtain the new person's chosen username and initial password through a trusted
  channel.
- Obtain the person's public SSH key through a trusted channel. Confirm that it
  belongs to them. Never request or accept a private key.

## Add a person

1. Open Cockpit at `https://SODA_HOST:9090`.
2. Open stock Cockpit's **Accounts** page and create an ordinary Linux user with
   the person's stable username and initial password. Do not grant administrator
   capability unless that is intended.
3. Install the person's public key in the primary account's standard
   `~/.ssh/authorized_keys` file through ordinary Linux administration.
4. Ask the person to sign in to Forgejo normally with the same Linux username
   and password. Forgejo uses PAM to authenticate the login and creates its
   account at that time.
5. In Forgejo's native user settings, the person registers any public SSH key
   they want to use for repository access.

The Soda **Projects** page does not create people, pre-create later Forgejo
accounts, or register primary-account keys with Forgejo. The person is not
added to `wheel` unless an administrator grants that native Linux capability.

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
features. Soda Setup is the one composition point that creates the initial
Linux and Forgejo administrator and installs that administrator's Forgejo key;
later people follow the native flow above.

Workspace setup copies the person's current public authorized keys once. A
later primary-key change does not silently modify existing workspaces; update
those standard files explicitly where needed.

Soda never copies private keys, Tea configuration, GitHub CLI configuration, or
tokens into another account.

## Verify the result

Ask the person to:

1. Sign in to Cockpit with the primary Linux username and password.
2. Complete their first normal Forgejo login with the same Linux credentials.
3. If they need repository SSH access, add and confirm their key in Forgejo's
   native user settings.
4. Open **Projects** and create their first workspace.

## Expected result

The teammate has one ordinary primary Linux account and, after first login, one
corresponding non-administrator Forgejo account. Linux and Forgejo each retain
their own keys. The teammate has no workspace until they select **Set up for
me**.

## If adding a person fails

Inspect the failing native owner. Use Cockpit or Linux tools to check account,
password, group, home, or `authorized_keys` problems. If first Forgejo login
fails, confirm the Linux credentials and inspect Forgejo's PAM error before
retrying. Manage repository keys through Forgejo after login; there is no Soda
person-creation workflow to roll back or resume.

## Remove a person

Person removal is destructive and administrator-only. Read [Data safety and
removal](../40-Operate-Soda-OS/30-data-safety-and-removal.md) before using it.
The operation deletes the person's workspaces first, Forgejo account second,
and primary Linux account last.

## Next step

Read [Projects and workspaces](20-projects-and-workspaces.md).
