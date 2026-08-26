# UTM development VM

Create an Apple Silicon Linux virtual machine in UTM with:

- Virtualize (not emulate), Linux, AArch64, and UEFI boot
- 4 virtual CPUs
- 8 GB RAM
- 64 GB disk
- the generated `SodaOS-0.1.0-aarch64.iso` as removable installation media
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

The installed system is headless. First connect using the Anaconda account,
create an Ed25519 key if that account does not have one, and import it into
Soda:

```sh
ssh-keygen -t ed25519
sudo sodactl people import \
  --username "$USER" \
  --display-name "Your Name" \
  --email you@example.test \
  --role admin \
  --ssh-key "$HOME/.ssh/id_ed25519.pub"
```

After import, sign in to `https://soda.local:9090` with the same Linux username
and password. The cockpit creates its self-signed certificate on first start.
Confirm the certificate exception only for the expected local `soda` host.

SSH instructions shown by the cockpit use each project's `soda-p-<slug>`
account. A collaborator's own key selects their worktree; collaborators do not
share the project account's password (the project account is locked).

The unattended `.artifacts/images/SodaOS-0.1.0-aarch64-test.iso` is strictly a
disposable test artifact. It uses username and password `soda-test`, powers off
after installation, and must not be distributed as a release image.
