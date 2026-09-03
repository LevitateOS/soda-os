Soda OS uses stock Cockpit as its browser administration surface. Cockpit owns
authentication, sessions, and general Linux administration. Soda adds focused
pages for the small number of product-specific journeys, including **Projects**.

Remote administration is SSH-first. Administrators can use ordinary Linux
commands over OpenSSH whenever the stock host interface is the clearer tool.

## Sign in as an administrator

An administrator is a primary Linux account in `wheel`. Connect through the
Tailnet with either:

```sh
ssh <administrator>@<soda-hostname>
```

or Cockpit at:

```text
https://<soda-hostname>:9090
```

Use `id` to confirm `wheel` membership before performing an administrative
operation. Linux group membership is the authority; Soda does not keep a
parallel role database.

## Use the right administrative owner

| Task | Where to perform it |
| --- | --- |
| Add a complete Soda person | **Add person…** on **Projects** |
| Grant or revoke host administration | Cockpit **Accounts** or native Linux `wheel` management |
| Add, edit, set up, or remove a project | **Projects** |
| Manage Forgejo repositories, collaborators, and roles | Forgejo |
| Manage Tailnet devices and access policy | Tailscale |
| Inspect services, storage, networking, and logs | Stock Cockpit or native Linux tools |
| Select an operating-system image | Native bootc commands |

When workflow execution should run on the Soda machine, follow
[CI runners](ci-runners.md).

## Add and administer people

Use **Add person…** for initial onboarding because it creates the person's
Linux, SSH, Forgejo, and Tea result together. Afterward, use stock Cockpit or
ordinary Linux tools for password changes and `wheel` membership.

Forgejo administrator status is separate from Linux administrator status. Use
Forgejo's own administration interface when a person needs a Forgejo role.

Before removing someone, follow [Data safety and removal](data-safety-and-removal.md).
The supported **Remove person…** action deletes that person's local workspaces
before deleting the primary Linux account last.

## Keep managed services private

The managed entry points are:

| Service | Port | Intended ingress |
| --- | ---: | --- |
| OpenSSH | 22 | Loopback and the Tailnet |
| Stock Cockpit | 9090 | Loopback and the Tailnet |
| Forgejo | 30000 | Loopback and the Tailnet |

Tailscale owns device membership and Tailnet access policy. Keep cloud security
groups, router forwarding, and host policy aligned with this private access
model.

## Diagnose with native tools

Start with the system that owns the failing behavior:

```sh
systemctl --failed
systemctl status <unit>
journalctl --boot --unit <unit>
ss -ltnup
df -h
bootc status
```

Use Cockpit's stock pages for services, logs, storage, networking, and terminal
access. Use Forgejo for repository and collaboration failures, Tailscale for
private reachability, and OpenSSH diagnostics for remote-session failures.

## Back up mutable state

Operating-system image changes preserve machine-specific state, but they are
not backups. Back up the data the team cannot reconstruct, including:

- unpushed workspace work and project-local data;
- Forgejo repositories and mutable Forgejo state;
- the Project catalog;
- home-directory configuration and keys; and
- other service data created by the team.

Test that backups can be restored without depending on the running machine.
Continue with [Updates and fallback](updates-and-fallback.md) for image changes.
