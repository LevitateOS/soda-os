# Soda bootc runtime image and installer

> [!IMPORTANT]
> This document describes the pre-reset image and installer implementation
> currently present in the repository. It is implementation evidence, not the
> target runtime ownership model. See the
> [architectural reset](architecture-reset.md) and
> [issue #40](https://github.com/LevitateOS/soda-os/issues/40) for the governing
> direction.

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
5. enables SSH, the one-attempt Tailscale enrollment unit, native nftables,
   Soda services, Avahi, and the persistent-state bind mounts;
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

This section records the issue #40 implementation boundary. Final installed
Tailnet-to-stock-Cockpit and workspace-account integration evidence remains
deferred to the dependent reset milestones and issue #25.

The Linux administrator and Tailnet portion of the first supported installation
path requires four values:

```text
administrator username
administrator password
administrator SSH public key
one-use Tailscale auth key
```

The required Soda Anaconda spoke composes native Anaconda user and SSH-key data:
Anaconda creates the ordinary Linux account, adds it to `wheel`, sets its Linux
password, and installs its SSH public key in standard
`~/.ssh/authorized_keys`. The native Users module holds the password only for
the bounded installation operation; the Soda task replaces that in-memory
value with an Anaconda-generated hash before output Kickstart is written.

A minimal first-boot systemd oneshot passes the enrollment credential to
`tailscale up` from root-owned `/var/lib/soda-install/tailscale-auth-key`,
removes the file after the single attempt, and disables itself whether the
attempt succeeds or fails. Tailscale retains its own node identity in its
upstream state location; Soda no longer relocates that state.

The same installation creates the only proactive Forgejo user: a same-named
Forgejo-local site administrator through Forgejo's native first-user signup.
The task initializes the target's package-owned Forgejo state, starts pinned
Forgejo on loopback with a sealed in-memory configuration that temporarily
permits registration, submits the password only in the loopback HTTP body,
verifies the active administrator, and stops the transient process. Forgejo's
durable configuration remains registration-disabled. Soda creates no separate
Forgejo password handoff: the password is never a process argument,
environment value, log field, or retained Soda or target file. Raw-QEMU
acceptance necessarily carries installer inputs in a protected, transient
Kickstart and OEMDRV image; it removes the Kickstart source after image
creation, then uses QMP to eject the OEMDRV and removes its host file after
Anaconda reports that it parsed the Kickstart.

Forgejo's native PAM source delegates later authentication to the shipped
`soda-forgejo` PAM policy. Any primary human Linux account accepted by that
policy can log in with its Linux username and password; Forgejo creates its own
ordinary native user record on first successful login. Linux account creation
performs no Forgejo operation, and later `wheel` membership has no Forgejo
effect. Derived workspace accounts are Linux-only development identities that
use their installed authorized public keys for direct OpenSSH access; the PAM
policy must reject them so that they never become Forgejo users.

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

The runtime uses Fedora's native `nftables.service` with one fixed Soda ruleset:
TCP 22, 9090, and 30000 are accepted on loopback and `tailscale0` and rejected
on other ingress. The ruleset otherwise keeps an accept policy and does not own
project-selected ports or unrelated Linux networking.

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
