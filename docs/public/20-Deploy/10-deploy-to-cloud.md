# Deploy to a cloud or VM

Import the reusable Soda QCOW2, provision a Linux administrator with standard cloud-init, and connect privately.

## Prerequisites

Use an x86-64 or AArch64 instance matching your [verified QCOW2](05-verify-downloads.md).
The platform must import that disk format, supply native cloud-init user-data,
and provide a usable graphical or serial console independently of SSH.
Allocate CPU, memory, and disk for the team's workloads and retained data.

Have a personal SSH public key, an administrator password hash, and a Tailnet
account for cloud access. The hash enables console, Cockpit, PAM, and password-
based sudo login; a public key alone enables only SSH authentication.
No Soda checkout, manually built credential ISO, or public-SSH bootstrap is needed.

## Protect the network before boot

For a cloud instance, configure its security group before first boot: deny
public inbound access to all Soda and development services. Keep outbound access
for DNS, Tailscale, Git hosts, dependencies, and OS images, and allow responses
to outbound connections through the provider's native stateful filtering.

Do not open public SSH or Cockpit while setting up Tailscale. The provider console
is the initial access path. Keep this network boundary after enrollment; a host
firewall allowance for Cockpit is not permission to open provider public ingress.

For a local VM, use a trusted LAN route or enroll Tailscale from its console.
Consult the VM platform for bridging and guest networking; host-only connectivity
is not access from another client. See [the service reference](../50-Operate/20-administration.md#service-endpoints).

## Import the disk

1. Verify the compressed image and signed record, decompress it, and check the
   raw QCOW2 hash as described in [Verify downloads](05-verify-downloads.md).
2. Import the QCOW2 as the boot disk. Enlarge it to the capacity needed for your
   workspaces and data; Soda grows its final root partition and filesystem.
3. Select native firmware and machine/device settings supported by the provider
   for that architecture, and retain console access.
4. Supply cloud-init user-data **before the first boot**. Use the provider's
   user-data facility; local virt-install can deliver it through its native
   cloud-init option. Do not manually construct a credential disk.

## Provision the administrator

Prepare a protected user-data file with the native cloud-init format. Replace
both placeholders with your public key and password hash; do not supply your
private key or plaintext password:

```yaml
#cloud-config
users:
  - name: owner
    groups: [wheel]
    shell: /bin/bash
    lock_passwd: false
    hashed_passwd: '$6$REPLACE_WITH_YOUR_PASSWORD_HASH'
    ssh_authorized_keys:
      - ssh-ed25519 REPLACE_WITH_YOUR_PERSONAL_PUBLIC_KEY
disable_root: true
ssh_pwauth: false
```

Use the native password-hash generation described by
[cloud-init's users/password examples](https://cloudinit.readthedocs.io/en/latest/reference/examples.html).
Enter a password through a protected prompt, not command arguments or recorded
terminal output. `ssh_pwauth: false` keeps SSH key-based; it does not disable
console, Cockpit, or Forgejo PAM password login.

Protect the file and provider metadata. Cloud-init and the provider may retain
user-data, including password hashes; deleting your local file does not erase
those copies. Apply the provider's native retention/access controls.

## Deploy on Scaleway

Within your Scaleway project, use Instances, Object Storage, security groups,
and an authenticated Scaleway CLI:

1. Choose an Instance type with the matching architecture and an Availability Zone.
   Upload the verified **decompressed `.qcow2`** to an Object Storage bucket in
   the corresponding region.
2. Import it as a snapshot in that zone using Scaleway's
   [snapshot import procedure](https://www.scaleway.com/en/docs/instances/how-to/snapshot-import-export-feature/)
   and [Block Storage CLI guidance](https://www.scaleway.com/en/docs/instances/api-cli/managing-instance-snapshot-via-cli/).
   Wait for import completion; select sufficient volume capacity.
3. Create a dedicated [stateful security group](https://www.scaleway.com/en/docs/instances/how-to/use-security-groups/)
   with inbound drop and outbound access. Add no public Soda service rules.
4. Use [Instance creation](https://cli.scaleway.com/instance/#create-server) with
   `stopped=true`, the selected snapshot/root volume, matching instance type,
   local disk boot, and the dedicated `security-group-id`. Confirm boot storage
   and security group before starting.
5. Supply your protected cloud-init user-data through Scaleway's native user-data
   facility, then start the Instance. Open its
   [serial console](https://www.scaleway.com/en/docs/instances/how-to/use-serial-console/)
   and log in as the cloud-init-created administrator.

Provider tools own exact snapshot/storage parameters and user-data delivery.
Keep the console available rather than replacing a failed import/provisioning
step with public SSH.

## Establish the first private connection

Tailscale initially runs unenrolled. At the provider/VM console, follow
[Tailscale's initial console sign-in](../30-Use-Soda/40-tailscale.md#initial-cloud-connection).
Complete its browser authentication from your client and any Tailnet device
approval. This step does not depend on Cockpit already being reachable.

Join your client to the permitted Tailnet, then open Cockpit through the server's
Tailnet address. Continue with [First connection](30-first-connection.md).
Use Cockpit's Tailscale page for subsequent device/routing management and its
Forgejo address-refresh result. Keep public inbound service access closed.

## Check before creating project data

Confirm normal console login and welcome, the expected architecture, enlarged
disk/filesystem capacity in Cockpit Storage, private SSH/Cockpit access, and
public service rejection. If cloud-init did not create a usable account, inspect
its native console diagnostics and provider user-data delivery; do not assume
that editing user-data after boot reruns account provisioning.

For failed imports or boot, check disk format, architecture, firmware, and boot
attachment. For private access errors, use the native Tailscale diagnostic and
[administration troubleshooting](../50-Operate/20-administration.md#troubleshooting).
