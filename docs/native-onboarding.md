# Native installation and onboarding

ISO installation uses stock graphical Anaconda for storage, networking, bootc
deployment, Linux user creation, and administrator selection. Root stays locked.
Reboot and log in normally before Soda Setup appears. ISO installation creates
`/etc/cloud/cloud-init.disabled`, preventing cloud-init from altering those accounts.

QCOW2 deployments use standard Fedora cloud-init delivered by VM tooling. Supply
the Linux account, personal SSH public key, optional password hash, and optional
network configuration through user-data. No Soda checkout or manually built
credential ISO is required. A key alone enables SSH authentication; it does not
supply a password for console, Cockpit, PAM, or password-based sudo.

Soda Setup is temporary and handles only missing network trust or Tailscale
enrollment. It starts after an authenticated administrator logs in on an
interactive machine console and uses ordinary privilege elevation. Cancellation
or failure returns to the logged-in shell. SSH, SCP, and SFTP do not launch it.
Native network readiness skips automatic Setup. Administrators can reopen it
with `sudo /usr/libexec/soda/soda-setup console` or through Cockpit.

Default-drop protection remains in place. Explicitly trust only a trusted LAN
connection; cloud services remain private through Tailscale. Tailscale never
disables the existing trusted-LAN path.

After successful cloud-init Tailscale enrollment and verification, invoke
`/usr/libexec/soda/forgejo-init refresh-tailnet`. Interactive Setup uses this same
entry point. It compares DOMAIN, SSH_DOMAIN, and ROOT_URL against the native
Tailnet identity. Matching values cause no writes or restart. Stale values cause
the existing Forgejo service restart; its inactive oneshot initializer applies
the address before the replacement process starts. Forgejo never waits for
cloud-final. Report enrollment success separately from refresh failure.

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
2. Provision QCOW2 through VM tooling; check key/password behavior, network
   protection, persistence, and skipping unnecessary interactive Setup.
3. Start Forgejo before cloud-init finishes Tailscale enrollment. Verify the
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
