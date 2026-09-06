# Administration and troubleshooting

Operate the shared host through stock Cockpit and native Linux tools, with deliberate access and maintenance choices.

Use your primary Linux account. Enable Cockpit administrative access or use
sudo for privileged tasks. Ordinary developer accounts should not receive
system privileges just to run a project.

## Service endpoints

`SODA_HOST` is the server's reachable trusted-LAN or Tailnet hostname/address.
Use this reference when configuring clients and host firewall allowances:

| Service | Endpoint | Access policy |
| --- | --- | --- |
| Human/workspace SSH, SCP, SFTP | TCP 22, `USER@SODA_HOST` | Native SSH firewall allowance |
| Cockpit | TCP 9090, `https://SODA_HOST:9090` | Soda's additional host firewall allowance |
| Forgejo website | TCP 30000, `http://SODA_HOST:30000` | Administrator opens host port for LAN access |
| Forgejo Git over SSH | TCP 22; copy the repository's SSH URL | Native OpenSSH integration, not a second embedded SSH server |
| Development server | Project-selected free port | Administrator opens direct access when needed; SSH forwarding is an alternative |

For trusted LAN access, go to **Cockpit → Networking → Firewall**, inspect the
zone assigned to the relevant interface, and add the intended service/port there.
Preserve existing administrator rules and Fedora's enabled firewall defaults.
Do not broadly trust every network or expose all development ports.

For cloud installations, keep provider public ingress to these services closed
and use Tailscale. Host firewall rules and provider security groups are separate
boundaries. Enrollment does not disable LAN access; exit-node LAN preferences
are explained in [Tailscale](../30-Use-Soda/40-tailscale.md).

## Routine checks

- **Overview/Metrics:** investigate CPU, memory, and I/O pressure before promising
  additional workspace or runner capacity.
- **Storage:** watch root/home and project-data capacity; inspect large caches,
  container volumes, and logs with their owning users before removal.
- **Services/Logs:** inspect a service's state and journal before restarting it.
- **Accounts:** maintain primary credentials and administrator membership. Use
  Soda's [person-removal path](40-data-safety-and-removal.md#before-removing-a-person)
  for cascading local deletion.
- **Updates:** coordinate explicit [download/apply/restart](../30-Use-Soda/60-updates-and-fallback.md).
- **Backups:** maintain separate copies and [test restoration](30-backups-and-restoration.md).

Useful read-only native diagnostics include:

```sh
bootc status --verbose
systemctl --failed
systemctl status cockpit.socket tailscaled.service forgejo.service
journalctl -u forgejo.service -n 100 --no-pager
tailscale status
sudo ss -lntp
```

Use Cockpit's native service controls or `systemctl` for a justified restart;
warn connected users first. OS-managed tools come from the image. Project tools
and configuration belong to [mise](../40-Develop/10-connect-and-develop.md#manage-development-tools),
not a second administrator tool installer. Administrators can customize native
interactive welcome guidance in `/etc/profile.d/soda-console-welcome.sh`.

## Troubleshooting

| Symptom | Check first |
| --- | --- |
| No Cockpit route | Correct LAN/Tailnet address, client route, native listener, selected host firewall zone; on cloud use the console for Tailscale bootstrap |
| Cockpit rejects login | Primary username and Linux password, not a workspace/token; SSH keys alone do not supply a password |
| Privileged page is unavailable | Linux administrator membership and Cockpit administrative access |
| TLS or SSH identity changed | Verify the intended host through console/admin evidence; do not globally disable checks |
| SSH key rejected | Exact workspace username, chosen client key, and that account's authorized keys; primary-key changes are not synchronized |
| Projects shows an account but no confirmed setup | Run its read-only inspection, then fix the reported key/clone issue before explicit retry |
| Git clone fails | Copy the host's actual SSH URL, check host identity, workspace key registration, repository permissions, and network reachability |
| Forgejo login works but has no admin | Expected for PAM; grant native Forgejo administration explicitly |
| Forgejo link uses the wrong address | Inspect Tailscale's separate Forgejo refresh result and native config/service; successful enrollment is not successful refresh |
| Development server unreachable | Its bind address and port, collisions, listener, allowed route/firewall, or SSH forward |
| Runner listens but receives no jobs | Provider registration, labels, workflow rules, and provider history |
| Update response or connection is lost | Inspect bootc booted/pending state before another command or restart |
| Removal reports failure | Preserve partial-result details, inspect remaining native state, then review scope again before confirming |

Use the owning public guide for the next action:
[Projects](../30-Use-Soda/20-projects-and-workspaces.md),
[Forgejo](../30-Use-Soda/30-forgejo.md),
[Runners](../30-Use-Soda/50-ci-runners.md),
[Tailscale](../30-Use-Soda/40-tailscale.md), or
[Updates](../30-Use-Soda/60-updates-and-fallback.md).

## Report a useful problem

Record the OS version and architecture, exact non-secret action, expected result,
observed diagnostic, timestamp, and relevant native service state. Distinguish a
page error from a command failure or lost response. Remove passwords, private
keys, tokens, personal/repository data, and authentication URLs before sharing
logs or screenshots. Never reset application data just to simplify a report.
