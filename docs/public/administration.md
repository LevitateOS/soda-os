Soda OS keeps general machine administration where Linux administrators expect
it: in stock **Cockpit**, Fedora's browser administration interface, and
ordinary Linux tools. Soda adds one focused **Projects** page only for the
machine-wide project catalog and workspace lifecycle.

Owners encounter Cockpit when operating the machine. Developers use the same
browser sign-in to discover projects, but only Linux administrators see the
supported action for removing another person from Soda OS.

## Product contract

### Cockpit is the administration interface

Cockpit provides the machine overview, metrics, services, logs, accounts,
terminal, storage, and networking pages. It also owns browser authentication,
sessions, TLS, and normal privilege elevation.

Every primary Linux account—the ordinary account representing one human—can
sign in to Cockpit with its Linux username and password. Workspace accounts
are the separate person-project development identities; they cannot sign in to
Cockpit, and developers reach them directly through SSH.

Linux `wheel` membership determines who is an administrator. Soda keeps no
separate administrator list. Use stock Cockpit or ordinary Linux tools for
normal primary-account creation, password changes, group membership, and
machine administration.

### Projects is the Soda-specific page

The **Projects** page adds only the actions needed for Soda's project workflow:

- see the appliance-wide project catalog;
- use **Add repository** or **New Forgejo project**;
- use **Edit** or **Set up for me**;
- use **Remove** for destructive project removal; and
- for administrators, use **Remove person…** for supported cascading human
  deletion.

The page reuses the current Cockpit identity and session. It is not a second
dashboard, account system, or general privileged command interface.

### Forgejo administration remains separate

**Forgejo** is the bundled Git hosting and collaboration service. Forgejo
manages its own users, administrator roles, repositories, permissions, keys,
tokens, sessions, issues, reviews, and releases.

The first installation creates same-named Linux and Forgejo administrators,
but they become independent accounts immediately afterward. Changing a Linux
password or `wheel` membership does not change a Forgejo role. Disabling or
deleting a Linux account does not claim to revoke existing Forgejo sessions,
tokens, keys, or repository access. Administrators handle those concerns in
Forgejo itself.

### Understand the two destructive paths

Any primary user may choose **Remove** for a project. The confirmation names
the project and permanently deletes all of its Soda-managed local workspace
accounts, homes, clones, dependencies, project-local data, and uncommitted or
unpushed work. The canonical repository is never deleted, and the catalog
entry remains if local deletion fails.

Only an administrator may choose **Remove person…**. The supported Soda-aware
action permanently deletes that person's local workspace accounts and homes,
then deletes the primary Linux account and home last. It does not delete the
person's Forgejo account or any Forgejo or external repository.

These actions do not provide archive, undo, rollback, or data recovery. Preserve
or push anything valuable before confirming them.

Do not confuse **Remove person…** with generic account deletion in Cockpit or
`userdel`. Generic Linux deletion affects only the account explicitly selected;
it does not cascade to Soda workspaces. Soda does not watch for that out-of-band
change or repair the resulting state.

Read [Updates and recovery](updates-and-recovery.md) for operating-system image
changes and [Projects and Git](projects-and-git.md) for project removal details.

## Current implementation

The current image includes Fedora's stock Cockpit host pages, Soda branding,
and the static Projects page. Root-required catalog and Linux changes pass
through one narrowly authorized synchronous operation. It cannot run arbitrary
commands or act as a general account, Forgejo, container, or filesystem API,
and it retains no credential or background job state.

The UI shows **Remove person…** only to a signed-in primary account whose
Linux account is in `wheel`. Project removal requires the exact project ID;
human removal requires the primary username to be entered twice. Both dialogs
state that local data is permanently removed and that Forgejo or the canonical
repository is left unchanged.

Code-level verification covers administrator authorization, validation before
deletion, process and account removal, catalog-last project deletion,
primary-last human deletion, and failure behavior. The complete multi-user and
destructive-ordering scenarios have also passed on an installed native x86-64
system; matching-native AArch64 repetition remains pending.

One native x86-64 installation has exercised stock Cockpit authentication,
its Fedora-owned administration pages, Projects discovery and setup, and
workspace-account rejection. Matching-native AArch64 installed-system evidence
for the same current path is pending.

Later-created primary users can sign in to Cockpit, but the intended Forgejo
PAM login is currently disabled pending a privilege decision. Administrators
must not treat Linux account creation as current proof that the new user can
also sign in to Forgejo.

Soda has no runtime administration daemon, general control CLI, or health API.
Use stock Cockpit, `systemctl`, `journalctl`, and ordinary Linux tools for host
inspection.
