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
Fedora RPM added to its pinned base and for the locally built Soda RPM inputs.
Those inputs include the narrow architecture-specific `soda-bun` and
`soda-tea` packages.
Both sibling locks include their independently resolved matching-native stock-
Cockpit and immutable-development-tool dependency closures. The Soda RPMs are
build inputs only; no mutable Soda RPM repository is created or embedded. Weak
dependencies are disabled.

During `just oci ARCH`, the Go builder:

1. validates the immutable base, platform, registry, and package lock;
2. reproducibly builds `soda-release`, `soda-runtime`, `soda-projects`,
   `soda-forgejo`, `soda-bun`, and `soda-tea` with the configured version,
   source revision, and source date;
3. installs the exact locked transaction into the pinned Fedora bootc base;
4. creates the temporary `soda-api` group and the Linux-native
   `soda-workspaces` classification group;
5. composes stock Cockpit's PAM policy and host packages and enables SSH,
   `cockpit.socket`, the one-attempt Tailscale enrollment unit, Forgejo, native
   nftables, and the reduced health-only Soda service;
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

The installer is for fresh installation only. It uses stock graphical Anaconda
with DHCP, a default hostname of `soda`, and Anaconda's normal storage and
installation workflow. Soda supplies branding, immutable-image composition,
and two fixed installer-only hooks; it does not ship a custom Anaconda spoke,
module, D-Bus service, or alternate installer UI. The administrator inputs come
from the mandatory protected OEMDRV medium described below, so the stock User
and Password spokes are hidden. The sidebar and product-mark PNGs are generated
from the approved Soda v3 SVG masters; the surrounding navy and cyan visual
treatment uses the same established palette in the Anaconda stylesheet.

### Protected OEMDRV installer input

Create a new answer medium on the matching-native architecture after building
the ISO and its release record:

```sh
ARCH=x86_64
go run ./cmd/soda-image --architecture "$ARCH" installer-input \
  --iso ".artifacts/images/SodaOS-0.4.0-${ARCH}.iso" \
  --release-record ".artifacts/releases/soda-os-0.4.0-${ARCH}.release.json" \
  --username soda-admin \
  --ssh-public-key-file "$HOME/.ssh/id_ed25519.pub" \
  --tailscale-auth-key-file /secure/path/tailscale-auth-key \
  --output /secure/path/soda-installer-input.iso
```

Without `--password-file`, the command reads and confirms the administrator
password from the controlling terminal. Automation may instead pass a
root-owned or user-owned mode-`0600` regular file with `--password-file`; the
Tailscale key must likewise be supplied through a mode-`0600` regular file.
Secret files cannot be symlinks. Never place either secret in an argument,
environment value, repository file, or log.

The command verifies the selected native architecture, release-record
platform, and exact installer-ISO checksum. It refuses to overwrite an output
and publishes a mode-`0600` ISO labelled `OEMDRV`. That medium contains a
secret-free Kickstart composition plus exactly the administrator username,
password, authorized public key, and one-use Tailscale key. The medium itself
therefore contains secrets: attach it only as removable installation media,
keep it protected, and destroy its host copy after the installer ejects it.
Normal installations omit `--unattended` and retain Anaconda's interactive
storage workflow. The repository acceptance harness alone uses that explicit
flag to add a fixed destructive partitioning recipe for its disposable VM.

The product ISO's boot entry selects `/ks.cfg` from `OEMDRV` and tells Anaconda
not to save input or output Kickstart. During `%pre`, the fixed installer input
hook mounts OEMDRV read-only with `nodev`, `nosuid`, and `noexec`, validates the
four values, creates only root-owned transient files below
`/run/soda-installer`, and emits native Kickstart `user` and `sshkey`
directives. It then unmounts and ejects the answer medium and waits for that
device to disappear. Ejection and removal are mandatory: if the medium remains
present, the hook removes its transient files and stops installation.

After Anaconda has created the Linux account, the fixed `%post --nochroot`
finalizer consumes and unlinks the transient inputs, validates the installed
account, and performs the bounded Forgejo handoff. It retains no plaintext
password in the target; the only transient secret handoff it writes is the
one-use Tailscale key for first boot. Both hooks exist only in the installer
environment; neither is installed in the Soda runtime image.

## Accepted initial provisioning outcome

The Linux administrator and Tailnet portion of the first supported installation
path requires four values:

```text
administrator username
administrator password
administrator SSH public key
one-use Tailscale auth key
```

The installer input hook hashes the password through `openssl passwd` on
standard input and emits native Kickstart `user` and `sshkey` directives into
the installer-only runtime directory. Anaconda remains authoritative for
creating the ordinary Linux account, adding it to `wheel`, setting its Linux
password, and installing its SSH public key in standard
`~/.ssh/authorized_keys`.

A minimal first-boot systemd oneshot passes the enrollment credential to
`tailscale up` from root-owned `/var/lib/soda-install/tailscale-auth-key`,
removes the file after the single attempt, and disables itself whether the
attempt succeeds or fails. Tailscale retains its own node identity in its
upstream state location; Soda no longer relocates that state.

The same installation creates the only proactive Forgejo user: a same-named
Forgejo-local site administrator through Forgejo's native first-user signup.
The installer-only finalizer initializes the target's package-owned Forgejo
state, starts pinned Forgejo on loopback with a sealed in-memory configuration
that temporarily permits registration, submits the password only in the
loopback HTTP body, verifies the active administrator, and stops the transient
process. Forgejo's durable configuration remains registration-disabled. The
password is never a process argument, environment value, log field, or retained
Soda or target file. This is a bounded installation handoff, not a runtime
Forgejo credential service.

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
password through PAM. The one-attempt enrollment unit always removes the
Tailscale credential and disables itself, whether enrollment succeeds or
fails. Soda retains no bootstrap database, API, custom authentication, public
onboarding endpoint, bundle format, durable workflow, retry or reconciliation
state, separate bootstrap user, runtime credential storage, or bootstrap
status.

There is no in-place Soda installer recovery workflow. If input validation,
account creation, Forgejo initialization, or target finalization fails, treat
the target as incomplete: correct the inputs, create a new protected OEMDRV
medium, and repeat a fresh installation. If the single Tailscale attempt fails,
use native local Tailscale recovery when available or reinstall with a fresh
one-use key. Do not reuse the ejected credential medium or expect Soda to retry
from retained state. Acceptance testing proves private OpenSSH and Cockpit
reachability and the absence of retained enrollment material; it creates no
runtime verification state.

The runtime uses Fedora's native `nftables.service` with one fixed Soda ruleset:
TCP 22, 9090, and 30000 are accepted on loopback and `tailscale0` and rejected
on other ingress. The ruleset otherwise keeps an accept policy and does not own
project-selected ports or unrelated Linux networking.

The Fedora 44 installer environment runs SELinux in permissive mode because its
live overlay cannot be relabeled; this boot option does not change the installed
Soda image, which retains enforcing SELinux.

Anaconda creates the administrator through the standard Kickstart `user` and
`sshkey` commands. Fedora bootc exposes `/home` through the image-owned
`/home -> var/home` symlink. Fedora Anaconda 44.30's bootc mount preparation
binds the host `/sys` non-recursively, which otherwise hides its nested
SELinuxFS mount from Anaconda's own final context pass. The installer image
carries one exact-version- and source-hash-guarded correction that also binds
`/sys/fs/selinux` and tracks it for Anaconda's normal reverse teardown. Anaconda
therefore remains responsible for applying the installed policy to
`/var/home`, including the administrator's SSH files. Soda runs no relabel
command and creates no runtime relabel service or second home authority.

The ISO embeds the local OCI payload under its exact digest in container
storage. Fedora 44's `bootc` Kickstart command installs from that ISO-local
`containers-storage:` reference, so installation does not require registry
access. Pinned `bootc` deliberately uses the live host's `/var/tmp` for large
import files. The installer environment mounts a 4 GiB ephemeral tmpfs there
before Anaconda starts, keeping payload scratch outside the small LiveOS
overlay and out of installed Soda state. The matching-native raw-QEMU harness
allocates 8 GiB for this path.

## Persistent host state

Bootc owns the image base. Linux owns primary and derived accounts, groups,
passwords, private homes, standard authorized-key files, and SSH host keys.
Forgejo owns its database and repositories. Tailscale owns its enrolled node
state. Soda's only mutable Projects state is the exact three-field catalog
below `/var/lib/soda/catalog`; complete workspace clones live in the derived
accounts' ordinary `$HOME/Projects` directories.

The pre-reset Soda database, copied person/project/repository state, shared
project mount, alternate authorized-key tree, Cockpit certificates, and
standalone dashboard state are not created. Neither `/opt/soda/toolchains` nor
`/var/lib/soda/toolchains` exists, and no toolchain mount, profile, readiness
record, downloader, or runtime manager is shipped.

## Immutable development toolset

The image installs the approved command collection from exact Fedora package
locks plus the checksum-locked, architecture-specific `soda-bun` and
`soda-tea` RPMs. The approved commands cover Go, Python and uv, Rust, Node.js,
Bun, native compilers and build systems, Git, SSH, GitHub CLI, the
Forgejo-compatible Tea CLI, rootless container tools, data and network
utilities, archives, and editors. The exact command-level contract is
installed at `/usr/share/soda/toolset-commands.txt`, with one command per line.
Image construction fails when any listed command is unavailable through
ordinary system `PATH`.

Bun and Tea source acquisition are bounded build inputs: each matching-native
builder fetches only the selected official architecture assets, verifies their
locked checksums and licenses, and builds the local RPMs without network
access. Soda performs no runtime tool discovery or download. Primary and
derived accounts use the same immutable commands while retaining their own
ecosystem caches, virtual environments, project-local dependencies, and
Git-host CLI authentication in their ordinary homes.

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

The local release record binds the runtime image's Soda version, source
revision, Fedora base reference, exact image reference, platform, RPM inventory
checksum, and installer ISO checksum. It does not independently record the
installer-environment source revision. When a validated runtime OCI is reused
for an installer-only change, identify the installer candidate by the
repository commit and exact ISO checksum while recording the embedded runtime
digest separately. Artifact construction does not require the record; protected
OEMDRV creation does.

## Current verification status

The protected installer path was exercised end to end on a fresh native x86-64
guest from installer source commit `2e5c596`. The run proved OEMDRV protection,
guest ejection and exact host removal, stock-Anaconda installation, Linux and
same-named Forgejo administrator creation, password and public-key SSH, correct
home and key SELinux labels, one-attempt Tailscale enrollment and handoff
deletion, stock Cockpit authentication and workspace rejection, Projects setup,
direct derived-account SSH, the immutable toolset, rootless Podman, obsolete-
state absence, the residual Health RPC, and the exact installed runtime digest.
The artifact hashes and failure history are recorded in
[bug-notes.md](bug-notes.md); they are local evidence, not a published release.

The latest protected-Kickstart installer path still requires independent
matching-native AArch64 construction, installation, and installed-system
verification. Earlier AArch64 evidence for the removed add-on does not qualify
this implementation.
