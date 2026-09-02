A Soda project is an appliance-wide invitation to set up a canonical Git
repository. It is not a copy of repository permissions, membership, or runtime
status.

## Product contract

The project catalog stores exactly three fields:

- immutable `id`;
- mutable `display_name`; and
- mutable, credential-free `canonical_url`.

Every primary human can discover, add, edit, or remove catalog entries. A
project can begin with an existing Git repository URL or as a native empty
repository in the initiating person's bundled Forgejo namespace. Soda creates
no README, artificial first commit, or initial branch merely to make an empty
repository appear populated.

Selecting **Set up for me** leaves a complete clone in that person's derived
workspace account. Public and SSH remotes need no password prompt from Soda.
HTTP credentials may be supplied for the single clone operation, but Soda does
not retain them. Afterward, each developer manages ordinary Git authentication
inside their own workflow.

Forgejo or the external Git provider owns repository access, collaborators,
branches, reviews, issues, releases, and deletion. Removing a project from Soda
permanently deletes its local workspace accounts, homes, clones, dependencies,
and uncommitted work, but never deletes the canonical repository.

## Current implementation

The stock Cockpit Projects page exposes **Add repository**, **New Forgejo
project**, **Edit**, **Set up for me**, and **Remove** actions. It shows the
canonical URL and the direct SSH command for each person's generated workspace.

Catalog changes are serialized and atomically replace a JSON file containing
only the three accepted fields. Setup clones as the unprivileged primary user,
uses anonymous sealed memory only when transient HTTP credentials are supplied,
and invokes a narrowly authorized helper only to publish the completed tree and
perform validated Linux mutations.

The current bundled Forgejo flow can create a truly empty repository as the
initiating user. The installer-created first administrator can authenticate to
that flow. Later primary users' intended native Forgejo PAM login remains
disabled pending the password-verifier privilege decision. External Git hosts,
public repositories, and user-managed SSH authentication remain ordinary Git
paths rather than provider integrations.

Focused tests cover exact catalog persistence, credential boundaries,
repository ownership, setup, edits that affect only future workspaces, and
catalog-last removal. Complete installed multi-user destructive acceptance is
still pending.
