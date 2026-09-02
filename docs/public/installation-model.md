Installing Soda OS is the owner's path from a blank machine to a private,
remotely reachable development system. It uses Fedora's familiar graphical
installer while keeping the initial password, SSH key, and network-enrollment
credential on separate removable media.

You will encounter this model only during a fresh installation. Soda OS does
not currently offer an in-place installer or a public installer download.

## Product contract

### What the installer creates

Each installation uses an image built for the machine's architecture. x86-64
and AArch64 are equal targets, and an image for one cannot stand in for the
other.

Soda uses stock graphical **Anaconda**, Fedora's operating-system installer.
Anaconda remains responsible for storage selection, networking, bootloader
setup, image deployment, and creation of the first Linux account.

The completed installation is expected to provide:

- one primary Linux administrator—the human's ordinary account for using the
  machine—with a password and SSH public key;
- `wheel` membership, which is the Linux fact that makes that account an
  administrator;
- one same-named initial administrator in **Forgejo**, the bundled Git hosting
  and collaboration service;
- one attempt to enroll the machine in the owner's Tailnet; and
- private access to OpenSSH and **Cockpit**, Fedora's browser administration
  interface, after enrollment succeeds.

A **Tailnet** is the private network created by Tailscale for the owner's users
and devices. Soda is intended to expose its administration and development
services through that network, not directly to the public Internet.

### The protected OEMDRV medium

Installation requires a second removable image labelled **OEMDRV**. OEMDRV is
the conventional label Anaconda uses to discover installation answers. Soda's
OEMDRV medium is paired with the exact installer image and contains four
values:

- the first administrator's username;
- the first administrator's password;
- that administrator's SSH public key; and
- a one-use Tailscale authentication key.

The OEMDRV medium contains secrets. Keep both its removable copy and its host
copy protected. During installation, Soda reads it without executing content
from it, ejects it, and waits for it to be physically or virtually removed.
Installation does not continue while that secret-bearing medium remains
attached. Destroy the host copy after the ejection has been confirmed.

### What happens during installation

The owner attaches the architecture-matched Soda installer and a newly created
OEMDRV medium, then boots the product installer.

Anaconda presents its normal graphical storage and networking workflow. The
administrator account fields are not entered again in Anaconda because the
protected medium supplies them. Anaconda creates the ordinary Linux account,
adds it to `wheel`, sets its password, and installs the supplied public key in
the standard `~/.ssh/authorized_keys` file.

Before the installed system first boots, a bounded installation step creates
the same-named Forgejo administrator. On first boot, the one-use Tailscale key
is offered to Tailscale once. The temporary key is then deleted and that
enrollment action disables itself whether the attempt succeeds or fails.

No Soda bootstrap account, credential database, public onboarding endpoint,
or background retry workflow remains after installation.

### After the first boot

The owner should expect the new machine to be a normal Tailnet member with an
ordinary Linux administrator. The administrator signs in through SSH with the
installed public key or through Cockpit with the Linux username and password.
The first administrator also uses the same username and initial password for
the initial Forgejo account; Linux and Forgejo manage those accounts
independently afterward.

Continue with [Connect and develop](connect-and-develop.md) for the private
access model and [Administration](administration.md) for adding people.

## Current implementation

There is no public Soda OS tag, downloadable ISO, supported end-user install
command, or production release today. Local unsigned candidate images are
development artifacts, not an end-user installation path.

The current installer uses stock Fedora 44 Anaconda and a protected OEMDRV
medium. Normal installation keeps Anaconda's interactive storage and network
screens. Only the disposable automated test guest uses an unattended,
destructive disk layout.

A fresh native x86-64 installation has verified the protected medium,
mandatory ejection and host-side removal, first Linux administrator, initial
Forgejo administrator, one-use Tailscale enrollment, deletion of the
enrollment credential, stock Cockpit, Projects, direct workspace SSH, and the
installed development tools as one path.

The current protected installer has not yet been repeated and verified on
matching-native AArch64 hardware. Earlier AArch64 evidence for a removed
installer design does not verify this one.

The current implementation has no in-place installer recovery. A failure
before the installed system is complete means correcting the input, creating a
new OEMDRV medium, and performing a fresh installation. If the single
Tailscale enrollment attempt fails, recovery is through native local Tailscale
operation when available or a fresh installation with a new one-use key; Soda
does not retain the credential for a retry.
