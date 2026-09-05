# Installer size accounting

> Historical record: references below to Soda Setup, selected-LAN trust, or
> default-drop host configuration are superseded by the current
> [native installation and access contract](native-onboarding.md). They are not
> operating instructions for the current image.


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
container base. The current installer now uses the separately locked Fedora
installer base and names its requirements explicitly.

The ISO inspection rejects an installer SquashFS that contains `/sysroot`.
Package-root additions require a direct installer ownership reason and updated
matching-native size evidence. This x86-64 result is not AArch64 evidence.

The direct-composition ISO passed source checks, artifact inspection, and a
native raw-QEMU graphical boot to the branded Anaconda welcome screen. A full
network installation is not claimed by this evidence: its exact runtime digest
was not anonymously available from GHCR.

This report predates the current Soda Setup workaround and the later long-term
decision for one complete installation journey without separate Soda-owned
post-install setup. Its package closure still includes OEMDRV and
installer-only provisioning inputs that are now deletion targets. The next
size comparison must begin from the one-ISO, graphical-Anaconda contract and
attribute every retained installer package to Anaconda, bootc, firmware,
branding, or network installation. Historical byte counts remain useful
evidence but do not preserve the old onboarding path or select its replacement.
