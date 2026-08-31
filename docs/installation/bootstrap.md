# Initial appliance bootstrap

**Status:** Target installation contract; exact transport and implementation
decisions remain under review in
[#40](https://github.com/LevitateOS/soda-os/issues/40). This is not yet an
operator procedure for the current image.

## Purpose

Bootstrap is the one-time transition from a fresh Soda installation into the
normal Tailnet-only administration model.

Before bootstrap, the machine has no established Linux administrator, trusted
SSH key, selected Cockpit authentication credential, or Tailnet identity. It
cannot require the owner to reach SSH or Cockpit through Tailscale before those
facts exist.

After bootstrap, the machine is administered through ordinary Linux,
OpenSSH, Cockpit, and Tailscale facilities. Soda does not remain an authority
for the administrator account, browser login, SSH session, or Tailnet node.

## Ownership after bootstrap

| Resulting state | Authoritative owner |
| --- | --- |
| Administrator account, password or other PAM credential, and administrator membership | Linux and its supported account facilities |
| Inbound SSH authentication and sessions | OpenSSH |
| Browser authentication and administration session | Cockpit and its supported PAM or authentication boundary |
| Tailnet node identity, reachability, approval, tags, and access policy | Tailscale and Tailnet policy |
| Per-machine input contract and bounded transition | Soda OS installation composition |

Tailscale establishes reachability. It does not replace Linux login, Cockpit
authentication, or process attribution.

## Per-machine input contract

The selected installation transport supplies one per-machine bootstrap input.
The logical input can include:

```text
Linux administrator username
SSH public key
selected initial Cockpit/PAM credential material
one-off Tailscale enrollment credential
bootstrap network configuration when required
optional hostname
```

The exact encoding is not yet selected. Bare-metal installer configuration,
cloud metadata, removable configuration media, and a generated per-machine
bundle may transport the same contract. They must not create different Soda
identity or workflow models.

The Cockpit/PAM field deliberately does not prescribe a Linux password hash,
certificate, or another supported mechanism. Stock Cockpit browser login is a
separate authentication event from SSH public-key login, so #40 must select a
complete initial authentication path.

## Required transition

The selected implementation performs these logical steps:

1. Establish conventional network connectivity sufficient for DNS, time, and
   access to the Tailscale control plane.
2. Create the first ordinary Linux account and grant administrator status
   through the selected Linux administrator signal.
3. Install the administrator's SSH public key.
4. Establish the selected supported Cockpit/PAM authentication method.
5. Enroll the durable appliance in the intended Tailnet with a one-off,
   non-ephemeral credential.
6. Verify appliance-local account, service, listener, and Tailnet readiness.
7. Remove temporary secret input and disable the bounded bootstrap operation.
8. Verify SSH and Cockpit from an authorized Tailnet client.

Whether Tailnet enrollment runs inside the installer or in a bounded first-boot
unit remains open. The observable result and cleanup requirements are the same.

## Secret handling

The raw Tailscale enrollment credential is per-machine, one-off, and
non-ephemeral. Device pre-approval and tags are properties of the selected
Tailnet policy; the installed Soda machine itself is durable.

Bootstrap secrets must not be embedded in or retained by:

- the reusable Soda image or generic installer ISO;
- retained installer input or generated output;
- installation or service logs;
- process command-line arguments; or
- the installed filesystem after successful consumption.

A root-readable temporary file is an acceptable delivery boundary only when
the implementation also proves that the installer, logging, and failure paths
did not copy the secret elsewhere. Deleting one working file is not sufficient
evidence by itself.

If a Linux password is selected for initial Cockpit and privilege
authentication, the cleartext is chosen or generated outside the appliance and
only the required Linux credential representation is supplied. That choice is
not a Soda password database.

## Failure and safe re-execution

Bootstrap crosses Linux and Tailscale boundaries and is not atomic. It reports
the observed result of each step rather than persisting a synthetic transaction
record.

If account creation succeeds but Tailnet enrollment fails:

- the existing Linux account remains authoritative;
- a retry does not create a duplicate account;
- a replacement one-off enrollment credential can be supplied;
- completed steps are inspected before re-execution; and
- the failure does not enable a public onboarding listener.

The final recovery mechanism remains open. Physical-console recovery may be
supported, but the normal installation path cannot require physical access.

## Verification levels

### Appliance-local readiness

The appliance can verify that:

- the administrator account exists and has the selected administrator
  membership;
- the SSH public key is installed with the intended ownership and permissions;
- the selected Cockpit authentication state exists;
- Tailscale reports a durable enrolled node and assigned address;
- OpenSSH and Cockpit are running; and
- listener and firewall policy do not expose Soda services directly to the
  public Internet.

### Client-to-appliance behavior

An authorized Tailnet client must separately demonstrate that:

- it can reach the appliance through Tailscale;
- it can authenticate to OpenSSH as the Linux administrator; and
- it can reach and authenticate to stock Cockpit.

The appliance cannot infer client ACL permission merely from its own local
readiness. Soda does not add a callback service to make these evidence levels
appear atomic.

Both verification levels apply on AArch64 and x86-64. Architecture-specific
installation and validation run on matching native hardware.

## Explicitly absent after completion

A completed bootstrap leaves no:

- Soda bootstrap user record;
- public onboarding endpoint;
- reusable enrollment credential;
- long-running bootstrap daemon;
- durable bootstrap workflow or retry state; or
- bootstrap reconciliation loop.

## Open choices

Issue #40 decides:

- the per-machine input transport for bare-metal, virtualized, and cloud
  installations;
- the initial Cockpit/PAM authentication mechanism;
- user-associated versus tagged Tailscale node identity;
- supported pre-Tailnet network configuration;
- installer-time versus bounded first-boot enrollment; and
- partial-failure retry and recovery behavior.

Once those decisions are recorded and implemented, this document becomes the
operator-facing procedure and may add exact bundle formats, commands, paths,
and recovery instructions verified against the selected upstream versions.

## Upstream references

- [Cockpit authentication](https://docs.cockpit-project.org/cockpit-guide/latest/guide/authentication.html)
- [Tailscale auth keys](https://tailscale.com/docs/features/access-control/auth-keys)
- [`tailscale up`](https://tailscale.com/docs/reference/tailscale-cli/up)
- [Tailscale device tags](https://tailscale.com/docs/features/tags)
- [Anaconda boot options and retained installation data](https://anaconda-installer.readthedocs.io/en/latest/user-guide/boot-options.html)
