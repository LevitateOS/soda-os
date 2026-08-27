# UTM development VM

Create an Apple Silicon Linux virtual machine in UTM with:

- Virtualize (not emulate), Linux, AArch64, and UEFI boot
- 4 virtual CPUs
- 8 GB RAM
- 64 GB disk
- the generated `SodaOS-0.2.0-aarch64.iso` as removable installation media
- bridged networking with wired DHCP and working Internet access

Boot the VM and complete the slim graphical Soda installer. Choose language,
keyboard and timezone, the target disk, wired DHCP networking, and the first
Soda administrator with a PAM password. The hostname is already `soda`.
Let installation finish, shut the VM down, eject the ISO, and boot from the
virtual disk.

For visual acceptance, keep the display at 1024x768 and capture the Soda boot
menu, welcome screen, installation summary, target-disk screen, DHCP network
screen, administrator screen, progress screen, and completion screen. If the
installer fails, preserve `/tmp/anaconda.log`, `/tmp/program.log`,
`/tmp/storage.log`, and `/tmp/packaging.log` under
`.artifacts/installer-logs/` before discarding the VM.

The installed system is headless. First connect using the Anaconda account and
import it into Soda:

```sh
sudo sodactl people import \
  --username "$USER" \
  --display-name "Your Name" \
  --email you@example.test \
  --role admin
```

After import, sign in to `https://soda.local:9090` with the same Linux username
and password. The cockpit creates its self-signed certificate on first start.
Confirm the certificate exception only for the expected local `soda` host.
Open **My Account** and add the public key and private-key path hint for the
client device. Generate an Ed25519 key on that client first when needed:

```sh
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519
```

SSH instructions shown by the cockpit use a `soda-<slug>` alias and each
project's locked `soda-p-<slug>` account. The alias selects the project and the
registered device key identifies the team member, who lands directly in their
personal workspace. Team members never share a project-account password.

The unattended `.artifacts/images/SodaOS-0.2.0-aarch64-test.iso` is strictly a
disposable test artifact. It uses username and password `soda-test`, powers off
after installation, and must not be distributed as a release image.
