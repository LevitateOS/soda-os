# WSL2 research: future x86-64 Windows support

WSL2 on x86-64 Windows gaming PCs is explicitly planned for a future release,
with no Soda WSL download at launch. This research does not gate the equal
x86-64/AArch64 ISO and QCOW2 products or approve a reduced development shell.
Windows-on-Arm is outside this particular investigation.

## Evidence and decision boundary

The 2026-09-04 study was documentary only: no Windows host, downloaded/verified
image, native execution, or Soda WSL prototype. Its exact references and
unexecuted experimental detail remain in the
[pinned study](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/wsl-feasibility.md).
Do not follow its superseded Soda Setup, custom firewall, or Forgejo-deletion
instructions. The [current product contract](../product-contract.md) governs
any future proposal.

The study inspected Fedora WSL 44-1.7 x86-64 metadata, WSL 2.7.13, a separately
pinned Microsoft 6.18.40.1 kernel configuration, and Soda source `53674227`.
Metadata was not proof of downloaded bytes, and the inspected kernel source
was not bound to the installed WSL release. Re-establish exact versions and
signed image identities on the native test machine rather than treating these
old inputs as a permanent lock.

The candidate recommendation is to investigate Fedora WSL with native package
maintenance alongside native image replacement. A package-based Soda release,
changed update/fallback contract, SELinux differences, and any new packaging
boundary still require explicit owner decisions. Missing evidence is not a
proven no-go; an upstream primitive is not a complete Soda product.

## Questions that remain independent

| Boundary | Established documentary fact | Required native proof or decision |
| --- | --- | --- |
| Unattended lifetime | Systemd starts services, but Microsoft says services alone do not keep an instance alive | Cold boot before interactive Windows login, closed terminals, logout, overnight idle, reboot, sleep/resume |
| Native startup | Task Scheduler has a boot trigger; WSL registrations belong to a Windows user | Same owning user can launch the named distribution non-interactively, without periodic relaunch or a resident keeper |
| First use | WSL distribution OOBE supports a command and default UID | One native account/provisioning journey, interruption behavior, normal login, stateless Soda welcome, usable private access |
| Service composition | Fedora/Microsoft recommendations alter some native units | Actual NetworkManager, tmpfiles, PAM, SSH, Forgejo, Cockpit, Runners, and Tailscale behavior; do not disable units blindly |
| LAN access | Mirrored networking offers direct LAN facilities on supported Windows 11 | Separate-client SSH/Cockpit/Forgejo/development traffic, hot reload, Windows/Hyper-V plus Linux firewall interaction |
| Tailnet access | Tailscale documents WSL operation and host-client/MTU caveats | In-WSL identity, actual transfers, approved routing, host-client coexistence, reboot persistence |
| Security | Inspected kernel source enabled SELinux | Actual shipped kernel, loaded policy, enforcing behavior, service confinement, Windows/root-owner trust differences |
| Filesystems | Linux storage and Windows-mounted drives have different permissions | Private homes, executable/symlink/locking behavior, UID ownership, no relocation to Windows paths to bypass failure |
| Native image replacement | Import/export handles rootfs snapshots | A newer OS with all current accounts, credentials, data, and roles retained; no demonstrated selective native merge yet |
| Fedora package updates | DNF5 owns normal package transactions | Real changed packages and preserved Soda behavior/data, including interruption; explicit decision about departure from bootc |
| Fedora release upgrade | Public DNF5 path uses an offline systemd transaction | WSL actually enters/completes the target; ordinary package updates do not prove this |

The public VM timeout setting is not a documented per-distribution always-on
contract. Do not use an undocumented timeout, dummy `sleep infinity`, watchdog,
custom Windows service, or relaunch loop to manufacture successful lifetime.
`wsl --system` is the system distribution, not a Soda startup entry point.

Task Scheduler password/S4U logon types have different native credential and
network constraints. Verify them under the registration owner's identity;
LocalSystem is not a demonstrated substitute. One-time native Windows
administrator configuration is within the investigation, not a Soda credential
store. Global `.wslconfig` and Hyper-V settings affect other distributions.

## Next authorized native campaign

Prerequisites are a disposable supported x86-64 Windows host, its owning Windows
account, a separate LAN client, approved Tailnet/client resources, exact signed
Fedora inputs, and a separately reviewed Soda packaging candidate. Stock Fedora
is a platform smoke test, not proof of Soda composition. No current QCOW2 should
be flattened and called a supported WSL image.

1. Record Windows edition/build, x64 hardware, WSL/kernel versions, native
   distribution registrations, network mode, existing firewall rules, image
   signatures/hashes, package/source identities, and permission boundaries.
2. Verify a disposable Fedora base and identify its real first-run/UID behavior.
   Prepare a separate Soda candidate only within approved packaging scope;
   do not bring back the removed Soda Setup implementation.
3. Resolve lifetime first. Launch once through the public named-distribution
   interface and native startup task; probe from another machine after terminal
   close, logout, idle, cold boot, and resume. A local `wsl --exec` probe can start
   a stopped distribution and create false uptime evidence. Sleeping hardware
   cannot serve requests; test recovery after wake rather than claiming otherwise.
4. Observe native provisioning, login, PAM roles, key entry, Tailscale, and service
   ordering. Preserve one accepted user journey rather than adding WSL onboarding.
5. Test direct SSH/SCP/SFTP, real private clones for two humans, mise installs,
   separate Tea/gh authentication, actual framework hot reload, both runner
   providers, and optional rootless Podman. Use real native services, not mocks.
6. Inspect actual SELinux/PAM/permissions and Windows interoperability. Bring
   security differences back for a decision rather than silently disabling
   enforcement or claiming hostile-tenant protection.
7. Seed current accounts/groups/passwords, keys, modified/untracked file contents,
   commits, catalog metadata, Forgejo repositories/issues/keys, CLI sessions,
   runner state, and Tailnet identity. Verify those same facts across native
   stop/start, updates, release upgrades, and separately scoped recovery probes.
8. Test the contract's own-workspace, administrator project, and primary-last
   person removal with repository and Forgejo-account preservation. Exercise
   partial/unknown results and exact renewed confirmation, not old deletion APIs.
9. Retain sanitized evidence; stop tasks/writers, log out the guest's Tailnet
   identity, and remove only exact disposable registrations/rules/resources
   created for this experiment. Unregistering a distro permanently deletes it.

## Maintenance and restoration cautions

A quiesced export can restore that snapshot, not newer state written afterward.
Stop application writers and confirm the distribution is stopped before export;
abrupt termination is not application consistency. Keep private backups secure.
Test a restore offline with the original stopped; never connect two live copies
of one Tailnet identity. Compare real contents and native application behavior,
not only unchanged `git status` or byte-identical application databases.

The public DNF5 upgrade/reboot path needs its own experiment. Do not invoke its
internal `_execute` subcommand, suppress native boot boundaries, or construct a
Soda updater when WSL does not execute the offline target. DNF transaction
history or restoring an old rootfs is not an account-preserving image fallback.
Any package delivery/version-compatibility model is a separate reviewed design.

## Upstream starting points

- [Microsoft distribution guidance](https://learn.microsoft.com/en-us/windows/wsl/build-custom-distro)
- [WSL systemd and lifetime](https://learn.microsoft.com/en-us/windows/wsl/systemd)
- [WSL networking](https://learn.microsoft.com/en-us/windows/wsl/networking)
- [WSL configuration](https://learn.microsoft.com/en-us/windows/wsl/wsl-config)
- [Hyper-V firewall](https://learn.microsoft.com/en-us/windows/security/operating-system-security/network-security/windows-firewall/hyper-v-firewall)
- [WSL filesystem permissions](https://learn.microsoft.com/en-us/windows/wsl/file-permissions)
- [Tailscale in WSL](https://tailscale.com/docs/install/windows/wsl2)
- [DNF5 system upgrade](https://dnf5.readthedocs.io/en/latest/commands/system-upgrade.8.html)
- [bootc installation model](https://bootc.dev/bootc/bootc-install.html)

Verify the exact native versions when testing. Upstream documentation establishes
candidate mechanisms, not successful Soda execution or an approved product change.
