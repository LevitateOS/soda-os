# Managed listeners

> [!IMPORTANT]
> This document records the listeners of the pre-reset implementation currently
> present in the repository. It is implementation evidence, not the target
> ownership model. Under the accepted
> [architectural reset](architecture-reset.md), derived workspace accounts share
> the host network, projects select non-conflicting ports themselves, Podman is
> optional, and Soda provides no port allocator, proxy, network-namespace
> manager, or container control plane.

Soda and project processes share the host network namespace. Soda keeps its
managed reservations small so ordinary development servers can keep their
usual localhost and forwarded workflows. These are current implementation
values, not permanent product doctrine.

| Owner | Protocol and port | Listener scope |
| --- | --- | --- |
| OpenSSH | TCP 22 | Fedora's listener may bind all host addresses; native nftables permits ingress only on loopback and `tailscale0`. |
| Cockpit | TCP 9090 | The current listener may bind all host addresses; native nftables permits ingress only on loopback and `tailscale0`. |
| Built-in Forgejo | TCP 30000 | Loopback before Tailnet enrollment; the appliance Tailnet IPv4 address after enrollment; native nftables rejects other ingress. |
| Tailscale | UDP 41641 | Fedora's current `tailscaled` default; vendor configuration owns the value. |
| Avahi | UDP 5353 | IPv4 and IPv6 multicast DNS. |

Forgejo's `SSH_PORT = 22` setting generates clone URLs; its embedded SSH server
is disabled, so OpenSSH remains the only TCP 22 listener. `sodad`,
`soda-authd`, and the local Tailscale API use Unix sockets rather than network
ports. Soda enables Fedora's native `nftables.service` with a fixed rule that
accepts TCP 22, 9090, and 30000 on loopback and `tailscale0`, rejects those
ports on other ingress, and leaves all other traffic and project-selected ports
under ordinary Linux and project ownership. Soda installs no additional
SELinux port policy or firewall daemon.

A project may use the same numeric port on a non-overlapping specific address,
but a wildcard bind conflicts with any existing listener for that protocol and
port. The first process to bind wins. When a development server reports
`address already in use`, inspect listeners with `ss -ltnup`; administrators
can inspect the corresponding managed service with `systemctl status` and
`journalctl -u SERVICE`.

The QEMU acceptance runner is separate from the appliance contract. By default
it temporarily forwards host TCP 2222 to guest SSH 22 and host TCP 9090 to
guest Cockpit 9090; both host-side values are configurable for concurrent or
conflicting local runs.
