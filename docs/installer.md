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

### Network installer

The public human journey uses one finished ISO:

1. Boot the architecture-matched ISO.
2. Use stock graphical Anaconda for storage, networking, firmware, bootloader,
   and bootc deployment.
3. Reboot into the installed system.
4. Complete Soda Setup after the installed system boots.

The ISO installs an exact architecture-specific GHCR digest and does not embed
the Soda runtime payload. A network or registry failure is a native installation
failure. Soda adds no mirror, embedded fallback image, alternate installer, or
download service.

The human supplies no Kickstart, OEMDRV, second credential disk, repository
checkout, Go command, xorriso command, or other provisioning medium.

### Reusable QCOW2

The reusable QCOW2 is built from the same exact OCI image as the installer.
After boot it uses the same Soda Setup state and operations as an ISO-installed
system. Soda Setup appears on the VM or supported cloud console and is also
available in Cockpit after network access exists.

Supported cloud platforms must provide console access. Soda does not consume
NoCloud or ConfigDrive user data, merge cloud metadata, accept a separate
`cloud-input`, install provider agents, or expose a public SSH bootstrap.
Until Soda Setup is complete, the machine remains unconfigured.

The image grows its final partition and filesystem to the supplied virtual
disk. The raw QCOW2 is a matching-native build artifact; the compressed image,
checksum, release record, and signing evidence are release outputs.

### Current Soda Setup workaround

Soda Setup is machine-wide and shared by ISO and QCOW2. It is available on a
physical, VM, or supported cloud console and through Cockpit via the same
bounded underlying operations. Both surfaces use the same state, and Soda
Setup can always be opened again later.

It remains available at startup until explicitly dismissed. Dismissal is not
available until:

- one primary Linux administrator exists;
- its password is set;
- its public SSH key is installed;
- the same-named Forgejo site administrator is ready; and
- Tailscale is connected or **Allow access from the local network** is
  explicitly selected for the current connection.

When the owner enters a Tailscale authentication key, Soda Setup passes it once
to Tailscale and removes it. Trusting the current local connection requires no
key. Tailscale remains a separate choice, can be configured later, and never
disables trusted local-network access.

The setup leaves no password, Tailscale key, bootstrap account, enabled
provisioning path, runtime status database, retry record, or background setup
service. A failure remains visible and is retried explicitly through the same
setup; Soda adds no recovery engine or reconciliation loop.

Automatic LAN/cloud detection and choosing setup enablement inside Anaconda are
not part of the current release contract. The long-term single-journey goal
does not select either mechanism.

## Installed ownership

After Soda Setup:

- Linux owns the administrator, password, `wheel` membership, home, and
  standard authorized key;
- Forgejo owns the same-named site administrator and registered SSH public key;
- Tailscale owns its node identity when enrolled;
- stock Cockpit owns browser administration; and
- Soda owns only completion of the current bounded Soda Setup composition.

Later primary humans are created through the supported administrator workflow,
receive a corresponding Forgejo account, and have their public key registered
there. Workspace accounts are Linux-only identities.

Tea and gh are available in workspaces but are never configured during
installation or Soda Setup. Each workspace login is manual and separate.

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
Project and workspace creation may offer multiple convenience selections, but
the list is open. Later installation targets either one workspace or the
project's shared tool scope.

Shared project tools are stored once and use upstream-native shared download
caches. Soda does not own cache format, downloads, version resolution, or
toolchain state. Installed dependencies and other mutable development state
remain workspace-private. Coding assistants are selected and authenticated per
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
