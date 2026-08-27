# Bootc runtime image release gate

This scenario covers the Soda OS 0.2.0 AArch64 runtime-image phase. Generated
images, package inventories, RPMs, keys, credentials, logs, and databases stay
under ignored artifact paths.

1. Run `just check`; require the exact Fedora bootc digest, `linux/arm64`, Soda
   registry name, state schema 2, source-date epoch, and package lock.
2. Run `just rpm`; require exactly the locked `soda-release`, `soda-runtime`,
   and `soda-cockpit` RPM inputs plus their recorded hashes.
3. Run `just oci`; require an OCI archive at
   `.artifacts/images/soda-os-0.2.0-aarch64.oci.tar` and no registry push.
4. Require the build to verify all locked Fedora and Soda NEVRAs, fixed UID/GID
   976, enabled SSH/Soda/Avahi services, enabled persistent-state mounts, the
   masked `bootc-fetch-apply-updates.timer`, and the installed RPM inventory
   checksum.
5. Run `go test ./...` and `go vet ./...` with a writable Go build cache.

Signing, publication, ISO generation, installation testing, and update APIs are
not release gates in this phase because those capabilities are not present.
