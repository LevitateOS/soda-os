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
- one common console/Cockpit first-boot setup;
- the shared project catalog;
- one derived Linux workspace account per selected person-project pair;
- branding and one focused Cockpit Projects page;
- Forgejo public-key registration;
- `mise`-backed workspace and shared-project tool scope; and
- fixed synchronous operations for setup and destructive local lifecycle.

### Installation and access

One finished network ISO boots stock graphical Anaconda. Anaconda handles
storage, networking, bootloader, firmware, and bootc deployment. The installed
system then presents the common interactive first-boot setup. The same setup is
used by QCOW2 and can be reopened in Cockpit.

First boot cannot be dismissed until an administrator, password, SSH public
key, Forgejo administrator, and either Tailscale connection or explicit
LAN-only choice exist. A Tailscale key is used once and removed. There is no
human OEMDRV, cloud-init provisioning path, second input image, or public-SSH
bootstrap.

Managed services are directly reachable on a trusted LAN. Cloud deployments
use Tailscale and never expose SSH, Cockpit, or Forgejo to the public Internet.

### Accounts, Forgejo, and workspaces

Each person has one ordinary primary Linux account. Development happens only in
derived workspace accounts. Every supported person also has a corresponding
Forgejo account and registered public SSH key. Git uses SSH.

Workspace setup copies only current public authorized keys. Tea and gh are
present in every workspace and authenticated manually and separately there.
Soda copies no private key, CLI configuration, or token.

Each workspace owns a private home and complete clone below `$HOME/Projects`.
Its UID owns its files, dependencies, caches, processes, and local state.

### Catalog and deletion

Every primary human can view and edit the shared project catalog. The catalog
has no approved closed metadata field list and stores no membership,
credentials, workspace state, processes, ports, containers, or jobs.

A person removes only their own workspace. An administrator may remove an
entire project, permanently deleting the shared entry and all local workspaces,
including uncommitted work, while preserving the canonical Forgejo repository.

Person deletion removes workspaces, the Forgejo account, and then the primary
Linux account. Both destructive operations stop at the first failure, report
the partial result, and allow explicit retry without rollback or hidden state.

### Development tools and lifecycle

`mise` installs and selects tools for one workspace or for the project. Shared
project tools are stored once, using upstream-native shared download caches.
Installed dependencies remain workspace-private. Soda owns no tool downloader,
cache, package manager, profile system, or version state.

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
| Mandatory OEMDRV and installer-time administrator/Forgejo provisioning | One ISO followed by common interactive first boot |
| Separate NoCloud, ConfigDrive, and cloud finalizer | The same console first boot for QCOW2 |
| Tailnet-only managed-service firewall | Direct trusted-LAN access plus cloud Tailscale access |
| Exact three-field catalog | No closed metadata field list |
| Soda-created Tea PAT/config and workspace copying | Manual Tea and gh login in each workspace |
| Custom `soda-bun` and broad immutable tool manifest | `mise`-owned tool installation and versions |
| Human deletion preserves Forgejo user | Workspaces, then Forgejo account, then Linux account |
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
at their exact commits. They do not prove the newly approved first-boot, LAN,
Forgejo-key, manual CLI-authentication, mise, person-deletion, or build-once
release paths.

No public download, finished release, or release-day validation result is
claimed here. Matching-native product evidence must be regenerated after the
replacement implementation is complete.
