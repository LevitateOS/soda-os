# Soda bootc runtime image and installer

The product contract is governed by [architecture-reset.md](architecture-reset.md).
This document records the target artifact boundary and the source currently
being replaced.

## Product contract

Soda produces equal architecture-specific outputs for x86-64 and AArch64:

- a bootc OCI image stored in GHCR;
- one network installer ISO; and
- one compressed reusable QCOW2 image.

Every architecture-specific input, build, inspection, installation, and
acceptance claim is produced on matching-native hardware.

The intended permanent architecture is one complete installation journey with
no separate Soda-owned post-install setup. The current release retains **Soda
Setup** as a temporary workaround after the installed system boots. This
document preserves that complete current journey until a proven replacement
exists; it does not prescribe moving every setup screen into a renamed custom
installer.

ISO installation uses stock graphical Anaconda for storage, networking, bootc
deployment, Linux user creation, and administrator selection. Root stays locked.
Reboot and log in normally; the welcome message shows connection details. ISO installation creates
`/etc/cloud/cloud-init.disabled`, preventing cloud-init from altering those accounts.

QCOW2 deployments use standard Fedora cloud-init delivered by VM tooling. Supply
the Linux account, personal SSH public key, optional password hash, and optional
network configuration through user-data. No Soda checkout or manually built
credential ISO is required. A key alone enables SSH authentication; it does not
supply a password for console, Cockpit, PAM, or password-based sudo.

Interactive local and SSH shells always show a concise welcome with the native
hostname, local Cockpit and Forgejo URLs, a current-user SSH command, and current
Tailscale status. It has no completion or dismissal state. Administrators can
customize the native `/etc/profile.d/soda-console-welcome.sh` entry point.
Non-interactive commands, SCP, and SFTP keep their ordinary output.

Soda preserves Anaconda/Fedora firewall defaults, with firewalld enabled and
TCP 9090 allowed for Cockpit. Administrators must allow Forgejo TCP 30000/2222
and selected development ports for LAN access through stock Cockpit's
**Networking → Firewall** page. Soda supplies no default-drop override, custom
zone, or connection-selection trust workflow. Enrolling Tailscale does not
change LAN access or an administrator's firewall choices.

Tailscale is preinstalled and tailscaled is enabled, initially unenrolled.
Administrators sign in through native browser authentication on the separate
**Cockpit → Tailscale** page. The page shows connection state, this device's
name and addresses, visible peers, eligible exit nodes, the native LAN-access
setting for exit-node use, exit-node advertisement and approval, and a link to
the official CLI documentation. It stores no authentication key or workflow state.

The page reads native state on opening and while active. Authentication URLs
are shown as soon as the native process emits them, before authentication
completes. Native status owns pending authentication and approval across page
loads. Closing the page closes its processes; it does not log out the machine.

When the page observes a connected machine, it invokes the existing
`/usr/libexec/soda/forgejo-init refresh-tailnet` command. That command compares
DOMAIN, SSH_DOMAIN, and ROOT_URL against the native reachable Tailnet identity.
Matching values cause no writes or restart. Stale values cause the existing
Forgejo service restart; its inactive oneshot initializer applies the address
before the replacement process starts. The native initializer also runs when
Forgejo starts. Enrollment success and refresh failure are reported separately.
There is no watcher or durable recovery state.

## Installed ownership

Every primary human uses ordinary Forgejo PAM login, without a first-owner
signup or administrator prerequisite. New configurations disable browser
registration. Forgejo administration is explicit and independent of Linux roles;
see the [native CLI creation and web promotion procedure](public/40-Operate-Soda-OS/10-administration.md#create-a-forgejo-administrator).
Existing roles and operator configuration are preserved.

## Persistent host state

Bootc owns the replaceable image. Linux owns accounts, groups, passwords,
homes, authorized keys, and SSH host keys. Forgejo owns its users, repositories,
and database. Tailscale owns its enrolled node state. Soda owns the shared
project catalog and only the irreducible workspace association.

The project catalog has no approved closed metadata field list. Workspace
clones and installed dependencies live under their derived Linux homes.

Soda creates no runtime person database, project-membership database,
repository projection, credential store, toolchain database, control socket,
daemon, API, bootstrap state, or updater state.

## Development tools

`mise` owns development-tool installation, versions, and project configuration.
People invoke and configure it directly inside their workspaces. Project
configuration is shared through the project's native repository workflow, and
upstream tools own their cache behavior. Projects exposes no tool selector,
installer action, shared tool storage, status, retry, or cleanup lifecycle.
Soda does not own cache format, downloads, version resolution, or toolchain
state. Installed dependencies and other mutable development state remain
workspace-private. Coding assistants are selected and authenticated per
workspace.

Tea and GitHub CLI remain normal workspace commands with manual, separate
authentication. Soda copies no CLI configuration or credential.

## Manual image lifecycle

The runtime disables automatic bootc updates. A Linux administrator uses native
bootc operations or the [Soda Updates page](cockpit-updates.md). Check fetches
metadata; Update and restart follows the configured source and requires native
booted-digest readback afterward. Existing exact-digest installations need an
explicit native switch before tracking a development tag. Installer exact-digest
binding is unchanged. Fallback selects an earlier exact OCI digest and must
preserve current machine state. Direct `bootc rollback` is unsupported unless
it is separately proved to preserve current `/etc` and `/var` state.

The narrow synchronous helper adds no release-discovery client, update daemon,
deployment database, retry process, or recovery service. Runtime updates require
no signature or release record. `just dev-image <architecture>` builds/publishes
one native OCI independently of installer construction.

## Direct artifact identity

`internal/build/oci` is the shared inspection owner for publication, installers
and acceptance. It checks archive paths/platforms, blob integrity, version/source/
base labels and the installed RPM inventory/sidecar. It returns in-memory OCI
facts, not a private release record. An earlier fallback image supplies its own
version and base, not the current specification's values.

ISO and raw/compressed QCOW2 construction remain independent commands against
the exact OCI digest. Native ISO squashfs/initramfs/configuration/branding checks
remain. ISO, raw QCOW2 and compressed QCOW2 have ordinary checksum sidecars;
checksums detect byte changes, not provenance. Acceptance consumes those actual
paths and verifies the installed booted digest for both deployment paths, while
retaining the native B → earlier A → B preservation journey. No combined signed
record, release workflow or sibling qualification is required for publication.

## Current implementation

At checkpoint `5cf31df`, OCI, network-ISO, QCOW2, release-record, stock-Cockpit,
workspace, and native bootc construction paths exist. The current source still
uses:

- mandatory protected OEMDRV media and fixed installer hooks;
- installer-time Linux, Forgejo, Tea, and Tailscale provisioning;
- separate NoCloud and ConfigDrive cloud-init inputs and a cloud finalizer;
- a Soda-created Tea token copied into workspaces;
- an exact three-field catalog;
- a custom `soda-bun` package and broad immutable tool manifest; and
- Tailnet-only service ingress.

Those mechanisms describe the present source only. They are superseded by the
product contract above and must be deleted with their replacement slices. The
old matching-native installation and QCOW2 results prove those historical
artifacts, not the current installation and connection behavior.

No new installed-system evidence, public ISO, public QCOW2, or final release is
claimed by this document.
