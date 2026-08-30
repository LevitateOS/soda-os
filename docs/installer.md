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
Fedora RPM added to its pinned base and for the four locally built Soda RPM
inputs. The Soda RPMs are build inputs only; no mutable Soda RPM repository is
created or embedded. Weak dependencies are disabled.

During `just oci ARCH`, the Go builder:

1. validates the immutable base, platform, registry, state schema, and package
   lock;
2. reproducibly builds `soda-release`, `soda-runtime`, `soda-cockpit`, and
   `soda-forgejo` with the configured version, source revision, and source date;
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

## Local installer media

Each architecture produces a separate single-platform OCI archive. Run
`just iso ARCH ARCHIVE` to validate that archive contains exactly one matching
`linux/arm64` or `linux/amd64` manifest, derive its exact manifest digest, and
embed it in the matching `bootc-generic-iso`. This path is entirely local: it
does not push to GHCR, access GHCR, use Cosign, require signing credentials, or
verify a signature. The ISO and checksum are architecture-named so sibling
artifacts cannot overwrite one another.

The installer is for fresh installation only. It uses graphical Anaconda with
DHCP, a default hostname of `soda`, and the normal interactive choices for
storage, networking, hostname, and the first administrator. Its sidebar and
product-mark PNGs are generated from the approved Soda v3 SVG masters; the
surrounding navy and cyan visual treatment uses the same established palette in
the Anaconda stylesheet.
The Fedora 44 installer environment runs SELinux in permissive mode because its
live overlay cannot be relabeled; this boot option does not change the installed
Soda image, which retains enforcing SELinux.

Before Anaconda creates the interactive administrator, the installer creates
the persistent `/var/home` parent in the mounted target. Fedora bootc otherwise
creates that parent only through `tmpfiles.d` on first boot, which is too late
for Anaconda to create `/home/<administrator>` through the image-owned
`/home -> var/home` symlink.

The ISO embeds the local OCI payload under its exact digest in container
storage. Fedora 44's `bootc` Kickstart command installs from that ISO-local
`containers-storage:` reference, so installation does not require registry
access.

## Persistent host state

Bootc owns the image base. Soda keeps mutable state under `/var/lib/soda` so it
survives replacement of the image: SQLite schema 4 state, Cockpit certificates,
projects, and toolchains. Image-owned mount units retain the existing visible
paths `/srv/soda/projects` and `/opt/soda/toolchains`; direct SSH workspaces and
the forced-command gateway therefore keep their established paths. `tmpfiles.d`
creates the persistent directories. Linux users and PAM passwords, SSH host
keys, `/etc/soda/authorized_keys`, and `/var/log/soda` are likewise host state.

## Manual OS updates

The runtime masks Fedora's automatic bootc update timer. An administrator uses
`sodactl os update status`, `check`, `stage`, and
`activate --confirm-reboot`. Checking resolves the installed sibling's
release-index entry once and inspects the immutable image metadata. It accepts
only an exact Soda repository digest, the installed platform, and state schema
4. Staging calls bootc with the exact `@sha256:` reference and download-only, so it
does not restart Soda services or change the running deployment. Activation
uses the already-downloaded deployment and requires the explicit reboot
confirmation. There is no background polling, download, activation, or reboot.

The optional local release record binds Soda version and source revision, the Fedora
base reference, exact Soda image reference, platform, state schema, RPM
inventory checksum, and the installer ISO checksum when an ISO is produced.
The state schema is 4; cross-schema state rollback is not supported.
