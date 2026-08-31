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

## Accepted initial provisioning outcome

This is the accepted target for the first supported path. The current installer
does not yet prove the complete behavior described below.

The Linux administrator and Tailnet portion of the first supported installation
path requires four values:

```text
administrator username
administrator password
administrator SSH public key
one-use Tailscale auth key
```

Anaconda and Kickstart create the ordinary Linux account, add it to `wheel`, set
its Linux password, and install its SSH public key. A minimal first-boot systemd
oneshot passes the enrollment credential to `tailscale up` from a temporary
file, removes that file after the enrollment attempt, and disables itself.

The same installation creates the only proactive Forgejo user: a same-named
Forgejo-local site administrator through Forgejo's native administrative
interface. The selected outcome gives it the same initial password as the Linux
account. The installer may reuse that password only through an existing bounded
handoff that leaves no Soda-owned credential state or retained plaintext. If
the current installer cannot provide that direct handoff, password equality is
reconsidered rather than expanded into Soda credential machinery.

Forgejo's native PAM source delegates later authentication to the shipped
`soda-forgejo` PAM policy. Any Linux account accepted by that policy can log in
with its Linux username and password; Forgejo creates its own ordinary native
user record on first successful login. Linux account creation performs no
Forgejo operation, and later `wheel` membership has no Forgejo effect.

Soda may set the PAM source's email domain to the fixed packaging convention
`localhost`, allowing Forgejo to initialize `<username>@localhost`. The setting
is not an installer input or upstream requirement. Soda does not collect,
persist, or manage per-user Forgejo email addresses.

The initial Forgejo-local administrator and same-named Linux account become
independent immediately after installation. Later account, password, role,
rename, disable, and deletion changes are not synchronized. Disabling a PAM
user's Linux account blocks later PAM authentication but does not claim to
revoke Forgejo sessions, tokens, SSH keys, or repository permissions.

After installation, the administrator connects through the Tailnet with
OpenSSH and authenticates to stock Cockpit with the ordinary Linux username and
password through PAM. Soda retains no bootstrap database, API, custom
authentication, public onboarding endpoint, bundle format, durable workflow,
retry or reconciliation state, separate bootstrap user, or runtime bootstrap
status.

The first version does not implement in-place recovery orchestration. If
Tailscale enrollment fails, the operator uses available local recovery or
corrects the installer inputs and reinstalls. Acceptance testing proves private
OpenSSH and Cockpit reachability and the absence of retained enrollment
material; it does not create runtime verification state.

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
