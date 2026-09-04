# Administration

Operate Soda through stock Cockpit and the native Linux, Forgejo, Tailscale, OpenSSH, and bootc boundaries.

## Prerequisites

- Use a primary Linux account in `wheel`.
- Connect through the trusted LAN or Tailscale, never a public cloud ingress
  rule.
- Keep current backups of data that cannot be reconstructed from Git.

## Use stock Cockpit

Open `https://SODA_HOST:9090` and sign in with the primary Linux account.
Cockpit provides the normal host pages for overview, metrics, services, logs,
accounts, terminal, storage, and networking. The Soda **Projects** page is the
focused interface for the shared catalog and workspace lifecycle.
The administrator-only **Runners** page operates local Forgejo and GitHub
runner capacity; each provider owns workflows, scheduling, and job history.

Use the [Cockpit guide](https://docs.cockpit-project.org/cockpit-guide/latest/)
for general Linux administration. Soda does not duplicate Cockpit's host
status, user listing, service, storage, or networking models.

## Know the native owner

| Task | Use |
|---|---|
| Add or inspect Linux users | Cockpit **Accounts** and native Linux tools |
| Grant administrator capability | Add the primary account to `wheel` in Cockpit |
| Manage repositories and collaboration | Forgejo or the external Git host |
| Manage registered repository SSH keys | Forgejo |
| Inspect or enroll Tailscale | Tailscale administration |
| Inspect SSH service and keys | OpenSSH and systemd |
| Add projects or manage workspaces | Cockpit **Projects** |
| Operate local CI runners | Cockpit **Runners** and the provider's native registration settings |
| Select and deploy OS images | bootc |

## Service endpoints

| Service | Port | LAN installation | Cloud installation |
|---|---:|---|---|
| OpenSSH | 22 | LAN or Tailscale | Tailscale only |
| Cockpit | 9090 | LAN or Tailscale | Tailscale only |
| Forgejo | 30000 | LAN or Tailscale | Tailscale only |
| Development server | Project-selected | LAN or Tailscale | Tailscale only |

Do not expose these ports to the public Internet on a cloud machine.

## Configure Tailscale later

A machine configured with **Allow access from the local network** can join
Tailscale later. Open Soda Setup from Cockpit and complete its Tailscale step,
or use the native
Tailscale administration path. Follow Tailscale's [device setup
guide](https://tailscale.com/docs/features/access-control/device-management/how-to/set-up).
The auth key is used once and removed. Tailscale enrollment does not disable
the trusted local-network connection.

## Native diagnostics

Use ordinary Linux tools so the output corresponds to the authoritative
service:

```sh
systemctl --failed
journalctl -b -p warning
ip address
ss --listening --tcp --numeric
tailscale status
bootc status
```

Use `systemctl status sshd`, `systemctl status cockpit.socket`,
`systemctl status forgejo`, or the corresponding journal when one service has
a problem.

## Back up what Git does not protect

Back up at least:

- Forgejo repositories and database state;
- the Soda project catalog;
- workspace files that have not been committed and pushed;
- primary and workspace homes containing irreplaceable user data;
- Tailscale and SSH host identity needed to preserve service identity;
- any application data maintained by development services.

An image update or fallback is not a backup. Git protects only content that was
committed and successfully pushed to a repository that is itself protected.

## Expected result

Every administrative fact can be inspected through its native owner, while
Soda-specific administration stays limited to Soda Setup, Projects, local
Runners, and their focused lifecycle actions.

## If something fails

Use the owning service's status and journal first. Do not create parallel Soda
state to compensate for a native error. For a partial destructive operation,
follow [Data safety and removal](30-data-safety-and-removal.md).

## Next step

Read [Updates and fallback](20-updates-and-fallback.md).
