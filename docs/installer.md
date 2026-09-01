# Soda bootc runtime image and installer

This document describes the current image and installer implementation. The
product contract remains in [architecture-reset.md](architecture-reset.md).

`distro/soda.toml` is the shared runtime image specification.
`distro/platforms/aarch64.toml` and `distro/platforms/x86_64.toml` are equal
sibling platform contracts. Each pins its Fedora 44 bootc manifest, OCI
platform, package and installer locks, tool inputs, artifact identity, and
release channel. The shared specification owns the Soda image name, version,
remaining runtime paths, and source-date epoch. The builder obtains the
source revision from the current Git commit, and every command requires an
explicit `--architecture aarch64` or `--architecture x86_64` selection.

The platform-selected runtime package lock records exact NEVRAs for every
Fedora RPM added to its pinned base and for the four locally built Soda RPM
inputs. Both sibling locks include their independently resolved matching-native
stock-Cockpit dependency closure. The Soda RPMs are build inputs only; no
mutable Soda RPM repository is created or embedded. Weak dependencies are
disabled.

During `just oci ARCH`, the Go builder:

1. validates the immutable base, platform, registry, and package lock;
2. reproducibly builds `soda-release`, `soda-runtime`, `soda-projects`, and
   `soda-forgejo` with the configured version, source revision, and source date;
3. installs the exact locked transaction into the pinned Fedora bootc base;
4. creates the temporary `soda-api` group and the Linux-native
   `soda-workspaces` classification group;
5. composes stock Cockpit's PAM policy and enables SSH, `cockpit.socket`, the
   one-attempt Tailscale enrollment unit, Forgejo, native nftables, the reduced
   residual Soda service, and the remaining toolchain mount;
6. masks the automatic bootc update timer while retaining manual bootc
   operations;
7. records the complete installed RPM inventory and verifies its SHA-256; and
8. exports an OCI archive without loading, pushing, signing, or publishing it.

OCI labels record the Soda version, Git revision, creation time, and pinned
base. BuildKit rewrites image timestamps to the configured source-date epoch
and omits provenance attestations from this local artifact.

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
`soda-forgejo` PAM policy. The accepted outcome is that a primary human can log
in with their Linux username and password, after which Forgejo creates its own
ordinary native user record. The exact pinned Forgejo service cannot currently
read the password verifier required by PAM without a new `/etc/shadow`
privilege boundary, so later-user PAM login remains deliberately unenabled
pending that decision. Linux account creation performs no Forgejo operation,
and later `wheel` membership has no Forgejo effect. Derived workspace accounts
are Linux-only development identities that use their installed authorized
public keys for direct OpenSSH access; the shipped account rule rejects the
`soda-workspaces` group so they cannot become Forgejo users once the primary
login boundary is resolved.

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

Bootc owns the image base. Linux owns primary and derived accounts, groups,
passwords, private homes, standard authorized-key files, and SSH host keys.
Forgejo owns its database and repositories. Tailscale owns its enrolled node
state. Soda's only mutable Projects state is the exact three-field catalog
below `/var/lib/soda/catalog`; complete workspace clones live in the derived
accounts' ordinary `$HOME/Projects` directories.

The pre-reset Soda database, copied person/project/repository state, shared
project mount, alternate authorized-key tree, Cockpit certificates, and
standalone dashboard state are not created. The image-owned toolchain cache and
mount remain temporary until issue #24 removes the runtime toolchain manager.

## Manual OS updates

The runtime masks Fedora's automatic bootc update timer. A Linux administrator
uses native `bootc` with an exact Soda image reference:

```sh
sudo bootc status
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

The download-only step does not change the running deployment. The second
switch command turns the already downloaded image into a bootable deployment;
the administrator then performs the controlled reboot. Supported fallback
uses the same sequence with an earlier exact Soda digest. Direct
`bootc rollback` is unsupported because it can restore the earlier
deployment's historical `/etc` instead of preserving current Linux account
state.

Soda ships no runtime release-index client, translated update state, update
API, CLI wrapper, polling, download service, activation service, retry, or
recovery process.

The optional local release record binds Soda version and source revision, the
Fedora base reference, exact Soda image reference, platform, RPM inventory
checksum, and the installer ISO checksum when an ISO is produced.
