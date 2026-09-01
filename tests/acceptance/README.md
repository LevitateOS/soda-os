# Bootc installation and native-product evidence

The governing product outcomes are in
[architecture-reset.md](../../docs/architecture-reset.md). This directory
contains the current raw-QEMU harness; issue #25 still owns its final
single-workflow migration and the final post-#39 execution.

Run every artifact and installation operation independently on matching-native
x86-64 and AArch64 hardware. Evidence from one sibling does not qualify the
other.

## Artifact ladder

1. Run `just check`.
2. Run `just rpm ARCH` and require the exact locked `soda-release`,
   `soda-runtime`, `soda-projects`, and `soda-forgejo` inputs.
3. Run `just oci ARCH` and inspect the matching-native OCI archive, installed
   package inventory, image labels, stock Cockpit payload, and absence of the
   deleted identity/project/dashboard/SSH control-plane payload.
4. Build the matching ISO from that local archive and verify its checksum and
   exact embedded image digest.
5. Install through native raw QEMU and capture the booted digest, native
   services, RPM inventory, and product scenarios.

## Raw-QEMU preparation

On matching-native x86-64 or AArch64 hardware,
`tests/acceptance/unattended.sh prepare` creates a protected, test-only OEMDRV
Kickstart input for a disposable installation. It requires a protected file
containing one disposable Tailscale auth key. The runner removes the Kickstart
source after creating OEMDRV; after Anaconda confirms parsing it, QMP ejects the
medium and the host file is removed.

Load the generated `runner.env`, then use:

```sh
tests/acceptance/bootc.sh launch install
tests/acceptance/bootc.sh wait
tests/acceptance/bootc.sh capture installed
tests/acceptance/bootc.sh stop
```

The administrator key is installed through Anaconda's native `sshkey` input in
standard `~/.ssh/authorized_keys`. Post-install checks use the enrolled
MagicDNS identity over the Tailnet. QEMU host forwards are test plumbing only,
not product exposure.

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
