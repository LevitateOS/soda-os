# Soda OS product contract

This is the accepted release-day product and its ownership boundaries, not a
report of implementation readiness. Explicit owner decisions govern this
contract; source, tests, research, and issues provide evidence and track work.
The [public handbook](public/10-Start-here/10-index.md) teaches the finished
experience. [Architecture](architecture.md) describes its implementation.

## Purpose and audience

Soda turns a powerful, privately reachable machine into a coherent remote
Linux development environment. A developer can use a computer they own as a
complete remote environment or additional capacity alongside their preferred
client. For teams, the business or technical owner selects a shared server;
developers use ordinary editors, terminals, browsers, and Git from their clients.

Soda is cloud-first, not cloud-only. Trusted LAN installations expose services
directly over the LAN; cloud installations use Tailscale, never public service
ingress. Public DNS is unnecessary. Initial installation requires a physical,
VM, or supported provider console, not public SSH or a Soda source checkout.

AArch64 and x86-64 are equal sibling targets for signed OCI images, network
installer ISOs, and reusable compressed QCOW2 images. Architecture-specific
preparation, builds, inspection, signing, publication, installation, and evidence
require matching hardware. WSL2 on x86-64 Windows is explicitly future-release
work, with no launch download; that exception does not narrow ISO/QCOW2 support.

## Native ownership

| Owner | Authoritative facts and behavior |
| --- | --- |
| Fedora, Anaconda, cloud-init, bootc | Base image, native installation/provisioning, deployments |
| Linux and OpenSSH | Accounts, passwords, groups, homes, permissions, processes, remote transports |
| Stock Cockpit | Browser authentication, sessions, TLS, administrative elevation, native host pages |
| Forgejo or external Git host | Users, keys, repositories, permissions, collaboration, provider CI |
| Tailscale | Tailnet identity, authentication, peers, routing, approvals |
| mise and language package managers | Tool versions, installation, repository configuration, dependencies, caches |
| Soda | Installable composition, branding, catalog, workspace convention, focused pages and narrow synchronous operations |

Retain Soda's coherent user workflow without copying upstream authority.
There is no general Soda daemon, runtime API, database, credential broker,
SSH gateway, container controller, tool manager, custom updater, or durable
job/retry/reconciliation system. Native configuration and bounded adapters are
legitimate; neither subsystem takeover nor deleting accepted Soda outcomes is.

## Installation and access

- Graphical Anaconda owns ISO storage, networking, bootc deployment, Linux user
  creation, and administrator selection. Root remains locked. The installed ISO
  disables cloud-init through its native disabled file.
- QCOW2 uses standard Fedora cloud-init delivered by provider or VM tooling.
  User-data supplies the primary account, SSH public key, password hash for
  password-based console/Cockpit/PAM access, and native network configuration.
  A public key alone does not supply a password. No manually built credential
  ISO, alternate Soda onboarding system, or public-SSH bootstrap is required.
- Interactive local and SSH shells always show stateless welcome guidance:
  native hostname, local and connected Tailnet URLs, current-user SSH command,
  and Tailscale status. Administrators may customize the native profile entry.
  Non-interactive SSH, automation, SCP, and SFTP keep ordinary OpenSSH behavior.
- Firewalld retains enabled Anaconda/Fedora defaults and native SSH access;
  Soda adds TCP 9090 for Cockpit. Administrators open Forgejo and development
  ports for LAN access through stock Networking → Firewall. Soda does not
  replace the default zone or overwrite later administrator choices.
- Tailscale is installed and enabled, initially unenrolled. Native console
  authentication provides the initial private cloud connection. Once Cockpit
  is reachable, its separate Tailscale page provides native browser sign-in,
  device addresses, peers, exit-node selection, LAN-access preference,
  advertisement, and approval guidance. Tailscale must not disable LAN access.
- Authentication links appear before sign-in completes. Native state owns
  pending authentication across page visits. Closing a page closes its handles,
  not the machine's Tailnet enrollment. Connection and Forgejo-address refresh
  failures are independent; no watcher or saved recovery workflow is added.

## People, Git, and credentials

Every human has one stable primary Linux username. Linux owns identity;
`wheel` is the Linux administrator fact. Stock Cockpit Accounts or ordinary
Linux tools create later humans and grant administrator membership. Soda has
no identity database, rename reconciliation, or independent role model.

Every primary human's ordinary Forgejo PAM login creates an ordinary Forgejo
profile, regardless of login order and without an administrator prerequisite.
New configurations disable browser registration. Linux administrators may
explicitly create a separate Forgejo administrator through its native CLI,
with a separate Forgejo password. Existing Forgejo administrators can promote
PAM users through the native web interface. Existing configuration and roles
are preserved; `wheel` does not grant a Forgejo role.

Workspace accounts are Linux-only development identities, never Forgejo users.
Git hosts own manual public-key registration and repository authorization.
Soda keeps no identity mirror, password verifier copy, shared token, permission
projection, or cross-system role synchronization.

## Projects and workspaces

Every primary human can discover and edit one shared declarative project
catalog. Repositories are created in Forgejo or an external Git host, then
added with their credential-free SSH clone URL. Projects does not create
repositories. Project ID and canonical URL are immutable; display information
and additional metadata are editable. There is no independently approved
closed metadata field list. Additional product-visible facts need an explicit
decision, not speculation about what a catalog could contain.

**Set up for me** creates one derived Linux account per human-project pair,
with a private UID, home, complete clone beneath `$HOME/Projects/<repository>`,
dependencies, caches, data, and processes. Development happens in workspaces,
not primary accounts. Linux-native account classification and the deterministic
association replace a Soda workspace database.

Setup requires a valid primary `~/.ssh/authorized_keys` before mutation. It
copies current public keys once into the workspace's standard file, with no
later synchronization. The workspace creates and keeps its outbound private
Git key. If Git authorization is missing, Projects presents its public key for
manual registration at the host; an explicit retry completes the clone.
Projects receives no Git password, registers no key, and retains no credential
or provisioning state. Tea and GitHub CLI are available in every workspace,
but authenticated manually and separately there. Assistant credentials are
also personal and workspace-specific; none are copied from the primary home.

Account existence remains visible after failed setup. It is not proof of a
complete checkout. Read-only inspection may report current keys/checkout
observations without mutating accounts, contacting Git hosts, or storing a
readiness flag. Unknown results remain unknown until checked.

People connect directly as the derived workspace username through OpenSSH.
There are no forced commands, synthetic homes, selectors, or custom SFTP
paths. Listing, inspection, and setup do not depend on Tailscale identity.
Browser SSH guidance uses the hostname through which Cockpit was opened.

Projects share the host network namespace and select non-conflicting ports.
A developer sends a normal server URL to a teammate; direct LAN or Tailnet
access includes hot reload without a Share action, port registry, process
tracker, or proxy. Optional user-operated Podman is not Soda's isolation model.
The trusted-team model reduces accidental interference, not hostile tenancy.

Developers invoke mise directly and share native tool configuration through
Git. Language package managers still own repository dependencies. Soda owns
no tool picker, installer action, cache format, shared installation store,
configuration writer, profile system, version state, or cleanup lifecycle.

## Destructive local operations

| Action | Authority and result |
| --- | --- |
| Remove my workspace | Caller removes only their own workspace; other workspaces and catalog remain |
| Remove project | Administrator stops local work, deletes every associated workspace, and removes the catalog entry last |
| Replace canonical URL | Administrator removes the project and all local workspaces, then adds it again |
| Remove person through Soda | Administrator removes local workspaces first and the primary Linux account last |

All these preserve the authoritative repository. Person removal neither
inspects nor deletes the Forgejo account and cannot depend on Forgejo being
available. Forgejo account deletion is a separate native action. Generic
Cockpit/Linux account deletion is non-cascading.

Local removal permanently destroys homes, clones, uncommitted work, dependencies,
and project data. The trusted team coordinates and preserves wanted work;
Soda supplies clear scope review and confirmation, not approvals, archival,
transfer policy, rollback, or recovery. Execution rechecks the selected native
identities. Failure stops further deletion and distinguishes confirmed,
uncertain, and unattempted results. Retry requires explicit renewed review.

## Cockpit, Runners, and Updates

Stock Cockpit remains the only browser administration surface. Soda adds branding,
Projects, an administrator-only Runners package, a separate Tailscale page, and
administrator-only Soda Updates. They reuse native authentication and elevation,
not another web server, session layer, daemon, or generic privileged bridge.

Runners manages local Forgejo/GitHub runner accounts, clients, and systemd
listeners through its narrow helper. Providers own registration, workflows,
scheduling, and job history. Soda does not become a CI control plane.

Soda Updates discovers approved published stable releases, verifies the signed
architecture-matched record and exact OCI digest, downloads it, and separates
that download from explicit Apply and restart. It uses native bootc, Skopeo,
and Cosign through a synchronous command. Native deployments—not a Soda or
browser database—own status. Automatic updates and automatic reboots are off.

Administrators retain ordinary bootc commands. Supported fallback selects an
earlier exact signed image while preserving current Linux accounts, passwords,
groups, roles, homes, catalog, workspaces, Forgejo, Tailscale, SSH, and other
mutable state. Direct `bootc rollback` is not supported without proof of that
invariant. Fallback is not data restoration and adds no recovery engine.

## Release evidence

Expensive matching-native installation, product, update, and fallback checks
run before release CI on user-controlled machines. Previous fallback A is a
signed published OCI digest, not a rebuild. One signed strict acceptance record
binds the exact source, suite revision, both architectures, required checks,
fallback identities, completion time, and approved signer. It authenticates
observations; it does not claim that later CI-built bytes were boot-tested.

Protected production CI verifies that record, runs cheap source checks once,
builds each architecture's release B once in parallel, structurally verifies
and signs the exact OCI/ISO/QCOW2 outputs, and publishes them unchanged. It runs
no guest or acceptance-only enrollment. Source identity, immutable digests,
checksums, provenance, anonymous retrieval, remote asset verification, and
production-commit checks remain required. The 30-minute target and 45-minute
slow-stage warning do not authorize a hard timeout or a weaker gate.

See [build and release](build-and-release.md) for operations and
[acceptance](../tests/acceptance/README.md) for evidence and outstanding coverage.
