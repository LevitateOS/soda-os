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
Native network readiness permits explicit dismissal but does not suppress
automatic Setup. It keeps appearing after an administrator console login until
they choose **Don't show Setup automatically**. Administrators can reopen it
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

## Installed ownership

The owner registers the first Forgejo account through the normal trusted LAN
or Tailnet before teammates sign in. Native first-user signup grants Forgejo
administration. Use independent Forgejo credentials, even with the same username
as the Linux owner. PAM remains active. Later Linux users' first successful PAM
login creates ordinary Forgejo accounts. Linux wheel membership grants no
Forgejo role. The team controls ongoing registration policy; there is no
mandatory registration-closing step or associated restart.

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
bootc operations with exact signed Soda image references. Supported fallback
uses the same native deployment path with the previous signed OCI digest and
preserves current machine state. Direct `bootc rollback` is unsupported unless
it is separately proved to preserve current `/etc` and `/var` state.

Soda ships no release-discovery client, update daemon, translated deployment
state, wrapper CLI, retry process, or recovery service.

## Release records

A strict release record binds the product version, source revision,
architecture, Fedora base, exact GHCR image digest, RPM inventory, ISO checksum,
raw QCOW2 checksum, and compressed QCOW2 checksum. Signatures and provenance
bind the record and OCI digest to the release workflow.

Release CI builds each architecture's new image once and derives its ISO and
QCOW2 from that exact output. It structurally checks and publishes those bytes
unchanged. The previous fallback image is downloaded by its earlier signed
published digest rather than rebuilt.

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
artifacts, not the current Soda Setup journey.

No new Soda Setup artifact, public ISO, public QCOW2, or final release is
claimed by this document.
