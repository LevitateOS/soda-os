Connect to Soda OS from a laptop or other client that is already authorized on
the same Tailnet. The first administrator can use OpenSSH for the command line,
stock Cockpit for browser administration, and Forgejo for Git hosting and
collaboration.

## Before you begin

Confirm that:

- the cloud deployment or on-premises installation completed;
- the client device is signed in to the intended Tailnet;
- the administrator has the private key matching the public key supplied
  during provisioning; and
- Tailnet policy permits the client to reach the Soda machine.

Use Tailscale's device list or MagicDNS to find the machine's private name.
**MagicDNS** is Tailscale's DNS service for reaching Tailnet devices by name.

## Connect with SSH

Open an ordinary SSH session with the initial administrator username:

```sh
ssh <administrator>@<soda-hostname>
```

The session is handled by OpenSSH and runs as the real Linux account. Confirm
the identity and administrator group:

```sh
id
```

The result names the administrator and includes `wheel` among its groups.

## Open Cockpit

In a browser on the authorized client, open:

```text
https://<soda-hostname>:9090
```

Sign in with the initial Linux username and password. Cockpit provides the
stock Linux administration experience with Soda branding and focused Soda
pages. Open **Projects** and confirm that its Project catalog loads.

## Open Forgejo

Open the bundled Forgejo service on the Soda machine's Tailnet name and port
`30000`. Sign in with the same initial username and password used during
provisioning.

The first account is the Forgejo site administrator. Linux and Forgejo own
their account state independently after installation, so changing one password
does not change the other.

## Expected result

The administrator can reach all three managed entry points through the
Tailnet:

- OpenSSH on TCP port 22;
- stock Cockpit on TCP port 9090; and
- Forgejo on TCP port 30000.

These services are private appliance interfaces. Keep provider firewalls,
router rules, and Tailnet policy aligned with private Tailnet access.

The machine is now ready for [adding people](people-and-access.md) and
[cataloguing projects](projects-and-workspaces.md).
