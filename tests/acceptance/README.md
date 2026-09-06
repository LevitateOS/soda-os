# Matching-native product acceptance

The product outcomes are governed by
[architecture-reset.md](../../docs/architecture-reset.md). Acceptance proves
those outcomes; it does not define them.

## Product evidence boundary

Run the complete suite before release CI on user-controlled matching-native
x86-64 and AArch64 machines. One architecture never qualifies the other.

The suite exercises the exact source commit intended for release and uses the
previous signed published OCI digest for fallback A. It must not rebuild A or
reconstruct historical ISO/QCOW2 artifacts.

After both sibling runs pass, produce one strict signed JSON record containing:

- schema;
- exact source commit;
- acceptance-suite revision or digest;
- both architectures;
- required scenario names and pass results;
- previous fallback OCI digest;
- completion time; and
- approved signer.

Cosign/Sigstore signs the record. It is an authenticated statement about these
pre-release runs, not a claim that release CI's later-built bytes were booted.

## Run reports versus qualification (schema 2)

`run` records only checks that actually returned success, at their call sites.
`summary.json` is a **run report**, not a qualification certificate. Its
`scenarios` object contains named `"pass"` observations; an absent name means
**not established** (failed, not reached, or not covered), never an inferred pass.
A report may be empty or partial. `completed_at` is the report completion time,
not a claim that every required check completed. `failure.txt` retains execution
and cleanup errors when writable; individual captures and `secret-absence.txt`
retain diagnostics. Reporting and sanitization failures also return CLI errors.
Validation failures before source/artifact identity and the evidence directory
are established do not produce a report. Later failures and cancellation retain
a partial report when reporting itself succeeds.

`record` separately requires every check below on **each** architecture before
writing or signing anything. It rejects schema-1 reports and unknown check names;
there is no legacy migration or override flag. The combined signed record also
uses schema 2. Signing authenticates the submitted observations; it adds no
coverage. A zero exit from `run` means its implemented itinerary and finalization
succeeded, not that release qualification is complete.

**Current qualification blocker:** the runner does not establish
`installed-onboarding-observations`, `trusted-lan-access`, or
`public-ingress-rejection`. Consequently its reports cannot currently pass
`record`. Do not remove these requirements or edit reports to invent passes.
A separately scoped implementation must connect the documented installed and
network-topology observations to qualification evidence. This milestone adds
neither public-side probe infrastructure nor a manual-pass input mechanism.

### Coverage map

Evidence paths below are relative to the run evidence directory; capture prefixes
have `.stdout` and `.stderr` files. The native helper/API checks below are not
installed-browser interaction evidence. Fixture changes occur only in disposable
guests. This table maps the product requirements below to checks; it does not
replace those requirements with a new product contract.

| Check | Establishing operation and mutation | Evidence / success boundary |
|---|---|---|
| `iso-first-boot-defaults` | `verifyInitialLocalForwardedAccess`: inspect ISO cloud-init/firewall/welcome defaults, then explicitly allow Forgejo fixture ports | `iso/local-forwarded-before-enrollment`, `iso/administrator-allows-forgejo`, `iso/local-forwarded-forgejo-before-enrollment.txt`; local forwarding, not an independently observed LAN |
| `qcow2-cloud-init-local` | `exerciseReusableQCOW2`: clone/grow disk, provision cloud-init, operator Forgejo signup, create local-only workspace, power down | `qcow2/core`, `qcow2/cloud-init`, `qcow2/volume-growth`, local project setup captures and `qcow2/native-service-state`; not the complete console/browser journey |
| `local-forwarded-access` | `verifyLocalForwardedAccess`: SSH/Cockpit readiness and native service assertions after enrollment; initial local checks already completed | `iso/local-forwarded-before-enrollment`, `iso/local-forwarded-after-tailscale`; probes reach `127.0.0.1` QEMU forwards, not a separate LAN client |
| `tailnet-access` | `verifyTailnetAfterLocalAccess`: SSH/Cockpit readiness, native enrollment state, Forgejo health | `iso/tailnet-after-local-forwarded`, `iso/tailnet-forgejo-after-local-forwarded.txt`; not public-ingress evidence |
| `installed-onboarding-observations` | **Not recorded by runner.** Operator installation/console/browser/reboot checks in Installation and native onboarding below | Requires normal console/welcome, Cockpit key entry and native enrollment, Forgejo advertised URL/clone/refresh and registration-policy observations, plus installed service-ordering evidence; pressing Enter or a helper/API result does not establish this composite |
| `trusted-lan-access` | **Not covered by runner topology.** Separate trusted LAN client | Real SSH/Cockpit, administrator-opened Forgejo/development access before/after enrollment as applicable; forwarding alone is insufficient |
| `public-ingress-rejection` | **Not covered.** Actual public-side probes against cloud deployment | Verify protected services reject public ingress while their Tailnet path works |
| `workspace-boundaries-and-git-keys` | `verifyWorkspaceBoundaries`: inspect seeded accounts/clones/UIDs/keys; append a later personal key | `product/*-workspace-boundary`, `workspace-uids`, Forgejo key captures, `workspace-forgejo-absence`, `workspace-key-copy-once`; every constituent must succeed |
| `ssh-transports` | `verifySSHTransports`: direct workspace command, SCP file, SFTP listing | `product/direct-command`, `scp-content`, `sftp.txt` |
| `development-server-access` | `verifyDevelopmentServer`: run two workspace-owned Python servers, change served content | `product/*development-server*`, `alice-hot-reload-*`; local-forwarded and Tailnet fetches, not browser hot-module replacement or independent LAN evidence |
| `native-mise-ownership` | `verifyMiseOwnership`: concurrent native installation in two workspaces, privacy checks | `product/mise-native-*`, `mise-workspace-privacy`, `cli-ownership-boundaries` |
| `workspace-removal` | `verifyWorkspaceRemoval`: remove Alice's workspace, reject nonadmin project removal | `product/own-workspace-removal`, `nonadmin-project-remove`; surviving peer accounts checked |
| `cockpit-auth-and-independent-roles` | `verifyCockpitAndRoles`: check authentication, promote Alice to wheel, verify Forgejo role remains ordinary | `product/cockpit-status.txt`, `alice-wheel-promotion`, `alice-forgejo-after-promotion`; no browser interaction claim |
| `external-ssh-repository` | `verifyExternalSSHRepository`: create bounded local git-shell host fixture, register key, retry clone, remove project and fixture | `product/external-ssh-*`, `local-guest-external-ssh-fixture-*`; not remote-provider deployment/reachability |
| `project-removal` | `verifyProjectRemoval`: create admin/Bob workspaces, remove project, check accounts gone and canonical repository retained | `product/removable-*`, `project-removal-preserves-forgejo` |
| `human-removal-preserves-forgejo` | `verifyIndependentPersonDeletion`: create person/workspace/owned repository, delete Linux person | `product/obsolete-*`, `linux-deletion-preserves-forgejo`; checks primary and seeded workspace account/home/process absence and retained Forgejo user/repository |
| `update-and-fallback` | `exerciseFallback`: switch B→A→B, verify booted digests and compare snapshots | `fallback/*`; bounded snapshot scope is `preservation.sh` plus native Forgejo authentication, not a full filesystem backup |
| `packaged-boundaries` | Final `captureCore`: packaged/native-service and forbidden-path assertions | `final/core`, `final/tailscale-access`; asserts enumerated boundaries, not absence of every conceivable control plane |
| `runner-completion` | Complete input preparation and implemented itinerary without error | All preceding itinerary operations returned successfully; does not imply the missing observations above |
| `evidence-and-cleanup` | Exact-resource cleanup, cleanup-log write, then credential scan | `cleanup.txt`, `secret-absence.txt`; any cleanup or sanitization failure leaves this check absent |

## Runner ownership and itinerary

Start reading at `internal/acceptance/runner.go`: `Run` prepares inputs, executes
`itinerary.go`, and always finalizes an initialized run. The itinerary is ordinary
ordered Go calls, not a scenario registry or resumable workflow. Read `execute`,
then `exerciseInstalledSystem`, then `exerciseProductScenarios` in that file.
The order is intentional:

1. Verify the published fallback signature and prepare the disposable registry.
2. Install through Anaconda; observe local-forwarded defaults before opening
   fixture Forgejo ports; discover native browser enrollment; recheck both paths.
3. Verify native first-owner signup, capture initial boundaries, and seed the
   kept project with a canonical commit, administrator/Alice/Bob workspaces,
   modified and untracked private files, and native mise tools before fallback.
4. Switch B→A→B and compare returned preservation snapshots. Enrollment stays
   intact across both replacement VM processes.
5. Run product checks in their written order. Inspect workspace keys and append
   Alice's later personal key; exercise SSH, development servers, and concurrent
   mise use; remove Alice's workspace while she is still non-administrative;
   then promote her primary account to wheel and check independent Forgejo roles.
   Subsequent checks use her primary account, never the deleted workspace.
   Exercise the external SSH fixture, project removal, and human deletion last.
6. Capture final boundaries, log out of the ISO guest's Tailnet enrollment, and
   power it down. Only then start the separate reusable QCOW2 guest, provision
   cloud-init, verify its local-only project setup, and power it down.
7. Finalize exact-resource cleanup, sanitize evidence, and write the partial
   schema-2 report. Qualification remains a separate operation.

### Data ownership

- `runner_init.go` returns `runInputs`: usable administrator credentials and
  connection, a personal-key generator, and protected paths for operator prompts.
  Paths are not a second source of operational passwords. The secret collection
  is only a redaction input; checks never retrieve credentials by label.
- `fixtures.go` defines concrete person, workspace, and seeded-project values.
  A person carries its incoming SSH connection/public key and explicit Linux and
  Forgejo credentials. The first owner's passwords are independent; ordinary
  teammate fixtures intentionally use their Linux password through native PAM.
  Workspace setup returns its actual connection and project identity only after
  successful retry and account checks. Linux, not these values, owns current
  account existence and roles.
- `scenarios.go` seeds preservation state and returns the three concrete
  workspaces. Product, identity, external-Git, and fallback checks receive the
  fixtures, remotes, image references, or evidence they use—not the whole runner.
  The sequential itinerary owns which fixtures remain usable after mutations.
- `remote.go` returns `CommandResult` with stdout, stderr, and the execution error.
  `Exchange` separately returns evidence-retention errors. Expected-failure
  assertions must check that retention error before accepting a command failure;
  `Capture` and `Sudo` propagate both. Evidence files are outputs for people,
  never an internal result bus. Setup diagnostics and preservation comparisons
  consume returned bytes, not reopened `.stderr` or snapshot files.

### Preservation and deletion observations

`preservation.go` passes explicit primary/workspace/project triples to the embedded
`preservation.sh`. It captures only those fixture accounts: passwd/shadow hashes,
group membership, actual homes and permissions, incoming/outbound key hashes,
Git HEAD/refs and file contents/metadata, seeded Node binary integrity and native
mise execution. Each canonical repository is freshly cloned and checked with
Git fsck. Catalog, Tailnet identity, active network connections and SSH host keys
are also compared. Forgejo authentication/administrator roles are read through
its native API, not its SQLite schema. Unrelated system groups and accounts are
not preservation fixtures. A failed command aborts the snapshot, including inside
command substitutions. Linux tests execute this script against disposable Git
fixtures and deliberately corrupt each protected class of state.

Deletion checks observe each target's UID and actual home before mutation, then
require native account absence, home/symlink absence and no remaining UID-owned
processes. Project removal also requires catalog absence and preservation of a
committed canonical file; human removal checks both the primary and seeded
workspace. These are observations of bounded fixtures, not a Linux state cache.

### Executed assertions

Remote calls pass literal argv. `remote.go` quotes each argument for OpenSSH's
remote shell, including empty sudo prompts and JSON. Shell programs are explicit
`bash -c` arguments or stdin, never an implicitly interpreted single argument.
Regression tests execute that boundary through a real shell, without a guest.

Absence checks require the native negative result: `getent` exit 2, `grep` exit 1,
or a successful empty systemd unit-file inventory. A transport/lookup failure is
not evidence of absence. Projects rejection requires exit 1 and the expected
product diagnostic; independent owner credentials require an observed HTTP 401.
Owner authentication captures are separate for ISO/QCOW2 and for each password;
PAM signup and later wheel-promotion observations have separate capture names.
These source tests do not establish live installed behavior.

### Native prerequisites and enrollment

Preflight checks the matching QEMU executable/firmware and SSH/SCP/SFTP tools
before guest mutation. Firmware resolution is shared with command construction;
`SODA_QEMU`, `SODA_QEMU_FIRMWARE`, and x86-64 `SODA_QEMU_VARS` remain explicit host
overrides. Acceleration/display support still requires native launch validation.
The fixed fixture ports 18080/18081 cannot collide with configured forwards or
the registry. The administrator explicitly opens those guest development ports
only after first-boot defaults have been checked. Fixture usernames are reserved
only within this disposable suite, not as Soda account restrictions.

Enrollment is observed through the known guest's local SSH connection. No host
Tailscale CLI, whole-peer-set comparison, newly seen peer, or fixed `soda` hostname
is required. Only the guest's own identity/address projection is retained in
`iso/guest-tailnet-enrolled.json`. The client still needs a working route to that
Tailnet address, proven by the separate access checks. Logout ownership begins
before polling so cancellation cannot lose an enrollment completed between polls;
cleanup uses the still-known local connection across B→A→B.

The signup prompt owns its input descriptor and closes it on cancellation. SSH
connection/liveness and development HTTP attempts have explicit native timeouts.
Personal fixture key paths must be new; there is no key-reuse recovery branch.
Fallback A needs its published OCI identity, native platform and OCI file, not
unused historical installer checksums/files. Candidate artifact validation and
published fallback signature/digest checks remain mandatory.

### Guest ownership

`guest.go` owns one guest lifetime: its disk/boot configuration, active VM, and
optional enrollment-attempt cleanup obligation. ISO and QCOW2 have separate
owners, each registered once with `Cleanup`:

- `restart` powers down and replaces a VM without logging out. Failed powerdown
  retains the old process for emergency stop; `LaunchVM` cleans up partial launch
  failures. No `**VM` or run-wide logout callback crosses phase boundaries.
- `shutdown` attempts logout before normal powerdown, even when logout fails.
- `cleanup` attempts logout once and stops the exact remaining process. Both
  attempts have independent bounded contexts, unaffected by itinerary cancellation.
  Repeated guest cleanup retains failures without repeating operations. A failed
  replacement can leave enrollment unreachable; that is reported as a cleanup
  failure, not treated as a successful logout or repaired by a new recovery VM.

The run-level finalizer separately owns the disposable registry and work-directory
cleanup. It preserves operator-owned credential files and retains sanitized run
evidence. The lifecycle tests use native test subprocesses speaking QMP, not real
QEMU guests; they verify ownership, ordering, cancellation, replacement failures,
and ISO/QCOW2 independence without claiming installed-system coverage.

This refactoring changes no qualification requirement, schema-2 check name,
installed-browser coverage, or network-topology claim. See the coverage map and
qualification blocker above.

## Required scenarios

### Installation and native onboarding

After separately authorized builds, run on both matching architectures:

1. Complete graphical Anaconda account creation, reboot, and verify normal login,
   home ownership, administrator privilege, and cloud-init-disabled ISO startup.
   Before firewall changes or Tailscale enrollment, require
   `systemctl is-enabled firewalld.service` → `enabled` and
   `systemctl is-active firewalld.service` → `active`. Require TCP 9090 in both
   runtime and permanent firewall configuration and confirm Cockpit is reachable.
   Confirm the welcome explains that administrators must open Forgejo and
   development ports through Cockpit → Networking → Firewall. The runner records
   these states in `iso/local-forwarded-before-enrollment` and checks the cloud-init disabled
   file. Only then does the suite administrator explicitly open Forgejo TCP
   30000/2222 in the disposable guest for subsequent local-forwarded tests; QCOW2 uses the same
   explicit fixture configuration, recorded as `administrator-allows-forgejo`.
   Those additional ports are not Soda image or installer defaults.
2. Provision QCOW2 through VM tooling; check key/password behavior, network
   access, persistence, and mandatory stateless welcome after
   administrator console login.
3. Start Forgejo before enrollment through the Cockpit Tailscale page. Verify the
   conditional refresh reruns native initialization and advertises the intended
   reachable Tailnet address. After native signup and workspace Git-key
   registration, clone using Forgejo's displayed SSH URL from the intended client.
4. Repeat address, reachability, and clone checks after reboot. Exercise a matching
   address and verify the running Forgejo process remains unchanged.
5. Cover LAN-only provisioning and preserved LAN access after enrollment. Verify
   the complete packaged service graph, including Fedora cloud-init and
   multi-user.target, has no ordering cycle; inspect boot logs for discarded jobs.
6. Verify independent owner credentials, native administrator privileges, and
   later ordinary PAM accounts with self-registration both enabled and disabled
   by team policy. Verify Cockpit key entry, real authorized_keys, one-time
   copying, and incoming workspace SSH.
7. Delete a Linux person through Soda and verify the same-named Forgejo account
   and its data remain. Source tests are not installed-system acceptance.

### ISO firewalld regression evidence

The reported 0.6.3 graphical ISO guest had firewalld enabled even though the
0.6.3 OCI image had it disabled. Inspection of the local x86-64 0.6.3 image
confirmed the package, `89-soda.preset` disable rule, direct image disablement,
and existing welcome warning were present. The old Kickstart `%post` only
created `/etc/cloud/cloud-init.disabled`.

The Anaconda code extracted from the matching installer environment explains
this difference: `modules/network/firewall/installation.py` defaults to
`firewall-offline-cmd --enabled --service=ssh` in the target when no firewall
mode is specified. `modules/boss/installation.py` schedules that configuration
before `%post`. This explicitly enables the service, independently of presets.
Replaying that command in a disposable x86-64 0.6.3 image container changed
`disabled` to `enabled`, creating both the multi-user startup link and the
D-Bus activation alias. The earlier disable-default fix's `%post` action,
`systemctl --root=/ disable firewalld.service`, removed both links and restored
`disabled`; subsequent explicit enablement succeeded. It operates offline in
Anaconda's target chroot, not on the installer's running systemd, so it does
not use `--now` or mask the unit. No runtime hook overrides later admin choices.

That reproduction is image-container evidence, not a newly installed boot.
The reported guest was not accessed or modified. With the subsequent product
decision to retain Anaconda/Fedora defaults, the current `%post` only adds
TCP 9090 and disables cloud-init. The image no longer
overrides firewalld enablement or its native preset. First-boot `enabled`/`active`,
Cockpit access, and administrator-controlled Forgejo access must be observed
through disposable native ISO acceptance on **both x86-64 and AArch64**. The investigation host lacked native
host `qemu-system-x86_64`, `cloud-localds`, and candidate/fallback release records
required by the existing full acceptance runner; no new ISO guest was run.
The sibling AArch64 run must reproduce image and installed-system checks on
AArch64 hardware. Source tests alone do not qualify either installation path.

The revised native-default behavior was replayed in a disposable x86-64 0.6.3
image container using the updated Soda preset: Fedora's preset enabled
firewalld, and adding TCP 9090 before and after Anaconda's firewall command
preserved enablement and the native SSH allowance. The default zone stayed
`public`; its only explicit port was `9090/tcp`, with neither `30000/tcp` nor
`2222/tcp` allowed. This checks offline configuration and idempotence, not a
running firewall or a newly built image.

### Access

- On a trusted LAN, SSH and Cockpit are initially reachable. Verify Forgejo and
  development ports are not allowed by default, then explicitly allow them as
  the administrator and verify direct access.
- In a cloud topology, SSH, Cockpit, and Forgejo are reachable through
  Tailscale and rejected from public ingress.
- Tailscale does not block LAN access.
- On the reusable QCOW2, Projects list and a complete manual-key workspace
  setup succeed over the trusted local path while Tailscale is disconnected.
- A separate Cockpit JavaScript source test verifies that browser SSH guidance
  follows the hostname used to open Cockpit and that Projects returns no
  selected LAN, Forgejo, or Tailnet endpoint. The native runner does not claim
  installed-browser evidence for that presentation behavior.
- A normal development-server link works for a teammate over LAN or Tailscale,
  including hot reload, without Soda port or process tracking.

### Identity and Git

- Linux owns one primary account per person; `wheel` alone owns administrator
  status.
- Development occurs only in derived workspace accounts.
- Native owner-first signup grants independent Forgejo administration. Later
  Linux users authenticate through PAM and receive ordinary accounts.
- Cockpit manages personal authorized keys. Neither Setup nor PAM registers
  those keys with Forgejo.
- Git uses SSH.
- Workspace accounts never become Forgejo users.
- Workspace creation copies only current public authorized keys once.
- Every workspace keeps its outbound private Git key locally. Its public key is
  registered manually through the authoritative Git host's native user
  interface, after which retrying setup completes the clone.
- Native Forgejo API fixture operations use its guest loopback listener rather
  than obtaining an endpoint from Projects.
- External-host SSH behavior uses one bounded `git-shell` account and bare
  repository inside the disposable guest. This proves native SSH repository
  interoperability and manual key registration, not a remote deployment or
  external-network reachability.
- Tea and gh are available and require manual, separate authentication in each
  workspace.
- No Tea token/configuration, gh configuration, private key, or credential is
  copied or retained by Soda.

### Projects, tools, and deletion

- Everyone can view and edit the shared project list without a closed metadata
  field contract or membership model.
- Repositories are created through native Forgejo behavior and then added to
  Projects with `add-existing`; Projects exposes no repository-creation action.
- An edit request omits the canonical URL, an injected URL is rejected without
  changing the catalog, and URL replacement requires administrator removal and
  re-addition. Project removal still preserves the authoritative repository.
- Each person-project pair receives an independent UID, home, full clone,
  dependencies, processes, and mutable state.
- `workspace_exists` follows derived Linux account existence. It is true for a
  retained account before clone retry succeeds and does not claim checkout
  readiness.
- A person can remove only their own workspace.
- Only an administrator can remove a whole project; it deletes the shared Soda
  entry and every local workspace, including uncommitted work, while preserving
  the canonical Forgejo repository.
- Person deletion removes local workspaces then the primary Linux account,
  preserving the same-named Forgejo account and owned repositories.
- Injected failures expose exactly what succeeded and remains; an explicit
  retry continues without rollback or hidden workflow state.
- `mise` is available for people to invoke directly in each workspace; project
  configuration, tool installation, caches, and lifecycle remain upstream-owned.
- Soda exposes no tool picker, install action, parallel tool state, shared mise
  storage, status translation, or cleanup path.

### Updates and absence

- Native manual bootc update preserves authoritative mutable state.
- Fallback to the previous signed OCI digest preserves current accounts,
  groups, homes, catalog, workspaces, Forgejo, Tailscale, and SSH state.
- Automatic updates remain disabled.
- The final system has no Soda daemon, API, identity database, membership
  model, credential broker, SSH gateway, container controller, dependency
  downloader/cache, updater, workflow engine, retry queue, or reconciliation
  loop.

## Failure and evidence

Keep failed evidence concise and free of credentials. Use fresh disposable
machine state for a new attempt. Clean up only exact resources created by the
run. Do not turn retries into durable workflow state.

Normalized preservation evidence excludes volatile timestamps, boot IDs,
process IDs, logs, and raw secret material. It records stable account, group,
home, key, catalog, workspace, Git, Forgejo, Tailscale, SSH, and deployment
facts.

## Go runner

Run one architecture on its matching-native machine from the clean source
revision named by the candidate release record:

```text
go run ./cmd/soda-acceptance run \
  --evidence .artifacts/acceptance/x86_64 \
  --candidate-record PATH --candidate-oci PATH \
  --candidate-iso PATH --candidate-qcow2 PATH \
  --fallback-record PATH --fallback-oci PATH \
  --administrator-private-key PATH \
  --administrator-public-key PATH \
  --administrator-password-file PATH
```

Use `aarch64` as the evidence-directory leaf on an AArch64 host. The private key
and password are disposable test credentials; both secret files must have mode
`0600` or stricter. No Tailscale auth-key file is consumed or accepted: ISO
enrollment uses native browser authentication, and QCOW2 stays unenrolled.
A future fixture that actually consumes an auth key may require a protected
reusable ephemeral key; the current flow must not request an unused secret.
The ISO uses only the installer. For the QCOW2 local-forwarded fixture, the runner
uses cloud-localds to deliver native cloud-init user-data automatically; install
cloud-localds and openssl on the matching host. Protected fixture files stay in
the disposable work directory and are removed during cleanup. The operator
creates the ISO Linux administrator in Anaconda, logs in, configures the network,
and adds the personal key through Cockpit Accounts. The runner prints only the protected input
paths, then resumes through native SSH and Tailscale readiness.

The run leaves `summary.json` with the observations it established and normalized
credential-free evidence, but removes its exact QEMU processes, disposable
loopback registry, generated keys/passwords, and VM disks. Operator-owned input
credential files are not deleted. A failed run retains a partial report and
sanitized diagnostics when possible, and reports cleanup failures explicitly.
Reporting failure returns an error rather than claiming a report was written.

Only reports with complete qualification evidence can be combined and signed.
The current runner alone cannot supply that evidence (see the blocker above).
Once matching x86-64 and AArch64 reports establish every required check and name
the same source and suite revisions, the signing interface is:

```text
go run ./cmd/soda-acceptance record \
  --x86-summary PATH --aarch64-summary PATH \
  --x86-release-record PATH --aarch64-release-record PATH \
  --expected-revision EXACT_MAIN_SHA \
  --output PATH \
  --approved-signer SIGSTORE_CERTIFICATE_IDENTITY \
  --oidc-issuer SIGSTORE_OIDC_ISSUER
```

Both candidate release records use the strict schema-3 decoder and must bind
the corresponding run's candidate digest, native platform, and exact main
revision. Both summaries must name that revision as their source and suite.
The maintained `Native acceptance evidence` workflow accepts base64-encoded
copies of those four credential-free inputs, invokes this command with its exact
`main` SHA, signs and verifies the combined record, and retains the six small
files for one day. It does not
run QEMU, receive a guest Tailscale credential, publish an image, or create a
release.

Release CI consumes that artifact before native preparation:

```text
go run ./cmd/soda-acceptance verify --record PATH --expected-revision EXACT_PRODUCTION_SHA
```

The adjacent `PATH.sigstore.json` bundle is required. Trust is fixed to the
maintained main signing workflow, not supplied by the record or a CLI override.
Both siblings must qualify at the exact production SHA; a merge commit or merely
equivalent source tree is not accepted as a substitute. Duplicate report/check
fields, unknown fields, mismatched identities and signature failures are errors.
Capture writes are exclusive: an accidentally reused label fails rather than
overwriting an earlier observation. Returned error text is redacted as well as
retained files; error identity remains available to programmatic callers.

The runner QCOW2 fixture covers cloud-init through QEMU loopback-forwarded
access, not an independent trusted-LAN client. The late-enrollment Tailnet, native Cockpit key UI, first-signup, registration
policy, and reboot matrix above still requires separately recorded installed
acceptance; a runner summary alone does not prove those interactive checks.

Run `sudo tests/acceptance/check-native-service-ordering.sh` on each installed
candidate after provisioning, and repeat after reboot and on the cloud-init-disabled
ISO. It inspects the actual Fedora and Forgejo units and boot journal.
Before prompting for owner signup, the runner probes the empty homepage and
attempts early web/API login using valid fixture Linux credentials. It requires
the visible owner entry, API HTTP 401, and a web redirect to native registration.
The runner then pauses for native first-owner signup, verifies the owner role
and independent credentials, and only then creates teammate fixtures. Captures
are `iso/owner-entry-*` and `qcow2/owner-entry-*`; the subsequent successful
first-owner registration must prove the early requests did not consume ownership.

### First-owner regression checks

On matching-native Soda hardware, with Python 3, OpenSSL, util-linux namespaces,
the packaged PAM stack, `git` account and `soda-forgejo-shadow` group available:

```sh
sudo unshare --mount --net --pid --fork --kill-child --mount-proc \
  --propagation private python3 tests/acceptance/check-forgejo-first-owner.py \
  --binary /usr/bin/forgejo --custom /usr/share/soda/forgejo/custom
```

Use the candidate's actual effective custom tree if testing an operator override.
The script requires a private PID namespace, uses temporary namespace-private
Linux credential files and databases, keeps SELinux enforcing, and cleans up
its fixtures. It does not modify real accounts, roles, services, or configuration.
It covers web/API/Git-over-HTTP entry, signup retry, competing registration,
independent owner passwords, ordinary PAM peers, workspace exclusion, no retained
PAM verifier, source activation choices, registration policy, process restart,
and the existing-accounts/no-administrator diagnostic. It does not establish OS
reboot, image-update, or complete browser usability acceptance.

For browser acceptance, start a separate empty matching-native Forgejo process
with the staged custom tree, Soda theme defaults, and active Soda PAM source:

```sh
node scripts/check-forgejo-owner.mjs http://127.0.0.1:PORT .artifacts/branding/owner
```

**This browser check creates an administrator through native registration. Use
only a disposable instance.** It generates its own temporary credential in
memory, checks light/dark/mobile presentation, early sign-in guidance and
API/Git rejection, failed signup/retry, administrator confirmation and navigation,
and established-instance sign-in. Do not run it on the server an operator is
about to claim. The native fixture additionally tests valid Linux credentials;
the browser check is not a substitute for the PAM matrix.

The native welcome and separate Cockpit Tailscale page require the installed
checks in [native installation acceptance](../../docs/native-onboarding.md#installed-acceptance).
Browser-authentication and exit-node evidence must exercise the real native
flow; auth-key fixtures and command-unit tests do not substitute for it.
