# Make the first connection

Finish the shared first-boot checklist and connect to Soda through the trusted LAN or Tailscale.

The same setup appears after an ISO installation and on the first boot of a
QCOW2. It runs on the physical or virtual console and can be reopened through
Cockpit after Cockpit becomes reachable.

## Prerequisites

- Console access to the installed Soda machine.
- A username and strong password for the first administrator.
- The administrator's SSH public key, such as the contents of
  `~/.ssh/id_ed25519.pub` on their client.
- For cloud deployment, a Tailscale auth key and working outbound network.
- For local deployment, either a Tailscale auth key or an explicit decision to
  use LAN-only access.

## Complete first-boot setup

Setup remains available at startup until the machine-wide checklist is
complete. Dismissal is enabled only after all five conditions are satisfied:

1. An administrator account exists.
2. Its password is set.
3. Its SSH public key is installed.
4. The same-named Forgejo administrator is ready.
5. Tailscale is connected, or **LAN-only** is explicitly selected.

Enter the administrator details and public key exactly as prompted. Soda uses
Linux for the account and password, OpenSSH for the public key, and Forgejo for
the repository account.

## Choose the access path

### Tailscale

Create a suitably scoped auth key by following Tailscale's [auth-key
guide](https://tailscale.com/docs/features/access-control/auth-keys) and [device
setup guide](https://tailscale.com/docs/features/access-control/device-management/how-to/set-up).
Enter the key only in first-boot setup. Soda uses it for enrollment once and
removes it immediately afterward.

Confirm the new node in the Tailscale admin console and record its MagicDNS
name. Do not paste the key into shell history, chat, issue trackers, or logs.

### LAN-only

Select **LAN-only** only for a machine on a trusted local network. This allows
setup to complete without Tailscale. An administrator can enroll the machine
later from Cockpit or with native Tailscale administration.

LAN-only is not a supported access mode for cloud deployments.

## Find the machine

For a LAN installation, obtain the address from first-boot setup, the console,
or Cockpit's networking page. For Tailscale, use the confirmed MagicDNS name or
Tailnet address.

| Service | LAN installation | Cloud installation |
|---|---|---|
| OpenSSH, port 22 | Direct LAN or Tailscale | Tailscale only |
| Cockpit, port 9090 | Direct LAN or Tailscale | Tailscale only |
| Forgejo, port 30000 | Direct LAN or Tailscale | Tailscale only |
| Project-selected development port | Direct LAN or Tailscale | Tailscale |

Tailscale never disables direct LAN access on a LAN installation.

## Connect

From the administrator's client:

```sh
ssh ADMINISTRATOR@SODA_HOST
```

Replace `ADMINISTRATOR` and `SODA_HOST` with the values shown during setup.
Open Cockpit at `https://SODA_HOST:9090` and sign in with the same Linux
account. Open Forgejo at `http://SODA_HOST:30000` and sign in with the
same-named administrator account created during setup.

Use the host identity shown by Soda instead of disabling SSH host-key or TLS
warnings. Investigate any unexpected identity change.

## Expected result

The first-boot checklist is complete, the setup screen can be dismissed, SSH
accepts the administrator's public key, Cockpit accepts the Linux login, and
Forgejo recognizes the administrator.

## If something fails

- **SSH:** confirm the username, host, route, and public key installed during
  setup.
- **Cockpit:** confirm port 9090 is reachable on the selected LAN or Tailnet
  path and use the Linux password.
- **Forgejo:** confirm port 30000 and use the Forgejo administrator credentials
  created by setup.
- **Tailscale:** correct the key or outbound networking from the setup screen;
  a failed key is not retained for background retry.

## Next step

Read [Add people and manage access](../30-Develop/10-people-and-access.md).
