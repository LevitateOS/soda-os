# Bootc installation and native-product evidence

The governing product outcomes are in
[architecture-reset.md](../../docs/architecture-reset.md). This directory
contains the current raw-QEMU harness; its final single-workflow migration and
final architecture-reset execution remain pending.

Run every artifact and installation operation independently on matching-native
x86-64 and AArch64 hardware. Evidence from one sibling does not qualify the
other.

## Artifact ladder

1. Run `just check`.
2. Run `just rpm ARCH` and require the exact locked `soda-release`,
   `soda-runtime`, `soda-projects`, `soda-forgejo`, and `soda-bun` inputs.
3. Run `just oci ARCH` and inspect the matching-native OCI archive, installed
   package inventory, image labels, complete stock Cockpit host payload,
   immutable tool manifest, and absence of the deleted identity, project,
   dashboard, SSH, telemetry, and toolchain-control payload.
4. Build the matching ISO from that local archive and verify its checksum and
   exact embedded image digest.
5. Install through native raw QEMU and capture the booted digest, native
   services, RPM inventory, and product scenarios.

## Raw-QEMU preparation

On matching-native x86-64 or AArch64 hardware,
`tests/acceptance/unattended.sh prepare` creates a protected, test-only OEMDRV
answer medium for a disposable installation through
`soda-image installer-input`. It validates the matching release record and
exact ISO checksum and passes the generated administrator password, public key,
and one disposable Tailscale key only through protected files. The shell runner
never expands either credential into Kickstart, argv, or environment values.
It selects the generator's explicit `--unattended` mode, which adds only the
fixed destructive storage and completion commands required by this disposable
VM; normal operator-created media remains graphical and storage-interactive.

The product ISO already selects `/ks.cfg` from the mandatory OEMDRV label; the
harness does not inject boot keys or replace the product boot path. Stock
Anaconda owns installation. Its fixed `%pre` hook validates the inputs, emits
native `user` and `sshkey` directives, and requests ejection in the guest. The
host requires that exact QEMU device to report an open, unlocked tray, removes
the medium from the already-open device, verifies the empty drive, and deletes
the secret-bearing host file. The host never forces the tray open. QMP evidence
is retained as `installer-input-eject.jsonl`.

The native VM uses 8 GiB of memory so the installer's 4 GiB ephemeral
`/var/tmp` mount can hold the immutable payload's transient import blobs. On
x86-64, QEMU keeps the installed disk as the default and boots the installer
media only once so the completed disk owns the first reboot.

Load the generated `runner.env` in two terminals. `launch` replaces its shell
with QEMU and remains in the foreground until the VM stops.

In terminal 1:

```sh
tests/acceptance/bootc.sh launch install
```

In terminal 2, after loading the same `runner.env`:

```sh
tests/acceptance/bootc.sh wait
tests/acceptance/bootc.sh capture installed
tests/acceptance/bootc.sh stop
```

The administrator key is installed through Anaconda's native `sshkey` input in
standard `~/.ssh/authorized_keys`. Post-install checks use the enrolled
MagicDNS identity over the Tailnet. QEMU host forwards are test plumbing only,
not product exposure.

Installed capture fails if saved input or output Kickstart, transient installer
state, installer-only hooks, legacy custom installer-extension paths, or the
one-use Tailscale credential remains. It also requires the enrollment unit to
be disabled. The accepted recovery path is a new disposable disk and a newly
generated OEMDRV image; the harness does not resume provisioning or preserve
credentials for retry.

## Native workspace slice evidence

Before the supported route switched, a disposable x86-64 guest demonstrated
stock Cockpit PAM login and Projects package discovery, catalog add/list,
synchronous setup, deterministic derived-account creation, complete clone
publication, direct shell/command/SCP/SFTP as the derived UID, and password
rejection. The route proof used a temporary package overlay on the previous
installed baseline and a test-only Tailnet-identity shim; it proves the user
path, not a fresh final image or live Tailnet exposure.

Focused and race tests additionally cover catalog edit validation, missing-key
preflight, transient Git credential transport, one-time key copying,
catalog-last project removal, primary-last Soda-aware human deletion, and
absence assertions for the deleted source and package owners. A fresh-image
inspection must still confirm removal of copied identity/project/Forgejo
projection state, the shared project mount, alternate authorized keys, forced
SSH behavior, standalone web services, and Soda SQLite authority.

The current runner captures installed platform and service evidence. Final
automation of every architecture-reset scenario and the post-#39 absence
inventory remain issue #25 work.

## Native host and immutable-toolset evidence

An installed-image capture requires the Fedora-owned Cockpit system, storage,
and networking packages plus Soda's branding and Projects package to be
discoverable. Before capture, authenticate as the primary account and exercise
Overview, Metrics, Services, Logs, Accounts, Terminal, Storage, Networking, and
Projects.
Use Projects to create a derived workspace and confirm that the derived account
is rejected by Cockpit PAM. Export its direct-SSH details in terminal 2; when
the primary administrator key was copied during setup, for example:

```sh
export SODA_ACCEPTANCE_WORKSPACE_TARGET='<derived-username>'
export SODA_ACCEPTANCE_WORKSPACE_KEY="$SODA_ACCEPTANCE_ADMIN_KEY"
export SODA_ACCEPTANCE_REQUIRE_WORKSPACE_TOOLSET=1
```

The UI observations must use Linux-owned values as displayed by stock Cockpit;
there is no Soda host-status RPC or telemetry page to compare.

`capture` sends the same reusable, unprivileged smoke script to the primary
account and, for milestone evidence, requires and exercises the derived account
when `SODA_ACCEPTANCE_REQUIRE_WORKSPACE_TOOLSET=1`. It
compares `/usr/share/soda/toolset-commands.txt` with the exact approved command
contract, resolves every entry through ordinary `PATH`, and builds or runs
small Go, Python, Rust, Node.js, Bun, C, and C++ programs in a user-owned
temporary directory. It also exercises representative Git/SSH, build, archive,
editor, and data tools plus rootless `podman info` and `podman unshare`. The
Podman checks use only native per-user state and do not add a Soda container
fixture or control path.

The capture must fail if `/opt/soda/toolchains`,
`/var/lib/soda/toolchains`, `opt-soda-toolchains.mount`, or
`soda-state-directories.service` exists. It must also prove that the residual
`sodad` surface remains health-only by running and validating
`sudo sodactl health`. When `SODA_ACCEPTANCE_ADMIN_PASSWORD_FILE` is set, the
password is supplied only on standard input; otherwise capture requires
passwordless non-interactive sudo for that command.

## Native update and fallback evidence

The bounded fallback fixture requires exact digest references for image A and
image B:

```sh
export SODA_ACCEPTANCE_IMAGE_A_REFERENCE='registry.example/soda-os@sha256:<a-digest>'
export SODA_ACCEPTANCE_IMAGE_B_REFERENCE='registry.example/soda-os@sha256:<b-digest>'

tests/acceptance/bootc.sh fallback seed-a
tests/acceptance/bootc.sh fallback capture a-installed
tests/acceptance/bootc.sh fallback stage b
tests/acceptance/bootc.sh fallback unlock
tests/acceptance/bootc.sh stop
```

After launching the installed disk again, capture and compare the updated
state. Repeat the same stage/unlock/reboot sequence toward A, compare current
state again, and finally recover forward to B. `fallback mutate-b` creates and
deletes authoritative B-era state only after the pre-mutation manifests have
proved equal. `fallback capture` records normalized Linux, workspace, catalog,
Forgejo, Tailscale, SSH-host-key, and automatic-update evidence without raw
password hashes or credentials.

Run the full sequence independently on matching-native x86-64 and AArch64
hardware. The x86-64 proof does not qualify AArch64 release completion.
