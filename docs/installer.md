# Soda bootc runtime image

`distro/soda.toml` is the schema-version-2 runtime image contract. It pins the
approved Fedora 44 bootc manifest digest, `linux/arm64` platform, Soda image
name, runtime state schema, package lock, Soda version, and source-date epoch.
The builder obtains the source revision from the current Git commit.

`packaging/bootc/packages.lock` records exact NEVRAs for every Fedora RPM added
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

ISO generation, image signing, registry publication, and an update API are not
implemented in this phase.
