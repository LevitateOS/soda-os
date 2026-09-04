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
Log in normally before temporary network-only Soda Setup. Add a personal SSH
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

Automatic image updates are disabled. A Linux administrator selects an exact
signed Soda image through native bootc operations:

```sh
sudo bootc status
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
sudo bootc status
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Supported fallback repeats the sequence with the previous signed Soda image
digest. Direct `bootc rollback` is unsupported because it may restore the
earlier deployment's historical `/etc` instead of preserving current accounts.
