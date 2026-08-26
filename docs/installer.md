# Soda installer customization

Soda OS uses the Rocky Linux 10.2 DVD payload and the exact
`anaconda-gui-40.22.3.46-1.el10.rocky.0.6.aarch64` installer package. The
installer is customized without an Anaconda fork or custom installer Python.

`distro/soda.toml` is the schema-version-2 image contract. It pins the Soda
profile ID, Anaconda package, volume ID, boot timeout, branding manifest, and
upstream overlay manifest. `just check` validates that contract, the committed
asset dimensions and hashes, and the spoke lists.

During `just rpm`, the Rust image builder:

1. verifies the signed Rocky checksum file and source DVD digest;
2. extracts the pinned Anaconda GUI RPM from that DVD;
3. verifies every overlaid upstream Glade file against
   `packaging/anaconda/upstream.toml`;
4. applies only the declared property overrides and validates the resulting XML;
5. creates an xz-compressed, reproducible `newc` archive at
   `.artifacts/installer/product.img`;
6. builds the three target RPMs and the isolated build-only branding RPM.

The profile leaves language, keyboard, timezone, automatic local-disk storage,
wired DHCP, and administrator creation visible. Software source and selection,
kdump, root password, manual or specialized storage, reclaim, encryption, and
advanced network controls are deliberately unavailable. Their required values
come from the Kickstart and profile contracts.

During `just iso`, the Rust builder extracts the source EFI tree, installs a
complete Soda GRUB configuration, rebuilds the AArch64 EFI image with
`mkefiboot`, replays the source boot layout with xorriso, rewrites media
identity, adds `product.img` and the local Soda repository, and implants the
media checksum. It then reads the resulting ISO back and checks its volume,
UEFI record, menu entries, Kickstart mode, Soda metadata, and product-image
contents.

Rocky Linux remains the kernel, userspace, package source, repository identity,
and RPM provenance. This base is documented in the installed
`/usr/share/doc/soda-release/BASE_SYSTEM.md`; Rocky names are not presented as
the product identity.
