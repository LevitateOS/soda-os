# Install on premises

Install Soda OS from its network ISO with graphical Anaconda and protected OEMDRV input.

Use the Soda OS network ISO to install a physical server or a locally managed
virtual machine. The installation uses Fedora's stock graphical **Anaconda**
installer and a separate protected input image labelled **OEMDRV**.

OEMDRV carries the first administrator credentials and Tailnet enrollment key.
Keeping those values separate from the reusable installer ISO lets one verified
ISO install multiple machines without embedding any team's secrets.

## Before you begin

You need:

- an x86-64 or AArch64 machine and its matching Soda OS release assets;
- working installation-time network access;
- the signed Soda provisioning command-line tool distributed with the release;
- installation media or virtual media for both the ISO and OEMDRV image;
- the first administrator's username and password;
- that administrator's SSH public key; and
- a new one-use Tailscale authentication key for the target Tailnet.

Back up every disk that may be selected in Anaconda. Installing an operating
system can permanently overwrite the selected storage.

## Download and verify the installer

Download the architecture-matched installer and verification material:

```text
SodaOS-<version>-<architecture>.iso
SodaOS-<version>-<architecture>.iso.sha256
soda-os-<version>-<architecture>.release.json
soda-os-<version>-<architecture>.release.json.sigstore.json
```

Verify the ISO checksum:

```sh
sha256sum --check SodaOS-<version>-<architecture>.iso.sha256
```

Verify the signed release record with Cosign:

```sh
cosign verify-blob \
  --bundle soda-os-<version>-<architecture>.release.json.sigstore.json \
  --certificate-identity \
  'https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  soda-os-<version>-<architecture>.release.json
```

Confirm that the record names the selected architecture, contains the same ISO
checksum, and identifies an exact Soda OS image digest.

The installer retrieves that exact image while installing, so the machine
needs working network access to the published image registry.

## Create protected OEMDRV media

Use `soda-image` from the signed provisioning-tool download for the same
release to create a new OEMDRV image bound to the exact installer ISO and
release record:

```sh
soda-image --architecture <architecture> installer-input \
  --iso SodaOS-<version>-<architecture>.iso \
  --release-record soda-os-<version>-<architecture>.release.json \
  --username <administrator> \
  --ssh-public-key-file <public-key-file> \
  --tailscale-auth-key-file <tailscale-key-file> \
  --output <new-oemdrv-image>
```

Omit `--password-file` to enter the password at the tool's protected prompt.
The tool validates the public inputs, writes the medium with restrictive file
permissions, and refuses to replace an existing output. Keep the result out of
shared folders, source control, shell arguments, logs, and unprotected virtual
media libraries.

## Start the graphical installation

1. Attach or write the verified Soda OS network ISO.
2. Attach the newly generated OEMDRV medium.
3. Boot the machine from the Soda OS ISO.
4. In Anaconda, select the installation disk, networking, locale, keyboard, and
   hostname.
5. Review the storage choice carefully before beginning installation.

The protected medium supplies the administrator account fields, so do not
create a second administrator in Anaconda. Anaconda remains responsible for
storage selection, networking, bootloader setup, and deployment of the exact
Soda OS image.

## Remove the protected medium

Soda ejects OEMDRV before installation continues. When prompted:

1. confirm that the physical or virtual tray is open;
2. detach the OEMDRV device from the machine; and
3. destroy the operator-side OEMDRV file and any copy retained by the
   virtualization platform.

Installation waits while the secret-bearing medium remains attached. Keep the
reusable Soda OS installer ISO, but never reuse OEMDRV for another machine.

## Complete first boot

The installed system contains:

- the primary Linux administrator in `wheel`;
- the supplied key in that account's standard `authorized_keys`;
- a same-named Forgejo site administrator;
- that administrator's private Tea login; and
- one attempt to enroll the machine in the selected Tailnet.

Linux and Forgejo begin with the selected username and initial password, then
manage their accounts independently.

If installation does not reach this complete result, correct the cause, create
fresh protected OEMDRV media with a new one-use Tailscale key, and perform a
fresh installation.

Continue with [Make the first connection](30-first-connection.md).
