# Cockpit: the Soda OS dashboard

Use Cockpit to administer the machine and open Soda's Projects, Runners, Tailscale, and Updates pages.

## Sign in

Open `https://SODA_HOST:9090` over the trusted LAN or Tailnet. Use your primary
Linux username and password. Workspace accounts are for development over SSH,
not dashboard login. Follow [First connection](../20-Deploy/30-first-connection.md)
for initial network access, certificate identity, and public-key setup.

Cockpit reuses Linux authentication. A personal SSH key is not a password and
does not by itself enable browser login. If your account was provisioned with
only a key, use native account administration to set a password.

## Choose the right page

| Page | Use it for |
| --- | --- |
| Overview and Metrics | Host identity, resource use, and capacity investigation |
| Accounts | Primary Linux users, passwords, administrator membership, authorized public SSH keys |
| Services and Logs | Native systemd service state and journal diagnostics |
| Storage | Disks, filesystems, mounts, and capacity |
| Networking → Firewall | Native interfaces and administrator-selected service/port allowances |
| Terminal | Ordinary commands as your signed-in account |
| [Projects](20-projects-and-workspaces.md) | Shared catalog, personal workspaces, reviewed local removal |
| [Runners](50-ci-runners.md) | Administrator-operated local CI clients and listeners |
| [Tailscale](40-tailscale.md) | Native sign-in, private device identity, peers, and optional routing |
| [Soda Updates](60-updates-and-fallback.md) | Verified release discovery/download and explicit restart |

Forgejo is a [separate website](30-forgejo.md) for repositories and collaboration.
Creating a repository there and adding its address to Projects are distinct steps.

## Administrative access

A primary account in Linux `wheel` can enable Cockpit **Administrative access**
when a task needs elevation. Cockpit may ask for the Linux password again.
Being signed in and having administrative access enabled are different facts.

Ordinary primary users can use the project catalog and their own workspace
operations without receiving system-wide administrative privileges. Whole-project
removal, person removal, local runners, and OS updates require administration.
Linux administration does not confer a Forgejo role.

Stock Accounts deletion is a native, non-cascading Linux action. For Soda-aware
person deletion, use [Projects' person-removal procedure](../50-Operate/40-data-safety-and-removal.md#before-removing-a-person).
It reviews and removes the person's local workspaces first.

## Refresh, reconnect, and sign out

A page refresh reads the current native state. It is not permission to retry an
operation that may already have changed files or deployments. If a connection
closes during setup, deletion, or update, reconnect and inspect before acting.
Keep important partial-result details before leaving the page.

Signing out of Cockpit ends your browser session; it does not remove a workspace,
stop development processes, or log the machine out of Tailscale. Coordinate
service stops and server restarts with the people using them.

For general host administration, use the
[stock Cockpit guide](https://cockpit-project.org/guide/latest/).
For Soda-specific symptoms and service endpoints, see
[Administration](../50-Operate/20-administration.md).
