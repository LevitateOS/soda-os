The Soda **Projects** page is the shared menu of repositories that developers
can turn into personal workspaces. It makes projects discoverable on the
machine without taking ownership away from Git or the repository host.

Owners and developers encounter this page in **Cockpit**, Fedora's browser
administration interface, when they add a repository, create one in
**Forgejo**, the bundled Git hosting and collaboration service, or select
**Set up for me**. They also use it for destructive local project removal.

## Product contract

### The project catalog is an invitation, not a permission system

The **project catalog** is one appliance-wide list visible to every primary
account—the ordinary Linux identity for one human. Each entry contains
exactly:

- an immutable `id`, used as the stable local identity;
- a mutable `display_name`, shown to people; and
- a mutable, credential-free `canonical_url`, identifying the authoritative
  Git repository.

The canonical URL may contain a transport username such as `git@host`, but it
must not contain a password or access token.

The catalog does not record who created a project, who may access its
repository, which branches exist, whether a clone is current, or what is
running. Forgejo or the external Git provider remains authoritative for
repository access, collaborators, branches, reviews, issues, releases, and
repository deletion.

Every primary user may add, edit, or remove a catalog entry. Soda adds no
owner-approval or project-membership workflow, so the trusted team is
responsible for coordinating those actions.

### Add an existing repository

Use **Add repository** when the canonical repository already exists in Forgejo
or another Git host. Supply a stable project ID, a display name, and the
credential-free clone URL.

Adding the entry only makes the project discoverable on this Soda machine. It
does not clone anything yet, grant repository access, or change the Git host.
Each developer separately selects **Set up for me**.

### Start a project in bundled Forgejo

Use **New Forgejo project** when a repository does not exist yet.

The intended outcome is a native empty repository in the initiating person's
Forgejo namespace and a corresponding catalog entry. Soda does not add a
README, create an artificial first commit, or invent a branch to make the
repository appear populated.

Forgejo owns that repository from then on. Repository settings, access, keys,
collaboration, and eventual deletion happen in Forgejo, not in the Soda
catalog.

### Set up a personal workspace

Before setup, make sure the primary account has a valid SSH public key in
`~/.ssh/authorized_keys`. Then select **Set up for me** beside the project.

Soda performs the Git operation as the signed-in primary user. Public and SSH
remotes use their ordinary Git authentication paths. When an HTTP remote needs
a username and password or token for this one clone, the operation may accept
them without retaining them.

Success means all of the following are ready before the action returns:

- a derived workspace Linux account for this person and project;
- the primary account's current public keys copied once; and
- a complete clone owned by the workspace below
  `$HOME/Projects/<repository>`.

The Projects page then shows the workspace username and direct SSH command.
After setup, repository authentication and Git work are ordinary developer
choices inside that workspace.

### Edit without rewriting existing workspaces

Use **Edit** to change the display name or canonical URL. The project ID stays
the same. An edit affects future setup only: Soda does not rename existing
accounts, move existing clones, or rewrite their Git remotes.

### Remove a project from Soda

**Remove** is destructive. It permanently deletes every local workspace
account for that project, including homes, complete clones, dependencies,
project-local data, and uncommitted or unpushed work. The catalog entry is
removed only after the local workspace deletions succeed.

Removal never deletes the canonical Forgejo or external repository and does
not archive or transfer local work. The team must preserve anything valuable
before confirming the action. There is no Soda approval, rollback, or recovery
workflow for the deleted local data.

## Current implementation

The current Cockpit page exposes these labels: **Add repository**, **New
Forgejo project**, **Edit**, **Set up for me**, and **Remove**. Destructive
removal requires the user to type the project ID exactly and explicitly warns
that the canonical repository will remain.

For an existing repository, the UI accepts HTTP, HTTPS, SSH, and SCP-style Git
remotes after rejecting embedded credentials and local-file paths. For setup,
the optional **Git username** and **Git password or token** fields are used only
for the synchronous clone request and cleared from the page afterward. Focused
tests verify that supplied HTTP credentials do not enter the privileged
workspace operation or Git arguments, environment values, and stored remotes.

The bundled Forgejo path currently asks for the signed-in user's **Forgejo
password**, creates a truly empty repository as that user, and adds the clone
URL to the catalog. The installer-created first Forgejo administrator can use
this path. Later-created primary users cannot currently sign in through the
intended Forgejo PAM source, so they cannot yet use the same flow with their
Linux credentials.

Code-level verification covers exact three-field catalog storage, edits that
affect only future setup, one-user workspace creation, credential boundaries,
native empty repository ownership, setup-versus-removal coordination, and
catalog-last deletion. Final installed multi-user destructive and failure
coverage remains incomplete, and the complete installed path still needs
matching-native AArch64 verification.
