# Soda bootc runtime image and installer

`distro/soda.toml` is the schema-version-2 runtime image contract. It pins the
approved Fedora 44 bootc manifest digest, `linux/arm64` platform, Soda image
name, runtime state schema, package lock, Soda version, and source-date epoch.
The builder obtains the source revision from the current Git commit.

`distro/locks/runtime-packages.toml` records exact NEVRAs for every Fedora RPM added
to the pinned base and for the three locally built Soda RPM inputs. The Soda
RPMs are build inputs only; no mutable Soda RPM repository is created or
embedded. Weak dependencies are disabled.

During `just oci`, the Go builder:

1. validates the immutable base, platform, registry, state schema, and package
   lock;
2. reproducibly builds `soda-release`, `soda-runtime`, and `soda-cockpit` with
   the configured version, source revision, and source date;
3. installs the exact locked transaction into the pinned Fedora bootc base;
4. creates the fixed `soda-api` group and `soda-cockpit` service account;
5. enables SSH, Soda services, Avahi, and the persistent-state bind mounts;
6. masks the automatic bootc update timer while retaining manual bootc
   operations;
7. records the complete installed RPM inventory and verifies its SHA-256; and
8. exports an OCI archive without loading, pushing, signing, or publishing it.

OCI labels record the Soda version, Git revision, creation time, pinned base,
and runtime state schema. BuildKit rewrites image timestamps to the configured
source-date epoch and omits provenance attestations from this local artifact.

## Release identity and installer media

Soda has one trusted-LAN release repository:
`registry.soda.local/soda/os`. Runtime images are AArch64 (`linux/arm64`) OCI
images and are identified for installation and update by an exact digest, never
by a tag. The registry CA certificate and Soda Cosign public key are explicit
build inputs and are embedded in the runtime image. The release administrator
holds the sole passphrase-protected Cosign private key outside the repository
and image.

The initial-install happy path is deliberately two-step:

1. `soda-image publish --defer-current` pushes the versioned runtime image,
   resolves its canonical registry digest, signs and verifies that exact digest,
   and prints the reference for the installer. It writes no release record and
   leaves `current` unchanged.
2. `soda-image iso --image registry.soda.local/soda/os@sha256:...` verifies the
   signed payload, embeds that exact digest in an AArch64 `bootc-generic-iso`,
   uses ext4, and writes ISO checksum and payload-provenance sidecars. A final
   `soda-image publish --iso ...` independently checks the ISO, signs the
   release record, and updates `registry.soda.local/soda/os:current` last.

The installer is for fresh installation only. It uses stock interactive
Anaconda text mode with DHCP, a default hostname of `soda`, and the normal
interactive choices for storage, networking, hostname, and the first
administrator. Declaring text mode directly avoids Fedora 44 Anaconda's broken
graphical-fallback chooser when the pinned bootc installer environment has no
local graphical frontend. Soda does not preserve the former custom graphical
overlay and does not perform an in-place Rocky conversion. The Fedora 44
installer environment runs SELinux in permissive mode because its live overlay
cannot be relabeled; this boot option does not change the installed Soda image,
which retains enforcing SELinux.

Before Anaconda creates the interactive administrator, the installer creates
the persistent `/var/home` parent in the mounted target. Fedora bootc otherwise
creates that parent only through `tmpfiles.d` on first boot, which is too late
for Anaconda to create `/home/<administrator>` through the image-owned
`/home -> var/home` compatibility symlink.

The ISO embeds the exact signed image digest in local container storage. Fedora
44's `bootc` Kickstart command accepts exact source and target image references
but does not expose bootc's runtime signature-policy flag, so installer trust is
established before installation by the signed ISO-bound release record and ISO
checksum. Installed-system updates independently enforce the embedded public-key
signature policy.

## Persistent host state

Bootc owns the image base. Soda keeps mutable state under `/var/lib/soda` so it
survives replacement of the image: SQLite schema 2 state, Cockpit certificates,
projects, and toolchains. Image-owned mount units retain the existing visible
paths `/srv/soda/projects` and `/opt/soda/toolchains`; direct SSH workspaces and
the forced-command gateway therefore keep their established paths. `tmpfiles.d`
creates the persistent directories. Linux users and PAM passwords, SSH host
keys, `/etc/soda/authorized_keys`, and `/var/log/soda` are likewise host state.

## Manual OS updates

The runtime masks Fedora's automatic bootc update timer. An administrator uses
`sodactl os update status`, `check`, `stage`, and
`activate --confirm-reboot`. Checking resolves `current` once and verifies the
Cosign signature before inspecting immutable release metadata. It accepts only
the Soda repository, `linux/arm64`, and state schema 2. Staging calls bootc with
the exact `@sha256:` reference and download-only signature enforcement, so it
does not restart Soda services or change the running deployment. Activation
uses the already-downloaded deployment and requires the explicit reboot
confirmation. There is no background polling, download, activation, or reboot.

The signed release record binds Soda version and source revision, the Fedora
base reference, exact Soda image reference, platform, state schema, RPM
inventory checksum, and the installer ISO checksum when an ISO is produced.
The state schema remains 2 for this MVP; cross-schema state rollback is not
supported.
