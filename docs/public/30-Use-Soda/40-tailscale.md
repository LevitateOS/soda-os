# Tailscale

Connect Soda privately, inspect its Tailnet identity, and manage native device and exit-node settings.

Tailscale is installed and its daemon enabled, initially unenrolled. Your
Tailnet's administrators own users, device approval, access rules, and routing
approval. Soda uses ordinary OpenSSH over that network, not Tailscale SSH.

## Initial cloud connection

Use the provider or VM **console**, not public SSH. Log in as the administrator
created by cloud-init, then run:

```sh
sudo tailscale up
```

Open the authentication URL it prints in your client's browser and sign in to
the intended Tailnet. Complete device approval in
[Tailscale administration](https://login.tailscale.com/admin/machines) if your
Tailnet requires it. This native browser flow needs no pre-created auth key.

At the server console, verify connection and obtain the private address:

```sh
tailscale status
tailscale ip -4
```

Connect your client to the permitted Tailnet, then open
`https://TAILNET_ADDRESS:9090`. Continue with
[First connection](../20-Deploy/30-first-connection.md). Keep the cloud security
group closed to public service ingress. Console access remains your recovery
path if the Tailnet route is unavailable.

## Sign in through Cockpit

For a server already reachable on the trusted LAN or Tailnet:

1. Open **Tailscale** in Cockpit with your primary administrator account.
2. Enable administrative access when prompted and select **Sign in**.
3. Follow the native authentication URL as soon as it appears; complete any
   approval at Tailscale's own administration surface.
4. Confirm the connected device name and addresses on the page.

The page observes native status when opened and while active. Leaving it closes
its observation/authentication processes; it does not log the machine out of
the Tailnet. Reopening reads native pending or completed authentication state.

## Device addresses and Forgejo links

Use the displayed Tailnet name when MagicDNS is enabled, or its Tailnet IP.
Visible peers reflect what Tailscale makes available; their presence alone does
not guarantee that an access rule permits a particular service.

When the page observes a connected machine, it requests Forgejo's native
address refresh. Matching advertised addresses cause no restart. Changed
addresses require the existing Forgejo service restart so its browser/clone
URLs use the connected identity. **Enrollment success and Forgejo refresh
failure are different results.** Use the page's explicit refresh retry after
resolving the Forgejo error rather than enrolling the machine again.

Projects uses the hostname in your Cockpit browser address for its SSH guidance.
LAN-only catalog and workspace setup do not require enrollment.

## Optional exit nodes

An exit node routes Internet-bound traffic through another device. It is not
required for ordinary Tailnet access to Soda.

- To use one, select an eligible exit node and apply the native preference.
- If you still need the connected local LAN while using it, enable **Allow local
  network access while using an exit node**.
- To stop using it, select the no-exit-node option and apply.
- To offer Soda as an exit node, enable advertisement, then obtain approval from
  the Tailnet administrator. Advertising is not the same as approval or verified
  routed traffic; check the page's approval guidance.

Review [Tailscale exit-node documentation](https://tailscale.com/kb/1103/exit-nodes)
before changing routing on a shared server. Coordinate connectivity changes with
other users and keep console access.

## Troubleshooting and leaving the Tailnet

For sign-in, approval, daemon, or routing errors, read the native diagnostic and
check `tailscale status`. Native CLI details are in
[Tailscale's command reference](https://tailscale.com/kb/1080/cli).
Do not open public service ports to bypass an enrollment problem.

Logging out removes private reachability until reauthentication. If you need to
do it, use the native `sudo tailscale logout` from a console or another route you
will retain, and manage the device record in Tailscale administration. Signing
out of Cockpit alone does not log out Tailscale.

Enrollment itself does not disable LAN access or overwrite administrator
firewall choices. See [Administration](../50-Operate/20-administration.md) for
host listeners and LAN firewall configuration.
