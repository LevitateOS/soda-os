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

The current nftables composition admits managed TCP services only on loopback
and `tailscale0`. It therefore blocks the approved direct-LAN path. Current
Forgejo endpoint and guidance code also assume Tailnet enrollment.

That Tailnet-only behavior is implementation debt tracked by issue #15. No
finished LAN-access artifact or matching-native LAN evidence is claimed here.

Raw-QEMU host port forwards are acceptance-harness details and never product
ingress evidence.
