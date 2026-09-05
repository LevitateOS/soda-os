# Make the first connection

Log in normally after Anaconda installation or native cloud-init provisioning.
The welcome message shows the hostname, local Cockpit and Forgejo URLs, your
SSH command, and current Tailscale status. It appears in interactive local and
SSH sessions, without a dismissal option. An administrator can customize the
message in `/etc/profile.d/soda-console-welcome.sh`.

## Local-network access

SSH, Cockpit, Forgejo and project-selected development ports are available on
the trusted LAN. Firewalld is installed but disabled by default, not masked.
Administrators can enable and configure it through stock Cockpit's
**Networking → Firewall** page.

Open the local Cockpit URL shown in the welcome message. Use the Linux account
created by Anaconda or cloud-init. A supplied SSH key enables SSH authentication;
a password is needed for password-based console, Cockpit and PAM login.

## Tailscale

Tailscale is preinstalled and its daemon runs initially unenrolled. An
administrator opens **Cockpit → Tailscale**, chooses **Sign in**, and follows the
native authentication URL. If device approval is required, the page links to
Tailscale administration. The page shows device identity and addresses, visible
peers, eligible exit nodes, LAN access during exit-node use, and exit-node
advertisement with its approval state.

Tailscale enrollment does not disable LAN access. When using an exit node,
use the native **Allow local network access while using an exit node** setting.
The mandatory welcome includes explicit Cockpit and Forgejo Tailnet links when
connected, using MagicDNS when enabled or a Tailnet IP address.

Projects builds SSH guidance from the hostname used to open Cockpit. Project
listing and workspace creation do not require Tailscale.

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
- **Tailscale:** inspect its native connection or authentication error in Cockpit.

## Next step

Read [Add people and manage access](../30-Develop/10-people-and-access.md).
