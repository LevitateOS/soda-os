# Make the first connection

Complete Soda Setup and connect through the trusted local network, Tailscale, or both.

**Soda Setup** appears after an ISO installation and on the first boot of a
QCOW2. It runs on the physical or virtual console and is also available through
Cockpit. Both surfaces use the same setup state and underlying operations.
Soda Setup can always be opened again later.

## Prerequisites

- Console access to the installed Soda machine.
- A username and strong password for the first administrator.
- The administrator's SSH public key, such as the contents of
  `~/.ssh/id_ed25519.pub` on their client.
- For cloud deployment, a Tailscale auth key and working outbound network.
- For local deployment, either a Tailscale auth key or a trusted current local
  network connection.

## Complete Soda Setup

Soda Setup remains available at startup until the machine-wide checklist is
complete and you dismiss it. Dismissal is enabled only after all five
conditions are satisfied:

1. An administrator account exists.
2. Its password is set.
3. Its SSH public key is installed.
4. The same-named Forgejo administrator is ready.
5. Tailscale is connected, or **Allow access from the local network** is
   selected for the current connection.

Enter the administrator details and public key exactly as prompted. Soda uses
Linux for the account and password, OpenSSH for the public key, and Forgejo for
the repository account.

## Choose the access path

### Tailscale

Create a suitably scoped auth key by following Tailscale's [auth-key
guide](https://tailscale.com/docs/features/access-control/auth-keys) and [device
setup guide](https://tailscale.com/docs/features/access-control/device-management/how-to/set-up).
Enter the key only in Soda Setup. Soda uses it for enrollment once and
removes it immediately afterward.

Confirm the new node in the Tailscale admin console and record its MagicDNS
name. Do not paste the key into shell history, chat, issue trackers, or logs.

### Local-network access

Select **Allow access from the local network** only when you trust the current
local network connection. This explicitly allows Soda services over that
connection and lets Soda Setup complete without Tailscale.

Tailscale enrollment is a separate choice. Enabling Tailscale never disables
local-network access, and an administrator can enroll the machine later by
reopening Soda Setup in Cockpit or by using native Tailscale administration.
Cloud deployments use Tailscale rather than trusting a public network
connection.

## Find the machine

For a LAN installation, obtain the address from Soda Setup, the console,
or Cockpit's networking page. For Tailscale, use the confirmed MagicDNS name or
Tailnet address.

Open Cockpit with the LAN hostname or Tailnet name that developers should use.
The Projects page builds its SSH guidance from that browser hostname; it does
not choose or require a Tailscale identity. Before Tailnet enrollment, bundled
Forgejo advertises the machine's static hostname. After enrollment, restarting
Forgejo lets it advertise the Tailnet identity while the LAN path remains
available.

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

The Soda Setup checklist is complete, the setup screen can be dismissed, SSH
accepts the administrator's public key, Cockpit accepts the Linux login, and
Forgejo recognizes the administrator.

## If something fails

- **SSH:** confirm the username, host, route, and public key installed during
  setup.
- **Cockpit:** confirm port 9090 is reachable on the selected LAN or Tailnet
  path and use the Linux password.
- **Forgejo:** confirm port 30000 and use the Forgejo administrator credentials
  created by setup.
- **Tailscale:** correct the key or outbound networking from Soda Setup;
  a failed key is not retained for background retry.

## Next step

Read [Add people and manage access](../30-Develop/10-people-and-access.md).
