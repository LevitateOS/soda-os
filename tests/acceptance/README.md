# Bootc runtime image, installer, and update release gate

> [!IMPORTANT]
> This document records the pre-reset acceptance workflow currently present in
> the repository. Its Soda database, project/worktree, forced-SSH, toolchain,
> release-index, and update-control-plane assertions are implementation evidence,
> not preservation requirements. See the
> [architectural reset](../../docs/architecture-reset.md) and
> [issue #25](https://github.com/LevitateOS/soda-os/issues/25) for the target
> product-level acceptance outcomes.

This scenario covers Soda OS 0.4.0 on Fedora 44 bootc for the equal AArch64 and
x86-64 sibling architectures. Run it independently for each architecture;
evidence from one does not satisfy the other's gate. Generated images, package
inventories, RPMs, credentials, logs, databases, and ephemeral registry
state stay under ignored artifact paths.

1. Run `just check`; require each selected sibling's exact Fedora bootc digest,
   OCI platform, Soda registry name, state schema 4, source-date epoch, and
   package lock.
2. Run `just rpm ARCH`; require exactly the selected platform's locked
   `soda-release`, `soda-runtime`, `soda-cockpit`, and `soda-forgejo` RPM inputs plus their
   recorded hashes.
3. Run `just oci ARCH`; require an OCI archive at
   `.artifacts/images/soda-os-0.4.0-ARCH.oci.tar` and no registry push.
4. Require the build to verify all locked Fedora and Soda NEVRAs, fixed UID/GID
   976, enabled SSH/Soda/Avahi services, enabled persistent-state mounts, the
   masked `bootc-fetch-apply-updates.timer`, the embedded GitHub release index
   location, and the installed RPM inventory checksum.
5. Build the architecture-selected `soda-image iso` directly from the local
   OCI archive. Require a platform-matched `bootc-generic-iso`, ext4, an ISO
   SHA-256 sidecar, and an embedded payload matching the archive's exact digest.
   This step must not require a registry, network access, or signing credentials.
6. Optionally run `soda-image record --archive ... --iso ...`. Require its
   architecture-named local record to agree with the OCI labels and ISO
   checksum. If distributing a paired build, require `soda-release` to publish
   both sibling artifacts and one release index in a single GitHub Release.
7. On the selected platform's UEFI, complete the stock interactive Anaconda
   fresh-install flow.
   Require `bootc status` to report the ISO's exact digest, persistent schema-4
   Soda state, PAM users, Cockpit certificates, SSH host/device and outbound Git
   keys, direct
   project SSH, repositories, worktrees, toolchains, and logs after restart and
   reboot. Create one project through “Create a new repository on this Soda
   server” and prove that its Built-in Git repository accepts the project deploy
   key and each Soda member's generated Git key. Create a second project through
   “Connect an existing Git repository”, add the displayed bootstrap person's
   public key to the external Git account, continue setup, and prove personal-UID
   SSH, direct commands, Git-over-SSH, and SFTP use the selected workspace.

For repeatable raw-QEMU qualification, run `tests/acceptance/unattended.sh
prepare`, load the generated `runner.env`, and use
`tests/acceptance/bootc.sh launch install`. Raw QEMU attaches a per-run
`OEMDRV` Kickstart ISO containing test-only identity, password, SSH-key,
storage, and reboot inputs. The qualified Soda installer ISO remains unchanged.
Credentials stay in the ignored acceptance evidence directory for automated
SSH, Cockpit, privileged bootc evidence, restart, and reboot checks. The
bootstrap key is written to Soda's authoritative per-person authorized-key path;
qualification must then register that same key through the normal account flow
before testing other people.
8. Publish a distinct runtime digest. While a direct SSH workload stays
   active, require `sodactl os update check` and `stage` to leave the running
   deployment and services unchanged. Require an ordinary reboot before
   activation to retain the booted digest; require
   `sodactl os update activate --confirm-reboot` to boot the staged digest and
   preserve all host state.
9. Run `go test ./...` and `go vet ./...` with a writable Go build cache.

The gate excludes automatic updates or reboot, alternate registries or channels,
and database schema changes.
