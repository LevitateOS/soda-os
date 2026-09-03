# Installer size accounting

Soda retains one graphical Anaconda installer ISO.  Its payload is a network
installation of the exact GHCR bootc image; the runtime image is not embedded
in the installer.

## x86-64 evidence

| Artifact | ISO bytes | Installer SquashFS bytes | Evidence |
| --- | ---: | ---: | --- |
| Fedora 44 Everything netinstall 44-1.7 | 1,217,329,152 | 904,056,832 | official Fedora checksum manifest |
| Soda before direct installer composition | 2,091,892,736 | 1,727,160,320 | historical local candidate |
| Soda direct-composition candidate | 1,067,196,416 | 870,174,720 | native build on 2026-09-03 |

The 823 MB excess was an entire Fedora bootc root at `/sysroot` inside Soda's
installer SquashFS. It was inherited by using the bootc base as the installer
container base. The installer now uses the separately locked Fedora installer
base and names its actual requirements explicitly: graphical Anaconda,
bootc-install support, kernel/modules, firmware boot packages, OEMDRV input,
and the installer-only provisioning tools.

The ISO inspection rejects an installer SquashFS that contains `/sysroot`.
Package-root additions require a direct installer ownership reason and updated
matching-native size evidence. This x86-64 result is not AArch64 evidence.
