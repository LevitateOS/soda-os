# Managed listeners

## Product contract

Soda is cloud-first, not cloud-only. Fedora and the owning upstream services
provide the listeners; Soda composes the accepted ingress.

| Owner | Protocol and port | Trusted LAN | Cloud |
| --- | --- | --- | --- |
| OpenSSH | TCP 22 | Directly reachable | Tailscale only; never public Internet |
| Stock Cockpit | TCP 9090 | Directly reachable | Tailscale only; never public Internet |
| Forgejo | TCP 30000 | Directly reachable | Tailscale only; never public Internet |
| Tailscale | Fedora package defaults | Optional | Private reachability owner |

Forgejo advertises SSH clone URLs through OpenSSH; its embedded SSH server
remains disabled. Soda has no runtime API, daemon, proxy, TCP control listener,
or local control socket.

At initialization Forgejo uses the machine's static hostname for its advertised
HTTP and SSH domains. When a Tailnet identity is available, initialization uses
its MagicDNS name when enabled, or its Tailnet IPv4 address instead. Native maintenance and acceptance calls from the host use
Forgejo's loopback listener; the Projects list does not publish or select a
Forgejo endpoint.

Workspace accounts share the host network namespace. Projects choose
non-conflicting ports. A developer can send a normal development-server URL to
a teammate and it works over the LAN or Tailnet, including hot reload.

Soda does not allocate ports, track processes or servers, provide a Share
button, proxy project traffic, create project network namespaces, or manage
containers. When a process reports `address already in use`, inspect native
state with `ss`, `systemctl`, and `journalctl`.

## Current implementation

Firewalld is installed and disabled by default, not masked. Administrators may
enable and configure it through stock Cockpit **Networking → Firewall**.
There is no Soda default zone override, custom Tailnet zone, selected-network
trust action, or service that changes an administrator's firewall choices.
Tailscale keeps its native netfilter behavior. Forgejo listens on IPv4 on LAN
and Tailnet addresses. Services and project-selected ports work directly on the
trusted network while the host firewall is disabled.

The separate **Tailscale** page provides native browser sign-in, state, device
addresses, visible peers and native exit-node settings. The native **Allow local
network access while using an exit node** preference controls LAN routing during
exit-node use. Linux forwarding is enabled through native sysctl configuration
for exit-node advertisement; the Tailnet administrator owns approval.

Raw-QEMU host port forwards are acceptance-harness details and never product
ingress evidence.

Projects list and setup operations are independent of Tailscale enrollment.
The Cockpit browser derives SSH guidance from the hostname used to open the
page, preserving the owner's selected LAN or Tailnet route.
