The Soda **Projects** page is the shared menu of repositories that developers
can turn into independent workspaces. It makes repositories discoverable on
the machine without taking ownership away from Git or the canonical repository
host.

Every primary user can add, edit, set up, and remove catalog entries. This is a
trusted-team workflow: coordinate shared catalog changes with the other people
using the machine.

## Understand the Project catalog

Each entry contains exactly three fields:

- **Project ID** is the immutable, stable local identity. It is a short
  lowercase value used for workspace association and paths.
- **Display name** is the human-readable name shown in Cockpit.
- **Canonical Git URL** is the credential-free clone URL of the authoritative
  repository.

The canonical URL may contain a transport username such as `git@host`, but it
must not contain a password or access token.

The catalog does not store repository membership, credentials, branches,
reviews, clone status, or running services. Forgejo or the external Git host
remains authoritative for those facts.

## Add an existing repository

Use this path when the repository already exists in Forgejo or another Git
host:

1. Open **Projects** in Cockpit.
2. Select **Add repository**.
3. Enter a stable **Project ID**.
4. Enter the **Display name**.
5. Enter the credential-free **Canonical Git URL**.
6. Select **Add repository**.

The new catalog entry becomes visible to every primary user. Adding it does not
clone the repository, grant Git access, or change anything on the canonical
host. Each developer performs their own workspace setup.

## Create a new Forgejo project

Use this path when the repository should begin in the bundled Forgejo service:

1. Open **Projects**.
2. Select **New Forgejo project**.
3. Enter **Project ID** and **Display name**.
4. Enter your **Forgejo password** for this request.
5. Select **Create project**.

Soda creates a native empty repository in the initiating person's Forgejo
namespace and adds its credential-free URL to the catalog. It does not add a
README, create an artificial first commit, or invent a branch.

Forgejo owns the repository from then on. Use Forgejo for collaborators,
permissions, branches, reviews, issues, releases, and repository deletion.

## Prepare for workspace setup

Before selecting **Set up for me**:

- make sure the primary account has at least one valid key in standard
  `~/.ssh/authorized_keys`;
- confirm that the canonical URL is correct; and
- confirm that you can authenticate to the repository when it is private.

If the primary account has no valid SSH key, setup stops before creating an
account or clone.

The copied SSH key is for inbound access to the new workspace. Outbound Git
authentication is separate. Depending on the repository URL and provider, use
an SSH agent, an existing private Tea login, or a username and password or
token supplied for the setup request.

## Set up your workspace

1. Find the repository on **Projects**.
2. Select **Set up for me**.
3. Leave **Git username** and **Git password or token** empty for a public
   repository or when ordinary SSH authentication is already available.
4. For an HTTPS repository that needs credentials, enter them for this request.
5. Select **Set up for me**.

Any entered Git credential is passed only to the unprivileged Git operation
and is not retained.

A successful setup leaves:

- one derived Linux workspace account for the primary user and project;
- a private workspace home;
- a one-time copy of the primary account's authorized SSH public keys;
- a one-time copy of that person's private Tea configuration; and
- a complete clone beneath `$HOME/Projects/<repository>`.

The clone must complete before the workspace becomes the accepted result. The
workspace account owns its files and processes and can be reached directly
through OpenSSH.

## Edit a catalog entry

Select **Edit**, change **Display name** or **Canonical Git URL**, and select
**Save changes**. The Project ID remains unchanged.

Edits affect future workspace setup. They do not rewrite or synchronize clones
that already exist. Developers update an existing clone with ordinary Git or
replace it through a deliberate local workflow.

## Remove a project

**Remove project** is destructive. It permanently deletes every local
workspace account and home associated with that Project ID, including clones,
dependencies, caches, project-local data, uncommitted changes, and commits that
were never pushed. The catalog entry is removed last.

The canonical Forgejo or external repository is preserved. Push important
commits and separately export non-Git data before confirming removal.

Read [Data safety and removal](data-safety-and-removal.md) before using the
action. For daily access to the resulting workspace, continue with
[Connect and develop](connect-and-develop.md).
