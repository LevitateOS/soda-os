# Base system

Soda OS is built from a Fedora bootc base pinned by digest. Fedora supplies the
kernel, userspace, package manager, systemd, SELinux policy, OpenSSH, bootc, and
their RPM provenance. Soda OS is independent and is not endorsed by Fedora.

## Native administration and access

Stock Cockpit owns browser authentication and the overview, metrics, services,
logs, accounts, terminal, storage, and networking pages. Soda adds branding and
one focused Projects package. There is no Soda telemetry service, host-status
API, or separate administration backend.

On a trusted LAN, OpenSSH, Cockpit, and Forgejo are directly reachable. Cloud
deployments use Tailscale and never expose those services to the public
Internet.

Anaconda creates ISO Linux accounts; standard cloud-init provisions QCOW2.
Log in normally for stateless welcome and connection guidance. Add a personal SSH
public key through Cockpit Accounts before the first workspace; cloud-init may
already supply it. Workspace creation copies those authorized keys once.

The owner registers the first Forgejo account through the normal trusted LAN
or Tailnet before teammates sign in. Native first-user signup grants Forgejo
administration. Use independent Forgejo credentials, even with the same username
as the Linux owner. PAM remains active. Later Linux users' first successful PAM
login creates ordinary Forgejo accounts. Linux wheel membership grants no
Forgejo role. The team controls ongoing registration policy; there is no
mandatory registration-closing step or associated restart.

Repositories are created through the authoritative Git host and added to
Projects with an SSH clone URL. Each workspace keeps its outbound private Git
key locally; the person registers its public key with the authoritative host
before retrying setup. Projects accepts no Forgejo password and creates no
repository or Git-host key record.

## Development tools

`mise` owns development-tool installation, versions, and project configuration.
People invoke and configure it directly inside their workspaces, and project
configuration is shared through the native repository workflow. Upstream tool
managers own their cache behavior. Projects has no tool selections, installer,
shared tool storage, or lifecycle; Soda has no cache service, downloader,
profile system, or toolchain database.

Tea and GitHub CLI are available in every workspace. Authenticate each one
manually and separately inside that workspace. Soda does not create or copy
their tokens or configuration.

## Manual image lifecycle

Automatic image updates are disabled. A Linux administrator uses Soda Updates
in Cockpit or native bootc. Check is informational; Update and restart follows
the native source at operation start and lets bootc activate/restart as needed:

```sh
sudo bootc status --json
sudo bootc upgrade --check
sudo bootc upgrade --apply
```

Reconnect and verify the actual booted digest and current accounts/data. A
successful command or disconnection is not boot proof. Digest-pinned systems
need an explicit native `bootc switch` to follow a moving tag; Soda never changes
the configured source implicitly. The selected pre-alpha development policy
requires no signature, release approval, or increasing version. Its OCI-only
publication command remains pending under #61.

Fallback selects an earlier exact architecture-matched Soda digest with native
`bootc switch` and must preserve current mutable state; arbitrary downgrade
compatibility is not established. Direct `bootc rollback` is unsupported because
it may restore historical `/etc` instead of preserving current accounts.
