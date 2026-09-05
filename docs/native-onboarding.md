# Native installation and onboarding

ISO installation uses stock graphical Anaconda for storage, networking, bootc
deployment, Linux user creation, and administrator selection. Root stays locked.
Reboot and log in normally; the welcome message shows connection details. ISO installation creates
`/etc/cloud/cloud-init.disabled`, preventing cloud-init from altering those accounts.

QCOW2 deployments use standard Fedora cloud-init delivered by VM tooling. Supply
the Linux account, personal SSH public key, optional password hash, and optional
network configuration through user-data. No Soda checkout or manually built
credential ISO is required. A key alone enables SSH authentication; it does not
supply a password for console, Cockpit, PAM, or password-based sudo.

Interactive local and SSH shells always show a concise welcome with the native
hostname, local Cockpit and Forgejo URLs, a current-user SSH command, and current
Tailscale status. It has no completion or dismissal state. Administrators can
customize the native `/etc/profile.d/soda-console-welcome.sh` entry point.
Non-interactive commands, SCP, and SFTP keep their ordinary output.

Soda runs on a trusted network. Firewalld is installed but disabled by default,
not masked. Administrators can enable and configure it through stock Cockpit's
**Networking → Firewall** page. Soda supplies no default-drop override, custom
zone, or connection-selection trust workflow. Enrolling Tailscale does not
change LAN access or an administrator's firewall choices.

Tailscale is preinstalled and tailscaled is enabled, initially unenrolled.
Administrators sign in through native browser authentication on the separate
**Cockpit → Tailscale** page. The page shows connection state, this device's
name and addresses, visible peers, eligible exit nodes, the native LAN-access
setting for exit-node use, exit-node advertisement and approval, and a link to
the official CLI documentation. It stores no authentication key or workflow state.

The page reads native state on opening and while active. Authentication URLs
are shown as soon as the native process emits them, before authentication
completes. Native status owns pending authentication and approval across page
loads. Closing the page closes its processes; it does not log out the machine.

When the page observes a connected machine, it invokes the existing
`/usr/libexec/soda/forgejo-init refresh-tailnet` command. That command compares
DOMAIN, SSH_DOMAIN, and ROOT_URL against the native reachable Tailnet identity.
Matching values cause no writes or restart. Stale values cause the existing
Forgejo service restart; its inactive oneshot initializer applies the address
before the replacement process starts. The native initializer also runs when
Forgejo starts. Enrollment success and refresh failure are reported separately.
There is no watcher or durable recovery state.

The owner registers the first Forgejo account through the normal trusted LAN
or Tailnet before teammates sign in. Native first-user signup grants Forgejo
administration. Use independent Forgejo credentials, even with the same username
as the Linux owner. PAM remains active. Later Linux users' first successful PAM
login creates ordinary Forgejo accounts. Linux wheel membership grants no
Forgejo role. The team controls ongoing registration policy; there is no
mandatory registration-closing step or associated restart.

| Input | Native owner and purpose |
|---|---|
| Personal SSH public key | Before the first workspace, use **Cockpit → Accounts → your account → Authorized public SSH keys**. Cockpit writes standard authorized_keys; cloud-init may already supply it. |
| Workspace Git key pair | The private key stays in the workspace. Register its public key with Forgejo or the applicable Git host, then retry workspace setup. |
| Repository SSH clone address | Enter the repository address in Cockpit's project catalog. It is an address, not a key. |

Workspace creation still requires personal authorized keys before mutation and
copies them once into the new workspace. There is no Soda key database, key
entry screen, or synchronization. Forgejo owns its account administration and
Git-key registration.

Soda's administrator-only person removal deletes local workspaces first and the
primary Linux account last. It neither inspects nor deletes a same-named
Forgejo account. Forgejo availability, ownership, and deletion restrictions do
not block Linux-person deletion. Delete Forgejo accounts explicitly inside
Forgejo. Linux preflight checks, partial-failure reporting, and generic
non-cascading Cockpit/Linux deletion remain unchanged.

Cloud-init and the VM provider may retain user-data, including secrets. Protect
that input and its retained copies. Removing a temporary enrollment-key file
does not erase cloud-init's instance cache or provider metadata.

## Installed acceptance

After separately authorized builds, run on both matching architectures:

1. Complete graphical Anaconda account creation, reboot, and verify normal login,
   home ownership, administrator privilege, and cloud-init-disabled ISO startup.
2. Provision QCOW2 through VM tooling; check key/password behavior, ordinary LAN
   access, persistence, and mandatory stateless welcome.
3. Start Forgejo before enrolling through the Cockpit Tailscale page. Verify the
   conditional refresh reruns native initialization and advertises the intended
   reachable Tailnet address. After native signup and workspace Git-key
   registration, clone using Forgejo's displayed SSH URL from the intended client.
4. Repeat address, reachability, and clone checks after reboot. Exercise a matching
   address and verify the running Forgejo process remains unchanged.
5. Cover LAN-only provisioning and preserved LAN access after enrollment. Verify
   the complete packaged service graph, including Fedora cloud-init and
   multi-user.target, has no ordering cycle; inspect boot logs for discarded jobs.
6. Verify independent owner credentials, native administrator privileges, and
   later ordinary PAM accounts with self-registration both enabled and disabled
   by team policy. Verify Cockpit key entry, real authorized_keys, one-time
   copying, and incoming workspace SSH.
7. Delete a Linux person through Soda and verify the same-named Forgejo account
   and its data remain. Source tests are not installed-system acceptance.

8. Verify the Tailscale URL appears before sign-in completes; cover native machine
   approval, reauthentication, unavailable daemon, cancellation by leaving the
   page, and reopening with pending or completed native authentication.
9. Exercise exit-node selection and its LAN-access setting. Advertise this machine
   as an exit node, observe pending approval, approve in the native Tailnet admin
   surface, and verify real routed traffic from another device.
10. Verify firewalld starts disabled, not masked; enable/configure it through stock
    Cockpit, reboot and verify the administrator choice persists. Restore the
    fixture's original firewall configuration before unrelated access scenarios.
11. Check actual local login, SSH login and interactive terminal welcome output,
    and unchanged non-interactive SSH, SCP and SFTP output. Confirm explicit LAN
    and Tailnet Cockpit/Forgejo URLs, including MagicDNS-disabled enrollment.
12. Verify an ordinary project-selected listening port from a second LAN machine
    before and after enrollment; a host-forwarded VM check alone is insufficient.
