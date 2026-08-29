# Soda bootc runtime image and installer

`distro/soda.toml` is the shared schema-version-2 runtime image contract.
`distro/platforms/aarch64.toml` and `distro/platforms/x86_64.toml` are equal
sibling platform contracts. Each pins its Fedora 44 bootc manifest, OCI
platform, package and installer locks, tool inputs, artifact identity, and
release channel. The shared specification owns the Soda image name, runtime
state schema, version, paths, and source-date epoch. The builder obtains the
source revision from the current Git commit, and every command requires an
explicit `--architecture aarch64` or `--architecture x86_64` selection.

The platform-selected runtime package lock records exact NEVRAs for every
Fedora RPM added to its pinned base and for the three locally built Soda RPM
inputs. The Soda RPMs are build inputs only; no mutable Soda RPM repository is
created or embedded. Weak dependencies are disabled.

During `just oci ARCH ...`, the Go builder:

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
`registry.soda.local/soda/os`. It carries equal AArch64 (`linux/arm64`) and
x86-64 (`linux/amd64`) images as separate single-platform releases. Installation
and update use an exact digest, never a mutable tag. Discovery is deliberately
architecture-specific through `current-aarch64` and `current-x86_64`; no
multi-platform index is used. Release records and artifacts likewise include
`aarch64` or `x86_64` in their names so sibling releases cannot overwrite one
another. The registry CA certificate and Soda Cosign public key are explicit
build inputs embedded in both runtime images. The release administrator holds
the sole passphrase-protected Cosign private key outside the repository and
images.

The initial-install happy path is deliberately two-step:

1. `soda-image publish --defer-current` pushes the versioned runtime image,
   resolves its canonical registry digest, signs and verifies that exact digest,
   and prints the reference for the installer. It writes no release record and
   leaves the selected architecture's discovery tag unchanged.
2. `soda-image --architecture ARCH iso --image
   registry.soda.local/soda/os@sha256:...` verifies the signed payload and
   platform match, embeds that exact digest in the matching
   `bootc-generic-iso`, uses ext4, and writes the ISO checksum. A final
   architecture-selected `soda-image publish --iso ...` independently checks
   the ISO, signs the architecture-named release record, and updates only the
   matching `current-ARCH` discovery tag last.

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
`activate --confirm-reboot`. Checking resolves the installed sibling's
architecture-specific `current` tag once and verifies the Cosign signature
before inspecting immutable release metadata. It accepts only the Soda
repository, the installed platform, and state schema 2. Staging calls bootc with
the exact `@sha256:` reference and download-only signature enforcement, so it
does not restart Soda services or change the running deployment. Activation
uses the already-downloaded deployment and requires the explicit reboot
confirmation. There is no background polling, download, activation, or reboot.

The signed release record binds Soda version and source revision, the Fedora
base reference, exact Soda image reference, platform, state schema, RPM
inventory checksum, and the installer ISO checksum when an ISO is produced.
The state schema remains 2 for this MVP; cross-schema state rollback is not
supported.
