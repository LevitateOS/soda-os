# Current Soda OS architecture

The product contract and ownership rules live in
[principles.md](principles.md) and [architecture-reset.md](architecture-reset.md).
This document separates the approved target from the source currently present
at checkpoint `5cf31df`.

## Product contract

Fedora bootc owns the base operating system and image deployment. Linux owns
accounts, groups, homes, permissions, and processes. `wheel` owns administrator
status. OpenSSH owns remote sessions. Forgejo or an external Git host owns
repositories and collaboration. Stock Cockpit owns browser administration.
Tailscale owns private cloud reachability. `mise` owns development tools and
versions.

Soda owns only:

- the branded image, network ISO, and reusable QCOW2 composition;
- the current bounded Soda Setup workaround shared by the console and Cockpit;
- the shared project catalog;
- one derived Linux workspace account per selected person-project pair;
- branding and one focused Cockpit Projects page;
- fixed synchronous operations for setup and destructive local lifecycle.

### Installation and access

The intended architecture is one complete installation journey with no
separate Soda-owned post-install setup. It does not require moving every setup
screen into a renamed custom installer; the final ownership split remains to
be proven against Fedora's native installation boundaries.

ISO installation uses stock graphical Anaconda for storage, networking, bootc
deployment, Linux user creation, and administrator selection. Root stays locked.
Reboot and log in normally before Soda Setup appears. ISO installation creates
`/etc/cloud/cloud-init.disabled`, preventing cloud-init from altering those accounts.

QCOW2 deployments use standard Fedora cloud-init delivered by VM tooling. Supply
the Linux account, personal SSH public key, optional password hash, and optional
network configuration through user-data. No Soda checkout or manually built
credential ISO is required. A key alone enables SSH authentication; it does not
supply a password for console, Cockpit, PAM, or password-based sudo.

Soda Setup is temporary and handles only missing network trust or Tailscale
enrollment. It starts after an authenticated administrator logs in on an
interactive machine console and uses ordinary privilege elevation. Cancellation
or failure returns to the logged-in shell. SSH, SCP, and SFTP do not launch it.
Native network readiness skips automatic Setup. Administrators can reopen it
with `sudo /usr/libexec/soda/soda-setup console` or through Cockpit.

Default-drop protection remains in place. Explicitly trust only a trusted LAN
connection; cloud services remain private through Tailscale. Tailscale never
disables the existing trusted-LAN path.

After successful cloud-init Tailscale enrollment and verification, invoke
`/usr/libexec/soda/forgejo-init refresh-tailnet`. Interactive Setup uses this same
entry point. It compares DOMAIN, SSH_DOMAIN, and ROOT_URL against the native
Tailnet identity. Matching values cause no writes or restart. Stale values cause
the existing Forgejo service restart; its inactive oneshot initializer applies
the address before the replacement process starts. Forgejo never waits for
cloud-final. Report enrollment success separately from refresh failure.

Managed services are directly reachable on a trusted LAN. Cloud deployments
use Tailscale and never expose SSH, Cockpit, or Forgejo to the public Internet.
Projects list and setup operations are independent of Tailscale enrollment.
The Cockpit page derives SSH guidance from the hostname used to open Cockpit;
the Projects protocol does not select a LAN or Tailnet host.

### Accounts, Forgejo, and workspaces

The owner registers the first Forgejo account through the normal trusted LAN
or Tailnet before teammates sign in. Native first-user signup grants Forgejo
administration. Use independent Forgejo credentials, even with the same username
as the Linux owner. PAM remains active. Later Linux users' first successful PAM
login creates ordinary Forgejo accounts. Linux wheel membership grants no
Forgejo role. The team controls ongoing registration policy; there is no
mandatory registration-closing step or associated restart.

Workspace setup copies only current public authorized keys. Tea and gh are
present in every workspace and authenticated manually and separately there.
Soda copies no private key, CLI configuration, or token.

Each workspace owns a private home and complete clone below `$HOME/Projects`.
Its UID owns its files, dependencies, caches, processes, and local state.
The workspace's outbound Git key remains private there. If the authoritative
Git host has not authorized its public key, Projects reports the key and the
person registers it natively before retrying setup. Projects accepts no Forgejo
password and registers no workspace key.

### Catalog and deletion

Every primary human can view and edit the shared project catalog. The catalog
has no approved closed metadata field list and stores no membership,
credentials, workspace state, processes, ports, containers, or jobs.
Repositories are created in Forgejo or the external authoritative Git host and
then added to the catalog with their SSH clone URL; Projects creates none. The
project ID and canonical URL are immutable after addition. Display information
and additional metadata remain editable. Replacing the URL requires an
administrator to remove the project and all local workspaces and then re-add
it; repository-host data remains untouched.

The project view reports `workspace_exists` from the derived Linux account's
existence. A retained account after failed Git authorization therefore remains
visible and removable even though its clone is incomplete; setup remains the
explicit retry that completes the clone.

A person removes only their own workspace. An administrator may remove an
entire project, permanently deleting the shared entry and all local workspaces,
including uncommitted work, while preserving the canonical Forgejo repository.

Person deletion removes local workspaces and then the primary Linux account.
Forgejo account deletion remains separate inside Forgejo. Both destructive operations stop at the first failure, report
the partial result, and allow explicit retry without rollback or hidden state.

### Development tools and lifecycle

People invoke and configure `mise` directly inside their workspaces. Native
project configuration is shared through the repository, and upstream tools own
their cache behavior. Installed dependencies remain workspace-private. Projects
has no tool selections, installer action, shared tool storage, status, retry, or
cleanup lifecycle. Soda owns no tool downloader, cache, package manager, profile
system, or version state.

Administrators use native bootc operations for manual update and supported
fallback. Automatic updates remain disabled. Soda has no updater or recovery
engine.

## Current implementation

The source already uses stock Cockpit, direct OpenSSH workspace accounts,
native Git/Forgejo boundaries, native bootc operations, and no general Soda
runtime daemon, API, database, or control socket.

The following current mechanisms conflict with the approved contract and are
implementation debt:

| Current source | Approved replacement |
| --- | --- |
| Mandatory OEMDRV and installer-time administrator/Forgejo provisioning | One ISO followed by the current Soda Setup workaround |
| Soda-owned cloud provisioning and finalizer | Standard Fedora cloud-init through VM tooling |
| Tailnet-only managed-service firewall | Direct trusted-LAN access plus cloud Tailscale access |
| Exact three-field catalog | No closed metadata field list |
| Soda-created Tea PAT/config and workspace copying | Manual Tea and gh login in each workspace |
| Custom `soda-bun` and broad immutable tool manifest | `mise`-owned tool installation and versions |
| Coordinated Linux/Forgejo deletion | Local workspaces then Linux; independent native Forgejo deletion |
| Release CI rebuilds fallback A and runs VM acceptance | Prior signed A digest plus signed pre-release evidence |

Current package, path, group, account-marker, polkit, staging, and process
commands remain implementation choices. They must be re-evaluated while their
owning issues replace the conflicting behavior.

## Boundaries that remain correct

- Stock Cockpit remains the only browser administration owner.
- Direct OpenSSH uses real workspace accounts and homes.
- Forgejo and Git hosts own repositories and collaboration.
- The canonical Forgejo repository survives Soda project removal.
- Native bootc owns update and fallback.
- AArch64 and x86-64 remain equal matching-native targets.
- No general Soda control plane may return.

## Evidence status

Historical native x86-64 and AArch64 runs prove the implementation that existed
at their exact commits. They do not prove the newly approved Soda Setup, LAN,
Forgejo-key, manual CLI-authentication, mise, person-deletion, or build-once
release paths.

No public download, finished release, or release-day validation result is
claimed here. Matching-native product evidence must be regenerated after the
replacement implementation is complete.
