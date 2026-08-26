# Soda installer customization

Soda OS uses the Rocky Linux 10.2 DVD only for its boot and graphical installer
runtime, including the exact
`anaconda-gui-40.22.3.46-1.el10.rocky.0.6.aarch64` package. The installed
payload comes from the current Rocky Linux 10 BaseOS and AppStream mirrorlists.
The installer is customized without an Anaconda fork or custom installer
Python.

`distro/soda.toml` is the schema-version-3 image contract. It pins the Soda
profile ID, Anaconda package, volume ID, boot timeout, Rocky mirrorlists,
package roots, weak-dependency policy, ISO size ceiling, branding manifest,
and upstream overlay manifest. `just check` validates that contract, the
committed asset dimensions and hashes, Kickstarts, and spoke lists without
contacting the network.

During `just rpm`, the Go image builder:

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

During `just iso`, the Go builder resolves and records the current network
payload, extracts the source EFI tree, installs a complete Soda GRUB
configuration, rebuilds the AArch64 EFI image with `mkefiboot`, and replays the
source boot layout with xorriso. The complete BaseOS and AppStream DVD trees
are removed before the image is written. The builder adds only the local Soda
repository, `product.img`, boot runtime, licenses, and release key, then
implants and verifies the media checksum. It rejects unexpected root files,
local Rocky package trees, extra Soda RPMs, or an image larger than 1.25 GiB.

Installation requires wired DHCP and Internet access. RPM weak dependencies
are disabled, while Rocky's minimal environment still selects its standard
general-hardware firmware packages. Exact Rocky RPM versions can advance while
Soda OS remains version 0.2.0.

Rocky Linux remains the kernel, userspace, package source, repository identity,
and RPM provenance. This base is documented in the installed
`/usr/share/doc/soda-release/BASE_SYSTEM.md`; Rocky names are not presented as
the product identity.
