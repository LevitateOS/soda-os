# Bootc runtime image, installer, and update release gate

This scenario covers Soda OS 0.3.0 on Fedora 44 bootc for AArch64. Generated
images, package inventories, RPMs, keys, credentials, logs, databases, and
ephemeral registry state stay under ignored artifact paths.

1. Run `just check`; require the exact Fedora bootc digest, `linux/arm64`, Soda
   registry name, state schema 2, source-date epoch, and package lock.
2. Run `just rpm`; require exactly the locked `soda-release`, `soda-runtime`,
   and `soda-cockpit` RPM inputs plus their recorded hashes.
3. Run `just oci REGISTRY_CA COSIGN_PUBLIC_KEY`; require an OCI archive at
   `.artifacts/images/soda-os-0.3.0-aarch64.oci.tar` and no registry push.
4. Require the build to verify all locked Fedora and Soda NEVRAs, fixed UID/GID
   976, enabled SSH/Soda/Avahi services, enabled persistent-state mounts, the
   masked `bootc-fetch-apply-updates.timer`, the embedded registry CA and Cosign
   public key, and the installed RPM inventory checksum.
5. In a disposable local HTTPS registry with an ephemeral passphrase-protected
   Cosign key, publish the OCI archive with `soda-image publish --defer-current`.
   Require a signed, verified exact `registry.soda.local/soda/os@sha256:...`
   payload and no release record or `current` tag.
6. Build `soda-image iso` from that exact signed digest. Require an AArch64
   `bootc-generic-iso`, ext4, ISO SHA-256 sidecar, payload provenance, and an
   embedded payload matching the exact digest.
7. Run the final `soda-image publish --iso ISO_PATH ...`. Require its signed
   release record to agree with the OCI labels and ISO checksum, and require
   `current` to be updated only after that record verifies.
8. On AArch64 UEFI, complete the stock interactive Anaconda fresh-install flow.
   Require `bootc status` to report the ISO's exact digest, persistent schema-2
   Soda state, PAM users, Cockpit certificates, SSH host/device keys, direct
   project SSH, repositories, worktrees, toolchains, and logs after restart and
   reboot.
9. Publish a distinct signed runtime digest. While a direct SSH workload stays
   active, require `sodactl os update check` and `stage` to leave the running
   deployment and services unchanged. Require an ordinary reboot before
   activation to retain the booted digest; require
   `sodactl os update activate --confirm-reboot` to boot the staged digest and
   preserve all host state.
10. Run `go test ./...` and `go vet ./...` with a writable Go build cache.

The gate excludes Rocky conversion, automatic updates or reboot, alternate
registries or channels, and database schema changes.
