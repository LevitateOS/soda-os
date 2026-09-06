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
account. Open Forgejo at `http://SODA_HOST:30000` and complete the first-owner
registration below before using it for your team.

Use the host identity shown by Soda instead of disabling SSH host-key or TLS
warnings. Investigate any unexpected identity change.

## Register the Forgejo owner first

**The first owner is the exception to normal Linux/PAM sign-in.** Register a
separate Forgejo account; do not use Linux sign-in to establish ownership.

1. Open `http://SODA_HOST:30000` through the trusted LAN or Tailnet. An empty
   instance shows **Create your Forgejo administrator account**.
2. Choose **Create administrator account** and complete Forgejo's native
   registration form. You may reuse your Linux username, but choose independent
   Forgejo credentials. Soda does not copy your Linux password into this account.
3. After successful registration, the administrator confirmation leads to
   **Site Administration**. You can also open `/admin` or use your avatar menu.
4. Later humans sign in with their Linux username and password. PAM creates
   ordinary Forgejo accounts for them. No manual PAM activation or restart is
   required, unless an administrator has deliberately disabled that source.

Before owner registration completes, web sign-in leads to registration and
PAM API/Git-over-HTTP authentication cannot create an account. A failed signup
can be corrected and retried without consuming the administrator position.

Your owner account continues using its separate Forgejo password on later
visits. Linux/Cockpit administrator status and `wheel` membership do **not**
grant Forgejo administration. The team controls later self-registration policy;
closing registration is optional and does not disable ordinary PAM sign-in.

### Existing accounts but no administrator

If Forgejo says **This Forgejo instance has no administrator**, it already has
accounts and is not an unclaimed server. Older installations could reach this
state through PAM login before owner registration. An update does not silently
promote an account, replace credentials, or erase the database.

**Registering another account will not repair it.** Ask the Linux administrator
to investigate the account-creation history and arrange explicit recovery after
[backing up the data](../40-Operate-Soda-OS/30-data-safety-and-removal.md).
Resetting Forgejo is not a routine troubleshooting step for a server containing
work. If registration is disabled on an empty server, the Linux administrator
must review its registration settings; Soda does not override that choice.

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
