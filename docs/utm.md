# UTM development VM

Use an Apple Silicon Linux virtual machine with:

- AArch64 virtualization and UEFI
- 4 virtual CPUs
- 8 GB RAM
- 64 GB disk
- the generated `SodaOS-0.1.0-aarch64.iso` as removable installation media
- bridged networking when `soda.local` must be reachable from the LAN

Complete graphical Anaconda installation, create the first administrator, then
eject the ISO before rebooting. The installed system is headless; use SSH and
`https://soda.local:9090` afterward.
