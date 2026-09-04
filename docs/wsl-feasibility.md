# WSL2 feasibility: x86-64 Windows gaming PCs

Research date: 2026-09-04. **Documentary assessment complete; native validation
unexecuted. Full Soda server viability remains unverified.**

## Decision and authority

Investigate Fedora WSL with native package maintenance as a candidate for live
validation. Native root-filesystem replacement has no demonstrated path in this
research that installs a newer OS while preserving all current Soda state.
Unattended lifetime and common setup remain independent gates for either model.
This is a recommendation about the next experiment, not approval of a WSL
product, a package-based Soda release, or a reduced development shell.

The owner amended [issue #46](https://github.com/LevitateOS/soda-os/issues/46)
for this investigation: **x86-64 Windows gaming PCs only**. Windows-on-Arm is out
of scope, not missing evidence. The issue's existing two-architecture wording
was read but not edited. This amendment is specific to WSL; the ISO/QCOW2
architecture contract is unchanged. This investigation does not gate their
qualification or release.

Other explicit owner decisions:

- Evaluate both image replacement and Fedora-native package maintenance.
- Bring SELinux/security differences back for an owner decision.
- Allow one-time administrator configuration through native Windows facilities.
- Require Windows-owned startup before interactive login and useful server
  lifetime after terminals close.
- Retain common Soda Setup, the full server workflow, and Tailscale identity
  inside WSL. Do not add a custom updater, separate onboarding, credential store,
  resident keeper, daemon, control plane, or reconciliation system.

There is no Windows machine available for this phase. No WSL image was
downloaded, installed, inspected, built, or executed, and no prototype was
implemented. All Windows and guest procedures below are **UNEXECUTED**. The
runbook includes explicit prerequisites where an actual Soda WSL candidate is
missing; an official Fedora installation is not that candidate.

## Evidence baseline

| Input | Exact reference inspected | What this establishes |
| --- | --- | --- |
| Soda source | `53674227e10f84c32a911223dc89f08098537ca4` | Pinned source examined, not installed WSL behavior. The checkout advanced externally during research; this task performed no pull or fast-forward, and the citations retain this baseline. |
| Issue #46 | Open; `updatedAt=2026-09-04T19:24:08Z`; no comments | Research scope before the owner's x86-64 amendment above. |
| Fedora WSL | `Fedora-WSL-Base-44-1.7.x86_64.wsl` | Published x86-64 input, not Soda packaging. [Fedora downloads][fedora-downloads] |
| Fedora WSL SHA-256 | `2e5b153ba4b639952bf546be577fc19b832fe8944caa7de342b32f10da7d319a` | Metadata in Microsoft's [distribution registry][distribution-registry]; image bytes were not downloaded or verified here. |
| WSL release | `2.7.13`, published `2026-09-04T15:41:02Z` | [Release inspected][wsl-release]; no installed version is known. The modern `.wsl` guide applies from 2.4.4. |
| Microsoft kernel source | `14794180686c2fb6307fbe359c359bec765249f3`, rolling-lts WSL `6.18.40.1` | Independently pinned [x86 build configuration][kernel-config]. The inspected WSL release notes do not bind this commit to the shipped kernel. |
| Soda x86-64 baseline | Fedora bootc 44; `bootc-0:1.16.10-1.fc44.x86_64`; Cockpit 366; Forgejo 15.0.7; mise 2026.9.1; Tailscale 1.98.8 | Repository [platform input][soda-platform] and [package lock][soda-lock], not the contents of Fedora's WSL image. |
| Windows host/build | Unavailable | Record the actual edition, build, x64 CPU, WSL version, and kernel when native testing becomes possible. |

Evidence labels used below:

- **Documented/source fact:** supported by an upstream document or inspected code.
- **Integration gap:** a concrete difference between current Soda composition and
  the proposed WSL environment; not a permanent product limitation.
- **Inference:** a conclusion drawn from those sources, requiring the named test.
- **Unverified:** no native result or insufficient supporting evidence.
- **Owner decision:** accepting a product/security difference requires the owner.

## Findings

### Startup and unattended lifetime

Systemd starts Fedora services when the distribution starts, but Microsoft
explicitly says those services do not keep a WSL instance alive. The public
`vmIdleTimeout` option concerns the WSL VM, not a documented guarantee of
per-distribution lifetime. The released 2.7.13 source has a separate
`general.instanceIdleTimeout` setting, but the public configuration reference
does not document it as an always-on interface. Neither an undocumented timeout
value nor `sleep infinity` is a supported answer established by this research.
[Systemd][wsl-systemd], [public configuration][wsl-config],
[released configuration source][wsl-core-config]

Task Scheduler's startup trigger is a documented Windows-owned launch mechanism.
WSL registrations belong to a Windows user, so the task must target the same
owner and named distribution. An interactive-only principal cannot satisfy
startup before login; LocalSystem is not a demonstrated substitute for that
owner's registration. Password logon uses existing Windows credential handling;
S4U avoids storing a password but has documented network/encrypted-file limits.
Neither logon type has been proved here to start this WSL workload before login.
Native Windows credential handling must not be replaced with a Soda store.
[WSL environment][wsl-environment], [startup trigger][schtasks-create],
[task logon types][task-logon]

The first lifetime experiment is a one-shot named-distribution launch through
that native task. It intentionally exits: success of the task is not service
readiness or durable lifetime. Do not add periodic relaunch, a dummy foreground
keeper, or a supervisor to convert a failed lifetime test into a pass.
`wsl --system` targets WSL's system distribution, not the Soda distribution,
and is not an alternative Soda startup entrypoint.
[Released CLI definitions][wsl-cli-source]

**Inference:** native scheduling supplies a plausible boot-time launch attempt,
but unattended server lifetime remains unresolved. Logout, resume, and reboot
must be measured on the recorded WSL/Windows combination. Historical release
notes are not proof of present logout behavior. A sleeping PC cannot serve
requests; the relevant resume outcome is restored service availability and
preserved state without an interactive Linux session.

### Common setup and service composition

Microsoft provides a distribution OOBE command and default UID. Its first-run
exit status controls whether initial setup succeeded. It also recommends
disabling several systemd units that can interfere with WSL, including
NetworkManager and general tmpfiles setup. These are upstream integration
recommendations, not evidence that every affected Soda operation has failed.
[Microsoft distribution guidance][wsl-distro]

Soda's existing root-only entrypoint is
`/usr/libexec/soda/soda-setup console`. Password entry needs an actual terminal;
the installed systemd unit attaches the same command to `/dev/console`.
Cockpit invokes the same setup operations with privilege elevation.
[CLI][soda-setup-cli], [console][soda-console], [service][soda-setup-unit],
[Cockpit setup protocol][soda-cockpit-setup]

Two specific source gaps need experiments:

1. `q` leaves the console successfully without necessarily dismissing setup.
   Mapping that exit status directly to OOBE completion could skip the next
   automatic setup presentation. The native hook must preserve Soda's actual
   completion fact, interruption behavior, and normal post-setup default user.
   It must not implement another account-creation workflow.
2. `NativeNetwork.Status` calls `nmcli` before reporting Tailscale status, and
   `Service.CreateAdministrator` first reads that complete status. Disabling
   NetworkManager can therefore block setup itself, not merely its LAN button.
   Enabling it blindly could interfere with WSL networking. This is a source
   integration gap, not a native reproduction.
   [Network adapter][soda-network], [setup service][soda-setup-service]

Released WSL source runs OOBE as UID 0 and completes its default-user/OOBE state
after a successful exit. When `cloud-init-main.service` is not enabled, Fedora's
current `wsl-setup` source creates UID 1000 with wheel membership and passwordless
sudo; its other branch waits for cloud-init. That branch is upstream context,
not an allowed Soda onboarding route. This current package-source
revision is not independently tied to compose 44-1.7. These upstream conventions
make duplicate first-user creation and premature OOBE completion concrete things
to avoid; they do not authorize changing Soda's account semantics.
[WSL result handling][wsl-oobe-source], [Fedora package][fedora-wsl-spec],
[Fedora OOBE][fedora-oobe]

The first Soda experiment must use the existing command through a root-attached
WSL terminal and then through the native first-run hook. A terminal invocation
alone cannot prove OOBE. Stock Fedora's own user-creation flow is only a base
smoke test; it must not create Soda's first administrator before common setup.
Likewise, disabling all tmpfiles processing without examining Soda's package
rules would not preserve its service/account composition.

### Networking, firewalls, and service addresses

Mirrored networking provides a documented direct-LAN route on Windows 11 22H2
and later. Use a currently supported Windows 11 build; this feature minimum is
not a promise to support an obsolete Windows release. Windows localhost access
does not prove remote LAN ingress. The shared WSL Hyper-V creator ID and global
`.wslconfig` settings affect more than one distribution.
[Networking][wsl-networking], [configuration][wsl-config],
[Hyper-V firewall][hyperv-firewall]

The fixed Soda services use TCP 22, 9090, and 30000; project processes select
their own non-conflicting ports. Soda's current Linux firewall starts with a
drop policy and explicitly trusts a selected NetworkManager connection.
Forgejo advertises the static hostname, or its Tailnet identity when available.
Check those advertised clone/browser addresses from both client paths, not only
listener sockets. [Managed listeners](networking.md), [Forgejo initialization][soda-forgejo-init]

Microsoft documents reserved inbound ports and network settings that WSL owns
in mirrored mode. The fixed Soda ports are not in that list. Project port
conflicts with Windows listeners and the documented reservations remain native
platform constraints to expose through ordinary diagnostics; Soda must not
allocate ports or introduce a proxy/port registry.
[Mirrored-network considerations][wsl-troubleshooting]

The Windows firewall experiment uses named, scoped rules for the test LAN and
selected test ports. It does not globally allow all inbound WSL traffic. Linux
firewalld and Windows/Hyper-V filtering both need observation. Testing a trusted
LAN must not silently mark an untrusted connection trusted.
[Hyper-V rule interface][hyperv-rule]

Tailscale documents running inside WSL, but warns about simultaneous Windows
and WSL clients and MTU handling. Its page was last validated in November 2025;
those caveats require confirmation against the recorded client and networking
mode. The baseline test uses Tailscale inside the candidate with no active host
Tailscale tunnel on the disposable test machine. Existing host-client coexistence
must be recorded as an additional compatibility question, not worked around by
moving Soda's identity to Windows. Test real SSH/SFTP and larger transfers,
not only ping. [Tailscale's WSL guidance][tailscale-wsl]

### Security and filesystem ownership

The inspected Microsoft x86 kernel configuration enables SELinux support and
includes it in its LSM configuration. Therefore an assertion that current WSL
universally lacks SELinux would be unsupported. Kernel build configuration
alone proves neither the kernel shipped in WSL 2.7.13 nor Fedora's loaded policy
or enforcing behavior. Inspect all three on the native installation.
[Kernel configuration][kernel-config]

Soda also depends on normal Linux/PAM account checks, group membership, private
homes, OpenSSH, and service confinement. Forgejo's PAM configuration excludes
workspace accounts; its systemd service has specific filesystem permissions and
protection settings. Observe actual service startup and denials before proposing
any changes. Missing enforcement or unusable confinement goes in the security
tradeoff record; it is not automatically accepted or an automatic full no-go.
[Forgejo PAM][soda-forgejo-pam], [Forgejo unit][soda-forgejo-unit]

Keep homes, checkouts, dependencies, and authoritative service data on the Linux
filesystem. Windows-mounted paths have different permission semantics and are
not interchangeable with Linux homes. Windows interoperability and
`wsl --user root` also mean the Windows owner controls this Linux instance;
Linux users are not a boundary against that Windows owner. Document that trust
relationship and inspect Windows-mounted drives and process interoperability.
[WSL file permissions][wsl-permissions], [WSL commands][wsl-commands]

### State preservation and current acceptance coverage

The existing Soda acceptance implementation switches an installed bootc disk to
another image, boots it, and compares selected state before switching back.
It does not exercise WSL. Its manifest covers account/group records, selected
home/password/key fingerprints, workspace association/origin/porcelain status,
catalog data, Forgejo users, Tailscale public identity, NetworkManager zones,
and SSH host public-key fingerprints.
[Preservation manifest][soda-manifest], [fallback sequence][soda-fallback]

That manifest alone is insufficient for this investigation. In particular,
unchanged `git status` does not prove unchanged modified-file contents, and
unchanged Forgejo users do not prove preserved repositories, issues, keys, or
credentials. The runbook adds direct observations of those authoritative facts;
it does not introduce another Soda state schema or acceptance framework.

The inspected baseline includes the later setup/workspace and acceptance fixes
merged in #72. The earlier console hangup, key-copy, and remote-command findings
must not be reported as current WSL failures merely because they appeared in
older issue discussion. Source fixes still require native execution.
[Baseline changes][soda-baseline-commit]

### OS maintenance candidates

| Candidate | Established upstream boundary | Preservation/recovery consequence | Assessment |
| --- | --- | --- | --- |
| Fresh `.wsl` rootfs replacement | Installation/import registers a rootfs; export captures an existing distribution | No documented selective merge of current Linux and Soda state into a newer rootfs was found. Restoring an older whole-rootfs snapshot also restores its old state. | Unverified replacement path; do not invent a migrator. |
| Fedora package updates in the existing Linux filesystem | DNF5 updates installed packages and owns transaction state | Existing mutable files are not replaced wholesale, but package scriptlets, config handling, and application migrations still need R6 checks. Transaction history is not an account-preserving image fallback. | Candidate for native validation; owner decision about the changed update model. |
| Fedora release upgrade | DNF5 downloads a release-upgrade transaction and normally runs it in a systemd offline boot | WSL must actually enter and complete that offline target. Restarting WSL is not proven equivalent to a normal Fedora reboot into it. | Separate unresolved gate; ordinary package-update success does not cover it. |

[Import/export][wsl-import], [DNF5 upgrade][dnf-upgrade],
[DNF5 release upgrade][dnf-system-upgrade], [DNF5 offline operations][dnf-offline]

DNF5 documents its `_execute` offline subcommand as internal, not a user-facing
entrypoint. Its source and systemd unit explain the mechanism, but directly
calling that subcommand to work around WSL is not an established maintained
WSL upgrade interface. The runbook tests the normal public transaction/boot
path and reports failure; it does not invoke `_execute`, suppress reboots with
an internal switch, or add a Soda transaction runner.
[Offline executor source][dnf-offline-source], [upstream unit][dnf-offline-unit]

bootc manages bootable container deployments including the booted kernel.
WSL's native distribution format supplies a rootfs under WSL's kernel/boot
lifecycle. Reusing bootc's installed-disk acceptance as WSL evidence, or
flattening the QCOW2, would miss this boundary. This is not proof that every
bootc command fails in WSL; no upstream-supported complete bootc-on-WSL lifecycle
was established by the inspected sources. [bootc installation model][bootc-install]

For the package candidate, Windows maintains WSL/kernel components separately
from Fedora packages. Soda would need a reviewed package delivery and version
compatibility boundary for its existing components, including application data
migrations. This report provides neither that repository nor a release promise.
There is no accepted downgrade procedure that rolls back software while retaining
all newer mutable state; document that ownership/product difference explicitly.

## Requirement and decision matrix

Every native result is currently **unverified**. “Candidate” below describes
documentary feasibility, not a pass.

| Required outcome | Evidence / present result | Remaining native proof | Windows configuration | Additional Soda ownership if needed |
| --- | --- | --- | --- | --- |
| x86-64 WSL input | Official Fedora 44 image and registry checksum | Verify actual bytes, architecture, versions | WSL2 and virtualization | WSL packaging candidate; no repacked QCOW2 |
| Startup before login; terminal-independent lifetime | Native Windows scheduling and WSL facilities require the lifetime investigation below | Cold boot, logout, idle, stop/start, resume | Same Windows owner; native startup settings | No resident keeper or supervisor |
| Common setup | Existing root CLI; OOBE completion and NetworkManager gaps | TTY, interruption, default user, first admin/key, Cockpit reopen | Native first-run hook | At most a bounded hook and proven network adapter; no second onboarding |
| Primary humans and private workspaces | Linux-native source implementation | Distinct UIDs/homes, full clone, private dependencies/processes | Linux filesystem retained | Reuse current account/workspace operations |
| Cockpit, Projects, Forgejo, OpenSSH | Existing packaged services and PAM | Real login, authorization, SSH/SCP/SFTP, Forgejo URLs | Ingress rules | Packaging/service compatibility only where demonstrated |
| mise, Tea, gh | Package/source ownership exists | Repository-configured installs; separate manual authentication | Outbound networking | No Soda tool wrapper or credential copying |
| Administrator-only Cockpit Runners | Existing provider clients and [systemd listener][soda-runner-unit] | Real Forgejo/GitHub job, account isolation, start/stop/removal, reboot and upgrade | Outbound provider access; established lifetime gate | Reuse existing local composition; providers retain workflow authority |
| Optional Podman tool | Established installed-tool contract; WSL operation unverified | Native rootless process, storage, networking, and restart checks | WSL kernel/filesystem facilities | No Soda-managed container subsystem |
| LAN plus optional in-WSL Tailscale | Documented network facilities, concrete Soda adapter gap | Separate-client reachability, reload, MTU, simultaneous paths | Mirrored networking; scoped firewall rules | No port/proxy/identity control plane |
| No unintended public exposure | Current drop/trust model | IPv4/IPv6 applicable external rejection; report untested topology | Native Windows and Linux policies | No new public bootstrap |
| Trusted-team removal semantics | Ordered existing commands | Own workspace, admin project/person removal; partial failure | None specific | Reuse existing deletion operations |
| Durable image replacement | Snapshot commands exist; selective current-state preservation not established | A newer rootfs with all current facts retained | Native WSL import/export facilities | No custom migrator/updater; unsupported gap must remain visible |
| Fedora package lifecycle | Candidate; owner decision about bootc differences | Updates, release upgrade, interruption and state checks | Windows owns kernel/VM lifecycle | Package delivery/compatibility, not a package manager |
| Security and filesystem behavior | SELinux-capable source; installed policy unknown | Enforcement, PAM, permissions, symlinks, locking, Windows interoperability | Owner controls host and WSL | Security differences returned to owner |

## Native x86-64 runbook — UNEXECUTED

### R0. Execution boundary and evidence

These are instructions for a later authorized disposable experiment, not commands
run for this report. Use native x86-64 Windows and a separate LAN client plus an
authorized Tailnet client. Do not use CPU emulation. Record exact inputs instead
of claiming that a later `latest` package is the version researched here.

Use collision-checked names: `Soda46-FedoraBase`, `Soda46-Candidate`,
`Soda46-RestoreProbe`, and `Soda46-Fedora43Upgrade`. The first is a stock-Fedora
smoke test. The second needs
separately prepared and reviewed Soda WSL packaging. The third is an offline
restore probe, never a concurrent live clone of the same Tailscale identity.
The fourth tests Fedora's release-upgrade boot path without claiming Soda parity.

Keep a human-readable results file with test ID, timestamp, exact versions,
command/context, outcome, relevant sanitized output, and remaining work. Store
backups and private fingerprints separately under operator-controlled access.
Do not record interactive setup, passwords, auth keys, private keys, CLI tokens,
Forgejo secrets, or raw Tailscale state in shareable logs.

PowerShell, as the Windows account that will own the distribution:

```powershell
$ErrorActionPreference = 'Stop'
$BaseDistro = 'Soda46-FedoraBase'
$SodaDistro = 'Soda46-Candidate'
$RestoreDistro = 'Soda46-RestoreProbe'
$UpgradeDistro = 'Soda46-Fedora43Upgrade'
$LabRoot = Join-Path $env:USERPROFILE 'Soda46Research'
if (Test-Path $LabRoot) { throw 'Choose a fresh research directory; preserve existing work.' }
New-Item -ItemType Directory -Path $LabRoot
Get-CimInstance Win32_Processor | Select-Object Name, Architecture
Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, BuildNumber, OSArchitecture
wsl.exe --version
wsl.exe --status
wsl.exe --help
wsl.exe --list --verbose
Get-NetConnectionProfile
Get-Content (Join-Path $env:USERPROFILE '.wslconfig') -ErrorAction SilentlyContinue
```

Expected: x64 Windows and WSL2; all four names unused. Record WSL's actual
kernel version. Stop before any mutation if a name collides. Check native
command exit codes (`$LASTEXITCODE`) after each `wsl.exe` invocation; PowerShell's
error preference alone does not enforce them. WSL installation, if needed, is a
separate visible administrator step using `wsl.exe --install --no-distribution`;
record any requested reboot. Do not install an unrelated default distribution.

### R1. Verify and boot the official Fedora base

PowerShell, same Windows owner, native x86-64:

```powershell
$FedoraFile = Join-Path $LabRoot 'Fedora-WSL-Base-44-1.7.x86_64.wsl'
$FedoraUrl = 'https://download.fedoraproject.org/pub/fedora/linux/releases/44/Container/x86_64/images/Fedora-WSL-Base-44-1.7.x86_64.wsl'
Invoke-WebRequest -Uri $FedoraUrl -OutFile $FedoraFile
$ExpectedSha256 = '2e5b153ba4b639952bf546be577fc19b832fe8944caa7de342b32f10da7d319a'
if ((Get-FileHash -Algorithm SHA256 $FedoraFile).Hash.ToLowerInvariant() -ne $ExpectedSha256) {
    throw 'Fedora image checksum mismatch'
}
```

Also verify Fedora's signed `Fedora-Container-44-1.7-x86_64-CHECKSUM` using its
published key, following [Fedora's verification procedure][fedora-downloads].
With native GnuPG available on the x86-64 test host, use an isolated keyring:

```powershell
$FedoraKeys = Join-Path $LabRoot 'fedora.gpg'
$FedoraChecksum = Join-Path $LabRoot 'Fedora-Container-44-1.7-x86_64-CHECKSUM'
$VerifiedChecksum = Join-Path $LabRoot 'fedora44-verified-checksums.txt'
Invoke-WebRequest -Uri 'https://fedoraproject.org/fedora.gpg' -OutFile $FedoraKeys
Invoke-WebRequest -Uri 'https://download.fedoraproject.org/pub/fedora/linux/releases/44/Container/x86_64/images/Fedora-Container-44-1.7-x86_64-CHECKSUM' -OutFile $FedoraChecksum
gpgv.exe --version
gpgv.exe --keyring $FedoraKeys --output $VerifiedChecksum $FedoraChecksum
if ($LASTEXITCODE -ne 0) { throw 'Fedora checksum signature verification failed' }
Get-Content $VerifiedChecksum
```

Confirm the verified text names the exact `.wsl` file and the same SHA-256
computed above, and verify the signer against Fedora's published key details.
Stop on mismatch. Record whether the signature as well as the hash was verified;
without GnuPG or another documented verifier, signature verification remains
unexecuted and installation is gated. Then:

```powershell
wsl.exe --install --from-file $FedoraFile --name $BaseDistro
wsl.exe --list --verbose
wsl.exe --distribution $BaseDistro --exec uname -m
wsl.exe --distribution $BaseDistro --exec cat /etc/os-release
wsl.exe --distribution $BaseDistro --user root --exec cat /etc/wsl.conf
wsl.exe --distribution $BaseDistro --user root --exec cat /etc/wsl-distribution.conf
wsl.exe --distribution $BaseDistro --user root --exec rpm -q wsl-setup systemd dnf5 selinux-policy-targeted
```

Expected: `x86_64`, Fedora 44, WSL version 2 in the distribution listing. Record
actual installed versions, OOBE command, default-user settings, PID 1, and unit
defaults. Completing Fedora's own first-user prompt is **only base evidence**.
Do not claim Soda setup, service reachability, or durability from this result.

### R2. Candidate prerequisite and common setup

**Gate:** no Soda `.wsl` candidate is supplied by this report. The later
experiment needs an explicitly prepared x86-64 candidate using maintained Fedora
WSL construction and the selected Soda source/package inputs. Record its source,
package list, rootfs checksum, first-run configuration, and every difference
from stock Fedora. Do not invent a Soda package repository, import the QCOW2,
or manually pre-create its primary administrator as a shortcut.

For that candidate, review package-specific tmpfiles, PAM, systemd, and firewall
needs before proposing changes. If the NetworkManager dependency prevents setup,
retain the exact error as an integration gap. Further candidate preparation is
separate work; this document adds no adapter or package changes.

After the candidate has been installed as `$SodaDistro`, run from an interactive
Windows terminal owned by its registering account:

```powershell
wsl.exe --distribution $SodaDistro --user root --exec /usr/libexec/soda/soda-setup status
wsl.exe --distribution $SodaDistro --user root --exec /usr/libexec/soda/soda-setup console
```

Use disposable Linux users and keys. Exercise first-administrator creation and
Forgejo key registration, LAN trust, optional Tailscale, and dismissal. A
protected operator-owned reusable ephemeral Tailscale key is permitted; enter it
through the existing hidden prompt, never the command line or transcript.
Record its type truthfully without its value.

Repeat on a fresh candidate through the actual native first-run hook. Interrupt
before account creation and again after an incomplete setup; relaunch and verify
the same authoritative state resumes without duplicate identities. Test `q`
separately from dismissal. After completion, an ordinary WSL launch must select
the intended primary user; confirm with `id`. Reopen Setup through Cockpit and
verify it observes the same machine-wide state. A `status` call needs root;
identity checks and Projects actions must retain their ordinary-user context.

### R3. Configure and inspect native networking

On a disposable Windows host, preserve its existing `.wslconfig` contents and
merge these settings through the owner's normal editor; do not overwrite other
settings or create duplicate sections:

```ini
[wsl2]
networkingMode=mirrored
firewall=true
dnsTunneling=true
```

Configuration applies after the WSL VM stops. `wsl.exe --shutdown` affects **all**
distributions, so use it only on the dedicated test host with all work quiesced.
Record existing Windows listener conflicts and the intended trusted LAN.
Elevated PowerShell, same Windows owner:

```powershell
$WslCreator = '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}'
Get-NetFirewallHyperVVMSetting -PolicyStore ActiveStore -Name $WslCreator
Get-NetFirewallHyperVProfile -PolicyStore ActiveStore
Get-NetFirewallHyperVRule -VMCreatorId $WslCreator
Get-NetTCPConnection -State Listen | Where-Object LocalPort -in 22,9090,30000,18080,18081
$TrustedLanCidr = Read-Host 'Actual trusted IPv4 LAN CIDR for this test'
New-NetFirewallHyperVRule -Name 'Soda46-LAN' -DisplayName 'Soda46 LAN test' `
    -Direction Inbound -VMCreatorId $WslCreator -Protocol TCP `
    -LocalPorts 22,9090,30000,18080,18081 -RemoteAddresses $TrustedLanCidr `
    -Profiles Private -Action Allow
```

Expected: the rule exists only for the recorded test scope and the actual LAN
profile is appropriate. Inventory other effective allow rules; this one rule
alone cannot prove public rejection. Do not change global inbound defaults or
disable either firewall to manufacture a pass. Add an explicit IPv6 test/rule
only for the actual trusted IPv6 prefix; record IPv6 absence as a test limitation.

Candidate Linux, interactive administrator terminal:

```bash
ip -brief address
ip route
sudo ss -lntup
sudo systemctl status NetworkManager firewalld sshd cockpit.socket forgejo tailscaled --no-pager
nmcli --terse --fields NAME,TYPE,ZONE connection show --active
sudo firewall-cmd --get-active-zones
sudo /usr/libexec/soda/soda-setup status
sudo tailscale status
sudo tailscale ip -4
```

Record failures directly. Complete trust and Tailscale through common Setup;
do not synthesize NetworkManager state to satisfy its status check. Verify
Forgejo's advertised URLs and SSH guidance from both browser hostnames.

### R3a. Lifetime gate before full product testing

After interactive setup is complete, test the public named-distro launch from
PowerShell, then close every Linux terminal and SSH session:

```powershell
wsl.exe --distribution $SodaDistro --exec /usr/bin/true
Start-Sleep -Seconds 90
wsl.exe --list --verbose
```

Use the separate client to probe actual services after the quiet interval.
Do **not** run `wsl --exec systemctl ...` as an availability probe: that command
can start a stopped distro and manufacture apparent uptime. Repeat with an
overnight idle interval before final lifetime acceptance; 90 seconds is an early
idle-timeout discriminator, not an availability guarantee.

Create a native startup task only for the later authorized experiment. In
Task Scheduler (`taskschd.msc`), use these explicit fields:

| Field | Test value |
| --- | --- |
| Task name | `Soda46-Start`; abort on collision |
| Windows principal | The same account that registered `$SodaDistro`, not SYSTEM |
| Trigger | At system startup; no repeating trigger |
| Logon | Run whether the user is logged on or not; enter credentials only in Windows' native dialog |
| Action program | Actual full path to `wsl.exe`, normally `C:\Windows\System32\wsl.exe` |
| Arguments | `--distribution Soda46-Candidate --exec /usr/bin/true` |
| Conditions | Record them; do not require interactive login or idle status |
| Recovery | No automatic relaunch loop or retry policy added for this probe |

This tests Password logon under native Windows ownership. Record whether the
owner accepts that native credential use. If testing S4U instead, select the
native no-password/local-resources option, record it as a separate run, and
test its documented access limitations rather than assuming equivalence.
If the owning account cannot be used non-interactively, record that result;
do not copy its registration to SYSTEM or add a custom Windows service.

Elevated PowerShell can inspect and manually invoke the created task:

```powershell
Get-ScheduledTask -TaskName 'Soda46-Start' | Select-Object TaskName, State, Principal, Actions, Triggers
Get-ScheduledTaskInfo -TaskName 'Soda46-Start'
Start-ScheduledTask -TaskName 'Soda46-Start'
```

Exercise each case separately and retain external-client timestamps, Windows
task outcome/history, and subsequent guest journal evidence:

| Case | Procedure | Required observation |
| --- | --- | --- |
| Cold boot | Fully shut down the disposable Windows host, start it, leave it at the login screen | Services become reachable without Windows login or a WSL terminal; task launch success alone is insufficient. |
| Terminal close | Close all terminals and SSH sessions; wait as above | Subsequent LAN/Tailnet clients can connect to already-running services. |
| Logout | Sign out of the owning Windows desktop session; observe from another machine | Record whether availability persists or stops and whether native recovery exists. Do not assume historical behavior. |
| Reboot | Restart Windows after quiescing test writes | Startup task and services return without interactive login; R6 state remains current. |
| Sleep/resume | Use Windows' normal Sleep action, then wake it | No service expectation while asleep; availability returns after wake without opening WSL. |
| Explicit WSL stop | On the dedicated host, `wsl.exe --shutdown`; then invoke the startup task explicitly | Stop actually stops; a deliberate native start restores services/state. An ONSTART task is not automatically triggered by this command. |

If availability depends on a dummy process, undocumented timeout, or periodic
restart, the permitted lifetime route has not passed. Preserve the failure and
stop before treating downstream tests as full-server acceptance.

### R4. Product and reachability probes

Use common Setup for the first administrator. Create later `alice` and `bob`
through stock Cockpit/Linux, publish their inbound public keys normally, and
observe each first normal Forgejo PAM login. Create a disposable repository
through Forgejo and another through an authorized external Git host. Record
their SSH URLs. In Cockpit Projects add `wsl-kept` and `wsl-remove`, including
an arbitrary metadata field; test display/metadata editing and immutable URL
behavior. Workspace accounts must remain Linux-only.

The equivalent verified Projects interfaces, run in each primary user's
interactive Linux session, are:

```bash
printf '{}\n' | /usr/libexec/soda/soda-projects list
printf '%s\n' '{"id":"wsl-kept"}' | /usr/libexec/soda/soda-projects setup
```

When cloning lacks authorization, retain the public key diagnostic, register
that key natively at the authoritative Git host, and retry the same action.
The list must report account existence honestly while the clone is incomplete.
Record each returned workspace username; do not invent the derivation algorithm.
Repeat for both primary users and both Git hosts. Inspect full clones, separate
UIDs/homes/process ownership, and one-time inbound-key copying. Confirm no
primary Tea/gh credentials or private keys were copied.
[Projects protocol][soda-projects-protocol], [setup flow][soda-workspace-setup]

Separate LAN client, POSIX shell; supply the observed hostname and derived user:

```bash
read -r -p 'Observed LAN host: ' LAN_HOST
read -r -p 'Derived workspace username: ' WORKSPACE
ssh "$WORKSPACE@$LAN_HOST" 'id; printf "%s\n" "$HOME"'
printf 'soda46 transport fixture\n' > soda46-transport.txt
scp soda46-transport.txt "$WORKSPACE@$LAN_HOST:soda46-transport.txt"
sftp "$WORKSPACE@$LAN_HOST"
```

Inside SFTP, use `ls -l soda46-transport.txt`, `get soda46-transport.txt`, then
`quit`. Verify contents. Confirm the SSH host fingerprint through the console
before accepting it. Use ordinary OpenSSH; do not enable Tailscale SSH or bypass
host-key checking. Repeat from the Tailnet client using the observed Tailnet
hostname. Test a larger disposable transfer and compare hashes at both ends to
exercise the documented MTU concern.

In a workspace, use an owner-selected disposable repository with a versioned
`mise.toml`, a real dependency, and a development server with hot reload. Record
its exact commit/tool version and commands. Run `mise trust`, `mise install`,
then `mise exec -- <repository command>` directly after inspecting its config.
Repeat in another workspace; compare installed dependency paths and process
UIDs. Authenticate Tea and gh manually and independently using their native
prompts (`tea login add`, `gh auth login`); verify actual host operations without
printing tokens. These actions need authorized disposable Git-host resources.
[mise trust][mise-trust], [mise install][mise-install], [mise exec][mise-exec]

For a simple connectivity probe only, the same workspace may run:

```bash
mkdir -p "$HOME/soda46-http"
printf 'first\n' > "$HOME/soda46-http/probe.txt"
python3 -m http.server 18080 --bind 0.0.0.0 --directory "$HOME/soda46-http"
```

This command stays in the foreground; terminate with Ctrl-C. Python must already
be supplied by that workspace's tool configuration. From separate clients,
fetch `/probe.txt` over LAN and Tailnet, change its contents, and fetch again.
Use port 18081 for the second workspace. This is **not** evidence of framework
hot reload; record the real repository/browser reload test separately.

From clients, open Cockpit at `https://HOST:9090` and Forgejo at
`http://HOST:30000`, and exercise actual authentication and Projects. Verify
the Cockpit certificate through the console/operator trust path. Test all fixed
services and the selected development ports from an authorized external client
against applicable public IPv4/IPv6 endpoints: they must be unreachable. Do not
create router forwarding or public exposure for the test. Without an applicable
external topology, report that part unverified, not passed by a localhost probe.

### R4a. Existing local runners and optional containers

In Cockpit Runners, create disposable `wsl46-forgejo` and `wsl46-github` listeners
against authorized test repositories, using provider-issued registration through
the existing UI. Do not record tokens. Verify a non-administrator is refused.
Retain provider-owned job results from a small repository workflow that reports
its Linux UID and writes a disposable fixture. Exercise Start, Stop, and Restart
through the existing page; retain the listeners for R6 and test removal in R8.
Do not add a scheduler or custom listener.
Candidate administrator diagnostics:

```bash
sudo systemctl status soda-runner@wsl46-forgejo.service soda-runner@wsl46-github.service --no-pager
getent passwd soda-runner-wsl46-forgejo soda-runner-wsl46-github
```

Record actual client versions, service confinement, and provider registration.
Repeat the job after R3a lifetime transitions and R6 maintenance; provider
registration and current local state must remain usable. Source unit tests
with fake clients do not prove this result.
[Runner composition][soda-runner-native], [listener unit][soda-runner-unit]

For the optional installed Podman tool, run `podman version` and
`podman info --format json` in a normal workspace and retain sanitized storage,
rootless, cgroup, networking, and security observations. Missing packages or
unsupported kernel facilities are findings. Use the same reviewed repository's
native container commands with a recorded x86-64 image digest to test a real
rootless process, private persistent data, and selected development port before
and after restart. Those repository-specific commands are a prerequisite, not
supplied or executed here. Podman remains user-operated and is not Soda's
workspace-isolation mechanism. [Podman diagnostics][podman-info]

### R5. Filesystems, identities, and security

Candidate Linux, administrator, then the two normal workspace sessions:

```bash
uname -r
findmnt -T /home
findmnt -T /var/lib/forgejo
findmnt -t drvfs,9p
sudo getenforce
sudo sestatus
sudo cat /sys/kernel/security/lsm
sudo systemctl show forgejo -p User -p Group -p SupplementaryGroups -p ProtectHome -p ProtectSystem
sudo journalctl -b -u forgejo -u sshd -u cockpit.socket --no-pager
```

Record missing commands/files and denials as findings; do not suppress them or
disable enforcement. Compare the installed kernel's config with the pinned
source only when that kernel can actually be identified.

Each workspace, with disposable files only:

```bash
umask 077
mkdir -p "$HOME/soda46-fs"
printf 'private fixture\n' > "$HOME/soda46-fs/private"
ln -s private "$HOME/soda46-fs/link"
printf '#!/bin/sh\nprintf "executable fixture\\n"\n' > "$HOME/soda46-fs/run"
chmod 700 "$HOME/soda46-fs/run"
"$HOME/soda46-fs/run"
stat -c '%u:%g %a %F %n' "$HOME/soda46-fs/"*
flock "$HOME/soda46-fs/lock" sleep 30
```

While that lock is held, a second session as the same workspace runs
`flock -n "$HOME/soda46-fs/lock" true`; expect failure, then success after release.
As the other unprivileged workspace, attempt to read the first workspace's
recorded private-file path; expect denial. Verify ordinary executable/symlink
semantics persist after restart and lifecycle tests. Keep these files on Linux
storage; do not move workspace homes onto `/mnt/c` to bypass a failure.

From Windows, inspect access to a disposable file through
`\\wsl.localhost\Soda46-Candidate\home\...`, record the selected default Linux
user, then compare with an explicit root invocation. From a workspace, inspect
`/mnt/c` availability and whether a harmless `cmd.exe /c whoami` works. These are
observations of the trust boundary, not instructions to weaken or broaden it.

### R6. Preserve authoritative state across lifecycle experiments

Seed state **before** updating: two primary users, multiple derived workspaces,
different password/group/admin state, changed inbound and outbound keys, one
unpublished commit, modified tracked contents, untracked files, catalog metadata,
Forgejo repositories/issues/keys, separately authenticated Tea/gh workspaces,
and a live in-WSL Tailscale identity. Use only disposable credentials and data.

Record the same facts before and after each transition:

| State | Native observations and pass condition |
| --- | --- |
| Linux identities | `getent passwd`, `getent group`, `id`; private comparison of password-record fingerprints. UIDs, GIDs, home/shell paths, passwords, and administrator memberships remain current. |
| Homes and SSH | `stat`, private file hashes, SSH public fingerprints; actual login/SCP/SFTP after the transition. Inbound, outbound, and host identities remain usable. |
| Workspace data | `git rev-parse HEAD`, `git show-ref`, `git remote -v`, `git status --porcelain=v1`, hashes of changed/untracked fixtures. Repeat local builds and tool commands. |
| Catalog | Compare canonicalized `/var/lib/soda/catalog/projects.json`; preserve arbitrary metadata and immutable identities. |
| Forgejo | Observe users/roles, public keys, issue contents, repository refs and clone/push behavior through native UI/API/Git. Preserve `/etc/forgejo` and `/var/lib/forgejo` data, including secrets, without exposing them. SQLite byte equality across an application migration is not a substitute for semantic checks. |
| CLI authentication | Existing Tea/gh sessions still operate under their own workspace identities; no credentials were shared or copied by Soda. |
| Local runners and optional containers | Preserve runner accounts, client/provider state beneath `/var/lib/soda/runners`, and listener enablement; run another real job. Preserve any seeded user-owned container data and verify it through the same native repository commands. Keep all provider credentials private. |
| Tailscale | Compare public node ID/name/addresses and prove reconnect with the existing identity. Preserve private `/var/lib/tailscale` state without logging it or booting duplicate live identities. |
| Setup/network/services | Same dismissal/account facts, native firewall configuration, service enablement, advertised addresses, and successful remote access. |

Use the existing [Soda preservation manifest][soda-manifest] as a coverage
reference, not as a standalone WSL script: it assumes NetworkManager and contains
only the selected fields described above. The current QEMU/bootc runner is not
a WSL acceptance command. No result here may be attributed to that runner unless
it actually exercised the reported boundary.

Before the snapshot, stop project writers and container writers through their
native commands, and quiesce runner jobs/listeners through their provider/UI.
Record enablement so the restore can distinguish deliberate stops from damage.
In the
candidate's administrator terminal:

```bash
sudo systemctl stop forgejo
sync
sudo systemctl poweroff
```

PowerShell: inspect `wsl.exe --list --verbose` without launching another guest
command. The candidate must show Stopped. If clean poweroff did not stop it,
record that lifecycle failure and stop this backup procedure; do not silently
substitute abrupt termination. With the candidate confirmed stopped:

```powershell
$BackupVhd = Join-Path $LabRoot 'candidate-before.vhdx'
if (Test-Path $BackupVhd) { throw 'Preserve the existing backup' }
wsl.exe --export $SodaDistro $BackupVhd --vhd
Get-FileHash -Algorithm SHA256 $BackupVhd
wsl.exe --import $RestoreDistro (Join-Path $LabRoot 'restore') $BackupVhd --vhd
```

Abrupt `--terminate` alone is not an application-consistency guarantee.
Keep the original stopped while the restore probe runs, and never connect two
copies of its Tailscale identity. For a restore probe, use an isolated network
environment and inspect offline state before allowing services to connect.
Record all R6 facts. This proves only restoration of that snapshot, not upgrade
or preservation of changes made after it. Never restore an old whole-system
snapshot and call its old account/password state the current state.

For a newer-image replacement experiment, first identify a supported native
procedure that preserves every R6 fact while actually changing OS contents.
No such complete procedure was established here. Do not fill this gap with an
invented copy/migration script, bootc emulation, or hidden restoration of stale
state. The package-lifecycle candidate is described separately below.

### R6a. Fedora package maintenance and release upgrade

Candidate Linux, interactive administrator, with R6 baseline and a consistent
private backup already retained:

```bash
rpm -q dnf5 fedora-release systemd
sudo dnf5 --refresh upgrade
sudo dnf5 history list
```

Review the proposed transaction interactively. Record changed versions and
recheck all R6 facts plus real services/tools. If no packages change, this is
not update evidence. Restart through the tested native lifetime path and repeat
the checks. Do not use DNF history undo as a claimed substitute for image
fallback or for restoration of application data.

For an immediately specifiable **platform-only** release-upgrade probe, the
pinned distribution registry also lists
`Fedora-WSL-Base-43-1.6.x86_64.wsl`, SHA-256
`220780af9cf225e9645313b4c7b0457a26a38a53285eb203b2ab6188d54d5b82`.
Verify its signature/hash using the same native process as R1, install it under
`$UpgradeDistro`, and record its installed DNF5/systemd versions:

```powershell
$Fedora43File = Join-Path $LabRoot 'Fedora-WSL-Base-43-1.6.x86_64.wsl'
Invoke-WebRequest -Uri 'https://download.fedoraproject.org/pub/fedora/linux/releases/43/Container/x86_64/images/Fedora-WSL-Base-43-1.6.x86_64.wsl' -OutFile $Fedora43File
if ((Get-FileHash -Algorithm SHA256 $Fedora43File).Hash.ToLowerInvariant() -ne '220780af9cf225e9645313b4c7b0457a26a38a53285eb203b2ab6188d54d5b82') {
    throw 'Fedora 43 image checksum mismatch'
}
# Complete signed-checksum verification before installation.
wsl.exe --install --from-file $Fedora43File --name $UpgradeDistro
```

Inside that Fedora 43 distribution, seed disposable account/home/config fixtures,
record them, and run the public DNF5 path:

```bash
sudo dnf5 --refresh upgrade
sudo dnf5 system-upgrade download --releasever=44
sudo dnf5 offline status
sudo dnf5 offline reboot
```

After its attempted reboot, observe externally whether the WSL instance restarts.
If manual native launch is needed, record that intervention and use
`wsl.exe --distribution $UpgradeDistro` once. Inspect:

```bash
cat /etc/os-release
sudo dnf5 offline status
sudo dnf5 offline log
sudo journalctl -b -u dnf5-offline-transaction.service --no-pager
```

Expected: a real completed Fedora 43-to-44 transaction, preserved fixtures,
and a usable system. If the normal systemd offline target is not entered or the
transaction fails, stop and retain native status/journal evidence. Do not invoke
the internal `_execute` command. A successful stock-Fedora probe establishes
the platform mechanism only: it does not prove that current Fedora-44 Soda
packages survive a future Fedora release upgrade. That full Soda test needs
reviewed package inputs for both release endpoints and all R6 checks; these
inputs are not provided here.

Reuse `$RestoreDistro` as the disposable recovery copy, with the original stopped
and its duplicate network identity isolated. For each interruption case, start
from a fresh import of the protected pre-transaction snapshot. While the operator
observes the transaction downloading, and in a separate fresh run while it is
applying, deliberately force-stop **only** that recovery copy:

```powershell
wsl.exe --terminate $RestoreDistro
wsl.exe --distribution $RestoreDistro
```

Record the exact interrupted stage and native error. This is intentionally an
abrupt-failure experiment, unlike the clean snapshot procedure. Inspect DNF's
status, filesystem, application data, and supported recovery instructions.
Do not interrupt a user's retained candidate or repeatedly retry a partially
successful operation. Report whether recovery preserves current state or instead
requires restoring an older backup; the latter is a data-recovery limitation,
not a successful state-preserving fallback.

### R7. Removal and explicit partial failure

Run these only against the named disposable fixtures, after preservation tests.
Keep non-administrator and administrator observations separate. From an ordinary
primary user's interactive session:

```bash
printf '%s\n' '{"id":"wsl-remove"}' | /usr/libexec/soda/soda-projects remove-workspace
printf '%s\n' '{"id":"wsl-remove"}' | /usr/libexec/soda/soda-projects remove
```

Expected: own workspace removal succeeds; whole-project removal is refused for
a non-administrator. Recreate the disposable workspaces through the normal setup
flow, then run project removal as a primary administrator. All local workspaces
and the catalog entry disappear; the canonical Forgejo repository remains.

Create disposable primary `obsolete` through Linux/Cockpit, let it sign into
Forgejo, give it a workspace and a repository it owns, then as the administrator:

```bash
printf '%s\n' '{"username":"obsolete"}' | /usr/libexec/soda/soda-projects delete-human
```

Use the existing native-repository-owner refusal scenario: expect workspace
removal, Forgejo refusal, the primary Linux account retained, and a precise
partial-result diagnostic. Resolve that disposable repository through Forgejo's
normal owner action, explicitly retry, and verify the ordered completion. Do
not substitute a mocked failure. Separately verify that generic account deletion
remains non-cascading using a different disposable fixture.
[Deletion order][soda-human-delete], [existing partial-failure scenario][soda-removal-test]

### R8. Cleanup

Inventory exact resources first. Remove only the two test runners through the
existing Runners page and remove their provider registrations through the
provider's native interface as needed. Remove only the recorded test containers
and volumes through their repository/native commands; never prune shared storage.
Stop native startup tasks before stopping the
candidate, then log the test guest out with `sudo tailscale logout` while its
identity is still available. Preserve sanitized evidence and any operator-kept
private backup. Do not remove user Git-host resources, credentials, or directories
outside the disposable names recorded in R0.

Elevated PowerShell, only if the named task was created by this experiment:

```powershell
Disable-ScheduledTask -TaskName 'Soda46-Start'
Stop-ScheduledTask -TaskName 'Soda46-Start'
```

On Windows, remove only the named test firewall rule and any exact test startup
task created by the lifetime experiment. Restore only configuration changes made
for this experiment, preserving later unrelated edits. Unregistering a distro
permanently deletes its local contents; after the operator confirms the exact
disposable names and retained evidence, use the corresponding commands:

```powershell
Unregister-ScheduledTask -TaskName 'Soda46-Start'
Remove-NetFirewallHyperVRule -Name 'Soda46-LAN'
wsl.exe --unregister $UpgradeDistro
wsl.exe --unregister $RestoreDistro
wsl.exe --unregister $SodaDistro
wsl.exe --unregister $BaseDistro
```

Run each only if that resource was actually created. Do not use wildcard task,
firewall, distribution, or directory cleanup. Backups include identities and
credentials; keep or delete them explicitly under operator control.

## Completion and follow-up boundary

The first live campaign should resolve unattended lifetime and shared setup
before spending effort on full product testing. A failure can justify no-go only
when a required outcome cannot be achieved through the permitted boundaries.
Missing hardware, a missing candidate, or unclear documentation means unverified.
Package-lifecycle and security differences remain owner decisions.

If the x86-64 campaign succeeds, the smallest implementation proposal is bounded
WSL packaging, a native first-run hook to common Setup, only demonstrated
service/network integration changes, and native Windows configuration documented
for the owner. Any native package-maintenance choice must explicitly replace the
WSL update contract; it must not grow a Soda updater. No implementation is
authorized by this report. Issue #46 remains unchanged and open.

## Verification of this report

The research inspected source and upstream documentation only. All 33 upstream
reference targets resolved during the link check; Soda file/line anchors were
checked against the pinned Git objects. Reference definitions and the relative
documentation link were checked locally. Bash code blocks passed `bash -n`;
PowerShell and guest command interfaces were compared with source/documentation,
but no PowerShell parser or Windows runtime was available. `git diff --check`
is the documentation whitespace check. No image verification, package operation,
unit/acceptance suite, Windows configuration, or native product test was run.

## Source references

[fedora-downloads]: https://fedoraproject.org/misc/
[distribution-registry]: https://github.com/microsoft/WSL/blob/556440f392aef150dc2e8d39152b20a4d7d5b83c/distributions/DistributionInfo.json
[wsl-release]: https://github.com/microsoft/WSL/releases/tag/2.7.13
[kernel-config]: https://github.com/microsoft/WSL2-Linux-Kernel/blob/14794180686c2fb6307fbe359c359bec765249f3/arch/x86/configs/config-wsl
[wsl-distro]: https://learn.microsoft.com/en-us/windows/wsl/build-custom-distro
[wsl-networking]: https://learn.microsoft.com/en-us/windows/wsl/networking
[wsl-config]: https://learn.microsoft.com/en-us/windows/wsl/wsl-config
[hyperv-firewall]: https://learn.microsoft.com/en-us/windows/security/operating-system-security/network-security/windows-firewall/hyper-v-firewall
[hyperv-rule]: https://learn.microsoft.com/en-us/powershell/module/netsecurity/new-netfirewallhypervrule?view=windowsserver2025-ps
[wsl-troubleshooting]: https://learn.microsoft.com/en-us/windows/wsl/troubleshooting
[tailscale-wsl]: https://tailscale.com/docs/install/windows/wsl2
[wsl-permissions]: https://learn.microsoft.com/en-us/windows/wsl/file-permissions
[wsl-commands]: https://learn.microsoft.com/en-us/windows/wsl/basic-commands
[wsl-import]: https://learn.microsoft.com/en-us/windows/wsl/use-custom-distro
[wsl-systemd]: https://learn.microsoft.com/en-us/windows/wsl/systemd
[wsl-environment]: https://learn.microsoft.com/en-us/windows/wsl/setup/environment
[schtasks-create]: https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/schtasks-create
[task-logon]: https://learn.microsoft.com/en-us/windows/win32/taskschd/taskschedulerschema-logontype-principaltype-element
[wsl-cli-source]: https://github.com/microsoft/WSL/blob/80697fd42cca3de0c0d5dd1931c36112372a577e/src/windows/common/WslClient.cpp#L1743-L1746
[wsl-core-config]: https://github.com/microsoft/WSL/blob/80697fd42cca3de0c0d5dd1931c36112372a577e/src/windows/common/WslCoreConfig.h
[wsl-oobe-source]: https://github.com/microsoft/WSL/blob/80697fd42cca3de0c0d5dd1931c36112372a577e/src/windows/service/exe/WslCoreInstance.cpp
[fedora-wsl-spec]: https://src.fedoraproject.org/rpms/wsl-setup/blob/5e15d1f0e12f685fe2534de7c846a95bd9d1a4b2/f/wsl-setup.spec
[fedora-oobe]: https://src.fedoraproject.org/rpms/wsl-setup/blob/5e15d1f0e12f685fe2534de7c846a95bd9d1a4b2/f/wsl-oobe.sh
[dnf-upgrade]: https://dnf5.readthedocs.io/en/latest/commands/upgrade.8.html
[dnf-system-upgrade]: https://dnf5.readthedocs.io/en/latest/commands/system-upgrade.8.html
[dnf-offline]: https://dnf5.readthedocs.io/en/latest/commands/offline.8.html
[dnf-offline-source]: https://github.com/rpm-software-management/dnf5/blob/3c44b6252bd3f25a5d3dcc26c353ff2e086e7a1f/dnf5/commands/offline/offline.cpp
[dnf-offline-unit]: https://github.com/rpm-software-management/dnf5/blob/3c44b6252bd3f25a5d3dcc26c353ff2e086e7a1f/dnf5/config/systemd/system/dnf5-offline-transaction.service
[bootc-install]: https://bootc.dev/bootc/bootc-install.html
[mise-trust]: https://mise.jdx.dev/cli/trust.html
[mise-install]: https://mise.jdx.dev/cli/install.html
[mise-exec]: https://mise.jdx.dev/cli/exec.html
[podman-info]: https://docs.podman.io/en/latest/markdown/podman-info.1.html
[soda-runner-native]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/runners/native_create.go#L27-L62
[soda-runner-unit]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/packaging/rpm/runners/sources/systemd/soda-runner@.service
[soda-platform]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/distro/platforms/x86_64.toml
[soda-lock]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/distro/locks/runtime-packages-x86_64.toml
[soda-setup-cli]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/cmd/soda-setup/main.go#L19-L58
[soda-console]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/cmd/soda-setup/console.go#L55-L76
[soda-setup-unit]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/packaging/rpm/runtime/sources/systemd/soda-setup.service#L7-L18
[soda-cockpit-setup]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/cockpit/soda-projects/setup-protocol.mjs
[soda-network]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/setup/native_network.go#L37-L105
[soda-setup-service]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/setup/setup.go#L93-L164
[soda-forgejo-init]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/packaging/rpm/forgejo/sources/forgejo-init
[soda-forgejo-pam]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/packaging/rpm/forgejo/sources/pam/soda-forgejo
[soda-forgejo-unit]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/packaging/rpm/forgejo/sources/systemd/forgejo.service
[soda-manifest]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/acceptance/guest_scripts.go#L84-L124
[soda-fallback]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/acceptance/fallback.go#L26-L72
[soda-baseline-commit]: https://github.com/LevitateOS/soda-os/commit/53674227e10f84c32a911223dc89f08098537ca4
[soda-projects-protocol]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/projects/protocol.go#L18-L28
[soda-workspace-setup]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/projects/coordinator_setup.go#L37-L77
[soda-human-delete]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/projects/people/deletion.go#L29-L58
[soda-removal-test]: https://github.com/LevitateOS/soda-os/blob/53674227e10f84c32a911223dc89f08098537ca4/internal/acceptance/product_scenarios.go#L260-L330
