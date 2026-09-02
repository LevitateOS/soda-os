# Managed listeners

Soda OS is operated through a trusted Tailnet. Fedora and the owning upstream
services provide the listeners; Soda only composes their private ingress.

| Owner | Protocol and port | Listener and ingress |
| --- | --- | --- |
| OpenSSH | TCP 22 | Ordinary `sshd`; native nftables permits loopback and `tailscale0` only. |
| Stock Cockpit | TCP 9090 | `cockpit.socket`; native nftables permits loopback and `tailscale0` only. |
| Forgejo | TCP 30000 | Loopback before enrollment and the appliance Tailnet IPv4 afterward; other ingress is rejected. |
| Tailscale | UDP 41641 | Fedora's packaged `tailscaled` default. |

Forgejo advertises SSH clone URLs on port 22; its embedded SSH server is
disabled, so OpenSSH remains the only TCP 22 owner. Soda has no runtime API,
daemon, TCP control listener, or local control socket.

Derived workspace accounts use the same host network namespace as other Linux
users. Projects select non-conflicting host ports themselves and may use
ordinary rootless Podman when useful. Soda does not allocate ports, proxy
traffic, create project network namespaces, or manage containers.

When a process reports `address already in use`, inspect native state with
`ss -ltnup` and the relevant service with `systemctl status` and `journalctl`.
The first process to bind a conflicting address and port wins.

The raw-QEMU test harness may forward host TCP 2222 to guest SSH 22 and host
TCP 19090 to guest Cockpit 9090. Those host-only test forwards are not part of
the appliance network contract.
