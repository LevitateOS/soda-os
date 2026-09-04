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

Workspace accounts share the host network namespace. Projects choose
non-conflicting ports. A developer can send a normal development-server URL to
a teammate and it works over the LAN or Tailnet, including hot reload.

Soda does not allocate ports, track processes or servers, provide a Share
button, proxy project traffic, create project network namespaces, or manage
containers. When a process reports `address already in use`, inspect native
state with `ss`, `systemctl`, and `journalctl`.

## Current implementation

`firewalld` uses `drop` as the default zone, so unassigned normal-network
connections do not admit inbound services. The post-install setup path selects
trusted LAN ingress explicitly with:

```
sudo soda-local-access CONNECTION on
```

This assigns that named NetworkManager connection to firewalld's `trusted`
zone and reactivates it. `soda-local-access CONNECTION off` assigns it to the
`drop` zone again. The command does not infer trust from addresses or select a
connection itself.

Tailscale keeps its normal independent netfilter path. It does not modify the
NetworkManager connection zone, so enrolling Tailscale never narrows a trusted
LAN connection. Forgejo listens on IPv4; firewalld, rather than listener
binding, determines which ingress paths reach it.

Raw-QEMU host port forwards are acceptance-harness details and never product
ingress evidence.
