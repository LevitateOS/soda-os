# Make the first connection

Complete Soda Setup and connect through the trusted local network, Tailscale, or both.

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
Native network readiness permits explicit dismissal but does not suppress
automatic Setup. It keeps appearing after an administrator console login until
they choose **Don't show Setup automatically**. Administrators can reopen it
with `sudo /usr/libexec/soda/soda-setup console` or through Cockpit.

Default-drop protection remains in place. Explicitly trust only a trusted LAN
connection; cloud services remain private through Tailscale. Tailscale never
disables the existing trusted-LAN path.

After native Tailscale enrollment, run
`sudo /usr/libexec/soda/forgejo-init refresh-tailnet` to apply the address
when needed. Setup performs this step automatically; the cloud-init example
includes it. A refresh failure needs attention even when enrollment succeeded.

## Choose the access path

### Tailscale

Create a suitably scoped auth key by following Tailscale's [auth-key
guide](https://tailscale.com/docs/features/access-control/auth-keys) and [device
setup guide](https://tailscale.com/docs/features/access-control/device-management/how-to/set-up).
Enter the key in Soda Setup or supply protected cloud-init user-data. Soda uses it for enrollment once and
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
Forgejo advertises the machine's static hostname. After enrollment, the conditional native address refresh applies the Tailnet
identity when needed while the LAN path remains available.

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

Replace `ADMINISTRATOR` and `SODA_HOST` with your Linux username and the reachable machine address.
Open Cockpit at `https://SODA_HOST:9090` and sign in with the same Linux
account. Open Forgejo at `http://SODA_HOST:30000` and sign in with the
independent Forgejo account you register yourself.

Use the host identity shown by Soda instead of disabling SSH host-key or TLS
warnings. Investigate any unexpected identity change.

## Register the Forgejo owner first

The owner registers the first Forgejo account through the normal trusted LAN
or Tailnet before teammates sign in. Native first-user signup grants Forgejo
administration. Use independent Forgejo credentials, even with the same username
as the Linux owner. PAM remains active. Later Linux users' first successful PAM
login creates ordinary Forgejo accounts. Linux wheel membership grants no
Forgejo role. The team controls ongoing registration policy; there is no
mandatory registration-closing step or associated restart.

## Expected result

Network access works, SSH accepts the installed personal key, Cockpit accepts
the Linux password, and Forgejo recognizes its independent administrator.

## If something fails

- **SSH:** confirm the username, host, route, and public key in the Linux account authorized_keys.
- **Cockpit:** confirm port 9090 is reachable on the selected LAN or Tailnet
  path and use the Linux password.
- **Forgejo:** confirm port 30000 and use the Forgejo administrator credentials
  chosen during native signup.
- **Tailscale:** correct the key or outbound networking from Soda Setup;
  a failed key is not retained for background retry.

## Next step

Read [Add people and manage access](../30-Develop/10-people-and-access.md).
