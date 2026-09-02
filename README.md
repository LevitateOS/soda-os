# Soda OS

![soda os](assets/branding/source/soda-logo-horizontal.svg)

Soda OS is an opinionated Fedora bootc remote-development appliance for trusted
private networks, with equal AArch64 and x86-64 sibling support. Lightweight
clients connect over Tailscale and SSH to a more powerful development machine.
Linux, OpenSSH, Cockpit, Git, Forgejo, and bootc should own their existing
responsibilities.

Soda owns the branded installable composition, a broad curated image-resident
development toolset, a minimal catalog of offered projects, and the narrow
workflow that creates one derived Linux workspace account per human-project
pair. Each workspace account owns its private home, complete Git clone,
user-local dependencies, project data, and development processes. Humans enter
those accounts directly through ordinary OpenSSH; repository access and
collaboration remain native to bundled Forgejo or the external Git host.

Stock Cockpit provides host administration and authentication. One focused
Soda Projects page lets any primary human add, edit, remove, or set up a
catalogued project and gives administrators the supported cascading human
deletion action. It may invoke one short synchronous privileged operation for
the accepted catalog and workspace lifecycle, but Soda retains no general
project control plane, daemon, database, RPC API, credential store, job engine,
or reconciliation system. Cockpit's Fedora-owned pages read generic host,
service, journal, storage, and network state directly from Linux; Soda does not
translate or copy that telemetry. Podman is available as an optional
development tool and is not the isolation mechanism.

The [base principles](docs/principles.md) state the product purpose and ownership
philosophy. The [architectural reset](docs/architecture-reset.md) records the
accepted architecture and issue ownership. The native workspace and
image-lifecycle slices have removed the custom identity, project,
repository-projection, dashboard, SSH gateway, database, workflow, and
runtime-update layers. The native host and immutable-toolset slice has also
removed Soda telemetry and runtime toolchain management. Only the temporary
health-only daemon, local socket, and protobuf/gRPC shell remain for the final
control-plane deletion milestone.

This repository is independent from LevitateOS. It borrows the separation
between declarative distro specifications, Go orchestration, explicit
contracts, and scenario tests. Fedora supplies the pinned Fedora 44 bootc
base, kernel, userspace, RPM/DNF, systemd, SELinux, and SSH.

## Repository layout

- `cmd`: bounded runtime, Projects, helper, and artifact executables
- `cockpit`: the static stock-Cockpit Projects package
- `internal`: native Projects behavior, the temporary health-only runtime, and the artifact pipeline under `internal/build`
- `distro`: Soda identity, immutable tool manifest, distribution locks, and Fedora base metadata
- `packaging`: bootc and RPM inputs grouped by shipped package
- `assets`: canonical Soda branding sources and rendered assets
- `docs`: architecture, artifact, branding, and release operations
- `tests/acceptance`: blank-disk bootc acceptance scenario and runner
- `scripts` and `tools`: repository checks and developer tooling

## Development

```sh
just check
ARCH=x86_64 # or aarch64
just rpm "$ARCH"
just oci "$ARCH"
just iso "$ARCH" ".artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar"
```

Build artifacts are written under `.artifacts/` and are never committed.
`just rpm` builds the locked local Soda RPM inputs, including the narrowly
packaged Bun binary. `just oci` builds those RPMs and emits
`.artifacts/images/soda-os-0.4.0-${ARCH}.oci.tar` without loading or publishing
the image. `just iso` derives the exact image digest from that local archive and
embeds it in a platform-matched installer without a registry, signing key, or
network publication step. Architecture selection is always explicit; neither
sibling is a default or fallback.
The package lock pins every Fedora package added to the immutable base, and the
finished image contains a complete RPM inventory plus its verified SHA-256
checksum. Both sibling locks include their independently resolved
matching-native stock-Cockpit and development-tool closures. The installed
command contract is recorded at `/usr/share/soda/toolset-commands.txt`; every
listed command is ordinary immutable image content available through `PATH`.

Local development does not publish or sign images. Optional release metadata
records preserve the exact local archive digest, image labels, RPM inventory,
and ISO checksum. The paired GitHub publisher can distribute completed sibling
artifacts, but it does not participate in local OCI or ISO construction. See the
[release and operator runbook](docs/release-operations.md) for the exact
commands and [runtime image and installer contract](docs/installer.md) for the
artifact boundary.

## Target operating model

Installation uses stock graphical Anaconda and a protected, removable OEMDRV
answer medium bound to the exact installer ISO. Anaconda and Kickstart create
the first primary human Linux administrator and install that human's SSH public
key. One fixed installer-only finalizer creates the initial same-named Forgejo
administrator through Forgejo's native interface, and one bounded first-boot
invocation enrolls the machine in Tailscale. The installer requires the
secret-bearing OEMDRV medium to be ejected and removed before installation can
continue. Soda ships no custom Anaconda spoke, and no bootstrap service,
credential store, or private bootstrap state remains active after the one
Tailscale attempt; the disabled one-shot unit contains no credential.

Every primary human Linux account is a Cockpit identity and may become a native
Forgejo PAM user. Primary usernames remain stable while derived workspaces
exist. Derived workspace accounts are identified through Linux-native state,
are Linux-only development identities, and never become Forgejo users.

Any primary human may publish, edit, or destructively remove a catalog entry
containing exactly an immutable `id`, mutable `display_name`, and credential-
free mutable `canonical_url`. For a catalogued project, successful **Set up for
me** uses native user-authenticated Git or repository-host behavior, creates the
derived workspace account, and leaves a complete checkout below that account's
`$HOME/Projects/<repository>`. Setup requires a public key in the primary
account's standard `~/.ssh/authorized_keys`, copies those keys once, and retains
no Git credential or workflow state. A no-URL project begins as a native empty
Forgejo repository.

Removing a project deletes its derived workspace accounts, homes, checkouts,
and explicitly Soda-created paths before removing the catalog entry; it never
deletes the canonical repository. Supported human deletion is an
administrator-only Projects action that removes derived workspaces and deletes
the primary account last. Generic Cockpit or command-line account deletion is
out-of-band and non-cascading.

Projects choose non-conflicting host ports themselves. They may use Podman or
other ordinary tools when useful; Soda does not allocate ports or manage
network namespaces. The broad reviewed Go, Python, Rust, JavaScript, native
build, Git, container, data, archive, and editor toolset is installed in the
image. Users keep language packages, caches, and project-local dependencies in
their ordinary homes and workspaces; Soda has no runtime toolchain resolver,
profile, readiness database, downloader, or update service.

Linux administrators operate deployments through native
`bootc status`, `bootc switch --download-only <exact-reference>`, and
`bootc switch --from-downloaded`, followed by a controlled reboot. Supported
fallback selects an earlier exact Soda digest through the same switch path and
preserves current Linux account state. Direct `bootc rollback` is unsupported.
The automatic update timer remains disabled, and Soda ships no runtime update
service, discovery client, API, or CLI wrapper.

See the [architectural reset](docs/architecture-reset.md), the
[current implementation architecture](docs/architecture.md), the
[runtime image and installer contract](docs/installer.md), the
[managed listener contract](docs/networking.md), and the
[release and operator runbook](docs/release-operations.md).
