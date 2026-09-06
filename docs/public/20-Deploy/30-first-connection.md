# Make the first connection

Log in normally after Anaconda installation or native cloud-init provisioning.
The welcome message shows the hostname, local Cockpit and Forgejo URLs, your
SSH command, and current Tailscale status. It appears in interactive local and
SSH sessions, without a dismissal option. An administrator can customize the
message in `/etc/profile.d/soda-console-welcome.sh`.

## Local-network access

SSH retains the Anaconda/Fedora allowance, and Soda allows Cockpit TCP 9090.
Firewalld remains enabled with native defaults. Administrators must open Forgejo
TCP 30000/2222 and project-selected development ports for LAN access through
stock Cockpit's **Networking → Firewall** page.

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
account. Open Forgejo at `http://SODA_HOST:30000` and sign in with your Linux
username and password. PAM creates an ordinary Forgejo account on first login.

Use the host identity shown by Soda instead of disabling SSH host-key or TLS
warnings. Investigate any unexpected identity change.

## Forgejo administration is explicit

The first human uses the same Linux/PAM sign-in as everyone else and does not
become a Forgejo administrator automatically. Other users can sign in before
any Forgejo administrator exists.

When site administration is needed, a Linux administrator can
[create a separate Forgejo administrator using the native CLI](../40-Operate-Soda-OS/10-administration.md#create-a-forgejo-administrator).
That account has its own Forgejo password. It can then promote existing PAM
users through Forgejo's web interface. Linux `wheel` membership alone grants no
Forgejo role. There is no first-owner registration step.

## Expected result

Network access works, SSH accepts the installed personal key, Cockpit accepts
the Linux password, and Forgejo accepts Linux/PAM login as an ordinary user.

## If something fails

- **SSH:** confirm the username, host, route, and public key in the Linux account authorized_keys.
- **Cockpit:** confirm port 9090 is reachable on the selected LAN or Tailnet
  path and use the Linux password.
- **Forgejo:** confirm port 30000 and use your Linux credentials for a PAM account.
  Use the separate Forgejo password only for an explicitly CLI-created account.
- **Tailscale:** inspect its native connection or authentication error in Cockpit.

## Next step

Read [Add people and manage access](../30-Develop/10-people-and-access.md).
