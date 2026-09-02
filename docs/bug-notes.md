# Soda OS bug and incident notes

This file records failures that taught us something reusable. It is not a
replacement for the product contract, architecture record, issue tracker, or
acceptance suite. Normative requirements remain in `principles.md` and
`architecture-reset.md`; ownership interpretation remains in
`ownership-and-decision-discipline.md`. A note here explains what failed, what
evidence separated the real cause from plausible stories, which owner should
fix it, and how to avoid repeating the investigation.

## How to write a useful note

For every material bug, capture:

- the user-visible or operator-visible symptom;
- the outcome that was expected;
- the exact artifact, commit, architecture, and environment exercised;
- the earliest boundary known to have passed;
- the first boundary known to have failed;
- direct evidence, including exact errors or state where safe;
- hypotheses considered and the evidence that accepted or rejected each one;
- the authoritative upstream owner of the failing behavior;
- the smallest correction and why it belongs at that boundary;
- rejected fixes that would duplicate ownership or expand Soda;
- focused proof, artifact proof, and installed-system proof separately;
- remaining matching-architecture or release-level verification;
- whether any secret, persistent data, or destructive operation was involved.

Do not write "the installer is broken" when the evidence only proves that one
post-install label is wrong. Do not write "fixed" when only a unit test or image
build has passed. Do not turn a current path, package version, workaround, or
test topology into permanent architecture.

## Evidence vocabulary

Use these terms consistently:

- **Source proof:** inspection or tests show what code is intended to do.
- **Build proof:** the exact source produced an artifact successfully.
- **Artifact proof:** inspection shows the intended bytes are in the artifact
  and forbidden bytes are absent.
- **Installation proof:** a fresh machine installed from the exact ISO reaches
  the installed system.
- **Behavior proof:** the installed system performs the user-visible action.
- **Release proof:** matching-native evidence exists for every supported
  architecture and every release gate.

A later level does not follow automatically from an earlier one. In particular,
a successful command exit is not proof of its intended side effect.

## Installer-reset change timeline

This timeline makes superseded attempts explicit. A commit being historically
useful does not mean its mechanism remains accepted.

| Commit | What it established |
|---|---|
| `e7caff5` | Added protected OEMDRV answer-media generation. |
| `7911fd5` | Split the generator into cohesive validation and publication code. |
| `ab10584` | Activated stock Kickstart composition and deleted the custom add-on. |
| `13a2f2f` | Added hook, media, ejection, and installed-absence tests. |
| `0967780` | Documented the protected answer-media trust boundary. |
| `f00ebd7` | Tried Soda-owned home relabeling; this mechanism was superseded. |
| `dd10601` | Removed the acceptance-only hostname override. |
| `6711b5e` | Returned relabel ownership to Anaconda. |
| `2e5c596` | Exposed SELinuxFS so Anaconda's own relabel task could work. |

## 2026-09-02: Anaconda bootc installs left administrator SSH files mislabeled

### Intended outcome

Stock Fedora Anaconda should own installation and initial Linux-account
creation. Protected OEMDRV input should provide the administrator username,
password, SSH public key, and one-use Tailscale key. After installation:

- the Linux administrator should exist in `wheel` with a normal home;
- the standard `~/.ssh/authorized_keys` should permit ordinary OpenSSH login;
- Forgejo should contain the same-named site administrator;
- Tailscale should enroll once and delete its handoff;
- the installed image should remain SELinux-enforcing;
- Soda should not own a relabel service, account database, bootstrap daemon, or
  recovery workflow.

### What happened

A fresh x86-64 installation completed. The bounded finalizer created the
Forgejo administrator, first boot enrolled Tailscale successfully, password SSH
worked, and the administrator account and authorized key were present.
Key-based SSH failed.

The physical home tree had these actual SELinux types:

```text
/var/home/soda-test                         var_t
/var/home/soda-test/.ssh                    var_t
/var/home/soda-test/.ssh/authorized_keys    var_t
```

The installed Fedora policy expected:

```text
/var/home/soda-test                         user_home_dir_t
/var/home/soda-test/.ssh                    ssh_home_t
/var/home/soda-test/.ssh/authorized_keys    ssh_home_t
```

This was not a missing key, bad key, SSH configuration, Tailnet routing, user
creation, or password problem. The key existed and validated with
`ssh-keygen`; password SSH reached the same installed account; only the label
boundary was wrong.

### Misleading evidence

Anaconda invoked `restorecon` during its user task and the command returned
zero. That looked like successful relabeling, but the inode labels remained
`var_t`.

The installed Anaconda logs also did not explain the final relabel. Exact source
inspection showed why: Fedora Anaconda 44.30 schedules its work in this order:

1. Kickstart post scripts;
2. installer-log copying;
3. final `SetContextsTask` relabeling.

The logs are copied before the final context task runs. Their silence cannot be
used as evidence that the task did or did not run.

General lesson: before trusting a success status, observe the owned state that
the command was meant to change. Before trusting logs, prove where log capture
sits relative to the operation in question.

### First attempted correction: wrong owner

Commit `f00ebd7` added `restorecon` and `matchpathcon` calls to Soda's
installer finalizer. Focused tests proved command construction and path
validation, but they did not prove that libselinux could operate inside the
actual bootc target chroot. The approach also moved relabel ownership from
Anaconda/Fedora into Soda, contrary to the accepted boundary.

Commit `6711b5e` removed that correction and returned relabeling to Anaconda.
This was the right architectural rollback, but it did not by itself repair the
upstream execution environment. A new fresh install still had `var_t` labels.

Lesson: a mock that observes an attempted command does not prove the command's
platform preconditions or side effects. Do not let a convenient test seam turn
an upstream responsibility into a Soda responsibility.

### Root cause proof

Exact Fedora Anaconda 44.30 source showed that
`PrepareBootcMountTargetsTask._handle_api_mount_points()` did this:

```python
for path in ("/proc", "/sys"):
    sysroot_path = self._sysroot + path
    safe_exec_program("mount", ["--bind", path, sysroot_path])
    self._internal_mounts.append(sysroot_path)
```

`mount --bind /sys ...` is non-recursive. It exposes the `/sys` directory tree
but not the nested `selinuxfs` mount at `/sys/fs/selinux`. The target chroot
therefore had a directory at that path, but no SELinuxFS mount.

A disposable mount-namespace experiment proved the causal relationship:

- with `/sys/fs/selinux` mounted in the target, the target's `restorecon`
  changed a probe home from `var_t` to `user_home_dir_t` and `ssh_home_t`;
- with SELinuxFS absent, the same target `restorecon` returned zero and left the
  probe tree as `var_t`.

The installer journal also contained:

```text
chage: could not determine enforcing mode: No such file or directory
```

That was consistent supporting evidence, not the primary proof.

### Smallest correction

Commit `2e5c596` applies an installer-only, exact-version-guarded correction to
the pinned Fedora source:

```python
for path in ("/proc", "/sys", "/sys/fs/selinux"):
```

The existing Anaconda loop performs the bind and records every target in
`_internal_mounts`. Existing reverse teardown therefore unmounts nested
SELinuxFS before `/sys`. Existing `SetContextsTask` remains responsible for
relabeling `/var/home` after `%post`.

The build fails closed unless all of these remain exact:

- owner package: `anaconda-core-0:44.30-2.fc44`;
- original module SHA-256:
  `614ac3f3061d959144e0a2e80919012c7254d44b1fab04daea35b2bef52f3f86`;
- original defective line occurs exactly once;
- patched module SHA-256:
  `de1400f91d39bcdba5f34d17b4173ef779c9d890e3ac404565d0c781026163de`.

The patcher enters the build through a read-only BuildKit bind mount. It is not
copied into the installer image or runtime. Both normal and optimized Python
bytecode are regenerated. ISO inspection extracts the installed Anaconda module
and rejects an ISO without the reviewed tuple.

### Rejected alternatives

- Do not add Soda-owned `restorecon`, `setfiles`, or a relabel service.
- Do not disable SELinux or weaken the installed policy.
- Do not change the standard home path to avoid labeling the real path.
- Do not use first-boot autorelabeling; it defers an installer invariant and
  creates different boot and failure semantics.
- Do not use `--rbind /sys` while tracking only `/sys`; nested mounts then make
  plain teardown incomplete.
- Do not leave an ad hoc `%post` mount untracked when Anaconda already owns the
  bootc mount lifecycle.
- Do not bind-mount a replacement `/var` or search deployment directories.

### Installed proof completed on x86-64

A fresh native x86-64 raw-QEMU installation from commit `2e5c596` and ISO
SHA-256
`696f8708011dae6ee4426b5068dbcf6c310e0a379c3e54a35ed030b86ebfc49c`
showed:

```text
matchpathcon -V /home/soda-test
matchpathcon -V /home/soda-test/.ssh
matchpathcon -V /home/soda-test/.ssh/authorized_keys
```

all succeeding. Key-based SSH logged in directly as `soda-test`. The same
installation proved the exact Forgejo administrator, successful Tailscale
enrollment followed by handoff deletion and unit disablement, absence of both
installer-only executables from the runtime, and the exact booted runtime image
digest recorded in the release record.

The same machine then created a native empty Forgejo repository, synchronously
set up its derived workspace, accepted direct workspace SSH, accepted primary
Cockpit login, rejected workspace Cockpit login, and passed the complete
primary-and-workspace immutable-toolset capture. This is x86-64 behavior proof,
not AArch64 release proof.

## 2026-09-02: the acceptance harness used a hostname the product did not own

### What happened

The unattended runner defaulted the installed guest endpoint to
`soda-acceptance`, while the distribution and installer set the product
hostname to `soda`. The harness then waited for an identity the installed
product never advertised.

### Correction

Commit `dd10601` changed the default guest host to `soda` and removed an
unrelated host-name parameter from answer-media construction.

### Lesson

Test topology must consume the product's real identity source. Never invent a
second hostname, URL, port, account name, or path solely to make a harness
convenient. When readiness fails, compare the endpoint the harness is polling
with the identity the installed product actually owns before debugging the
service itself.

For fresh Tailnet tests, record the peer set before launch and identify the new
online node by ID and IP. Do not select an arbitrary old peer merely because it
has the same reusable hostname.

## 2026-09-02: protected input media must have a proved destruction boundary

### Risk

The OEMDRV disk necessarily contains the plaintext installer password and
Tailscale auth key. Attaching it for the entire installation, deleting it before
the guest releases it, or recording its contents as evidence would all violate
the intended trust boundary.

### Accepted boundary

- The host generator accepts secrets only from protected regular files or a
  terminal, never from argv or environment values.
- Its temporary directory is mode `0700`; the answer ISO is created atomically
  with host mode `0600`.
- The guest mounts the exact OEMDRV medium read-only with `nodev`, `nosuid`, and
  `noexec`, validates fixed bounded files, copies values into installer tmpfs,
  then unmounts and ejects the medium.
- QMP first observes that the guest opened and unlocked the exact tray. A
  virtual eject may still leave the backing medium attached. If it does, the
  host uses `blockdev-remove-medium` on that exact QEMU device, queries the
  device again, and requires an open, unlocked tray with no inserted medium
  before deleting that exact disposable image.
- Installed capture must prove that no answer medium, raw key, plaintext
  password, generated Kickstart, or installer handoff remains.

### Lesson

"The guest asked to eject" is not enough. Neither is an open tray by itself.
Destruction follows observed release of the exact resource. Cleanup must be
narrow: delete the one generated answer image, never a glob or a shared artifact
directory, and never the user's source key file.

## 2026-09-02: terminal automation exposed a disposable password in output

### What happened

An early diagnostic used `expect` to drive password SSH. The pseudo-terminal
echoed the disposable administrator password into captured command output.

### Correction

Do not use an interactive PTY for secret-bearing diagnostics. The safer local
diagnostic used an askpass program that read the protected acceptance password
file, while fixed remote input was sent over ordinary stdin. Production
Forgejo and Projects flows continue to accept the password only in the
unprivileged coordinator's stdin payload.

### Lesson

- A secret absent from argv can still leak through terminal echo.
- Never print, `set -x`, interpolate, log, archive, or include secrets in
  evidence.
- Prefer the product's real noninteractive secret channel.
- When a diagnostic cannot avoid a secret, keep the secret in a protected file
  and make the helper read that file without emitting its contents.
- Treat captured tool output as a possible disclosure surface.

The leaked value belonged only to a disposable acceptance administrator. That
does not make the method acceptable for repetition.

## 2026-09-02: chroot changes path meaning

A discarded finalizer experiment entered `/mnt/sysroot` with `chroot` and then
continued to address `/mnt/sysroot/var/...`. Inside that chroot, the prefix no
longer names the installed root; it names a nonexistent nested path. The
experiment also did not solve mount persistence: changing pathname resolution
does not change which filesystem backs a path.

Lessons:

- Draw the mount namespace and root-directory view separately.
- After `chroot(ROOT)`, target paths begin at `/`, not at `ROOT` again.
- Prove filesystem identity from mount information and persistent post-boot
  state, not from similar-looking path strings.
- Do not use chroot as a substitute for understanding bootc's physical root,
  deployment root, and persistent `/var` mounts.

## 2026-09-02: install-time `/var` cannot be inferred from post-boot paths

### What happened

One earlier custom-add-on attempt created the Linux and Forgejo administrators,
but first-boot Tailscale enrollment could not open:

```text
/var/lib/soda-install/tailscale-auth-key
```

The handoff did not survive into the installed system. The enrollment unit then
performed its required cleanup and disabled itself; repeated enrollment was not
the recovery mechanism.

### Why this took too long

After boot, `findmnt -T /var` correctly showed persistent ext4-backed bootc
state. That did not prove that the installer had written through the same mount
earlier. Installer-only `/mnt/sysroot`, Anaconda's physical root, a deployment
root, and the booted runtime root are different views. Similar path suffixes do
not prove identical backing storage.

The custom add-on alternated between trusting Anaconda's target `/var`, scanning
`ostree/deploy`, selecting a discovered deployment `var`, bind-mounting it over
the target, removing that machinery, and adding it again. That churn was itself
evidence that Soda had taken ownership of bootc/Anaconda storage topology.

### Current rule

The replacement finalizer accepts only Anaconda's fixed `/mnt/sysroot/var`. It
resolves the path within the installed system and verifies the exact read-write
mount through `/proc/self/mountinfo`. It does not scan deployments, choose a
deployment, construct a second `/var`, or bind-mount a replacement variable-data
tree.

The Tailscale key is the final target mutation, after Linux-account validation,
Forgejo creation, and transient-process cleanup. First boot must prove both
successful native consumption and deletion of `/var/lib/soda-install`.

Lesson: successful installer code is not evidence that a durable object reached
the filesystem consumed after reboot. Verify that exact object after first boot,
verify its native consumer, and verify its required deletion.

## 2026-09-02: installer scratch is not target state

The bootc payload import exceeded the small LiveOS overlay. An early approach
changed an Anaconda Payloads D-Bus service environment to redirect temporary
data. Exact process/path evidence instead showed that pinned bootc used the live
installer's `/var/tmp` for the large import.

The retained fix is one installer-only 4 GiB tmpfs mounted at `/var/tmp` before
Anaconda starts. It is scratch, not the installed system's `/var`, and installed
capture requires both the mount unit and `container_images_*` scratch to be
absent.

When an installer runs out of space:

- identify the exact process and exact write path;
- distinguish LiveOS overlay, installer tmpfs, target storage, and host
  container storage;
- do not redirect scratch into persistent product state;
- do not replace a complete D-Bus service to fix one directory's capacity;
- prove the scratch resource disappears from the installed product.

## 2026-09-02: keyboard automation is a state machine disguised as input

The earlier x86 harness waited for a GRUB prompt and injected keys to append
`inst.cmdline`. Other diagnostics showed how timing drift can send intended
shell commands into a login prompt instead. Keyboard injection depends on exact
screen state, focus, timing, and firmware behavior.

The protected-answer-media path now puts unattended storage behavior in the
secret-free Kickstart and selects the mandatory OEMDRV Kickstart through fixed
ISO boot arguments. The old QMP key injector is gone.

Use QMP for machine-state operations and evidence, not for typing credentials or
product configuration. If behavior can be expressed in the built artifact or a
standard installer protocol, do that. Keep the interactive development install
interactive; keep test-only automation explicit and separate.

## 2026-09-02: source success and artifact success were repeatedly conflated

Several different objects existed during the work:

- repository source at a particular commit;
- a reused runtime OCI archive whose payload had not changed;
- a rebuilt installer-environment OCI archive;
- a rebuilt ISO containing that installer environment and the exact runtime
  payload;
- a release record binding runtime digest and ISO checksum;
- a protected OEMDRV answer image;
- a fresh qcow2 installed from the exact pair.

Changing installer source does not change the runtime OCI, so reusing a
validated runtime archive is correct. It does require rebuilding the installer
environment, ISO, checksum, and release record. Conversely, passing `just
check` does not update any artifact. An ISO built before a final source edit is
stale even when the edit only tightens a build guard.

Practical rules:

1. Record the commit before starting an artifact proof.
2. Validate every reused input by checksum, platform, exact image digest, and
   relevant inventory.
3. Rebuild from the lowest changed ownership layer upward.
4. Put each candidate in a uniquely named directory.
5. Generate a new release record for the exact candidate ISO.
6. Never infer installed behavior from an earlier disk.
7. Use a fresh qcow2 after installer behavior changes.
8. Keep AArch64 and x86-64 artifacts and validation on matching-native hosts.

A root-owned installer archive can also make an ordinary `sha256sum` fail with
`Permission denied` after all source checks pass. That permission error is an
artifact-access problem, not a failed `just check`. Preserve the distinction in
status reports and use the narrow read-only privilege required to inspect the
exact file.

One x86-64 runtime archive also had a stale adjacent checksum sidecar. The
archive itself computed as:

```text
6870033abb4e992e93d5baa1fb9b5b712b5da6c219302c31217f9b45b19c434a
```

The sidecar's different older value was not evidence for the current bytes.
Recompute hashes from the object being reused; do not trust its name, timestamp,
directory suffix, or neighbor file.

Runtime revision and installer revision are also distinct. Rebuilding only the
installer can correctly retain the runtime OCI's older `source_revision` while
producing a different ISO. Identify an installer candidate by its repository
commit and exact ISO checksum, and record the embedded runtime digest
separately.

For the passing local x86-64 candidate:

```text
installer source commit:
2e5c596cf5613c6e32a246e6d9ce41c85d87e3b2

runtime OCI tar SHA-256:
6870033abb4e992e93d5baa1fb9b5b712b5da6c219302c31217f9b45b19c434a

embedded runtime manifest digest:
sha256:18c7463371d4b8a8b1b438501862eec30970288a95e2fdb714b189e4faa565be

installer-environment OCI tar SHA-256:
7ead9a2f37c41c964209ad0027820d3d094d4195c5021dda0f672c1ec357d4d5

installer ISO SHA-256:
696f8708011dae6ee4426b5068dbcf6c310e0a379c3e54a35ed030b86ebfc49c

release-record SHA-256:
6c9d94fb93f71423a38548a5c9c97c3b37fecb36c0d4153159b3dc6d81c043ab
```

These are local debugging/acceptance artifacts, not published release claims.

## 2026-09-02: mock coverage proved attempts, not platform effects

The first relabel implementation had focused tests that verified:

- a fixed physical home path;
- exact `restorecon` and `matchpathcon` argv;
- failure propagation.

Those tests were useful but insufficient. They mocked `subprocess.run`, so they
could not show whether the real target chroot exposed SELinuxFS or whether inode
labels changed.

For platform-sensitive code, add an evidence ladder:

1. unit test fixed inputs, validation, ordering, and failure propagation;
2. inspect the exact upstream version and its call order;
3. run a disposable mechanism probe at the real mount/namespace boundary;
4. build and inspect the exact artifact;
5. install on matching-native hardware;
6. observe the final owned state and user-visible behavior.

Do not delete unit tests because they are insufficient. State what they prove,
then add the missing level of evidence.

## 2026-09-02: partial success is not milestone success

The first stock-Kickstart x86-64 run proved OEMDRV handling, installation,
Linux account creation, password SSH, Forgejo creation, Tailscale enrollment,
and credential cleanup. It still failed key-based SSH because of the SELinux
labels. Reporting only "installation passed" would have hidden the defining
remaining failure; reporting only "SSH failed" would have discarded useful
passing evidence.

Keep a boundary table during long runs:

| Boundary | Result |
|---|---|
| Source and focused tests | pass/fail |
| Installer environment build | pass/fail |
| ISO inspection | pass/fail |
| OEMDRV guest ejection and host removal | pass/fail |
| Installation and first boot | pass/fail |
| Linux administrator and password | pass/fail |
| Forgejo administrator | pass/fail |
| Tailscale enrollment and handoff deletion | pass/fail |
| Public-key SSH and SELinux labels | pass/fail |
| Cockpit primary login/workspace rejection | pass/fail |
| Projects setup and direct workspace SSH | pass/fail |
| Immutable toolset and rootless Podman | pass/fail |
| Obsolete-state absence and booted digest | pass/fail |

Report the first failed boundary while retaining the exact earlier passes. Mark
the milestone complete only after every required row passes on one coherent
final installed candidate.

## 2026-09-02: stopping rules can become a failure mode

During this milestone, self-imposed attempt counts, arbitrary wall-time limits,
and a "no rebuild" rule prevented progress even though the user had authorized
continued work through recoverable build and harness failures. Those limits did
not protect product data or an ownership boundary; they created idle time and
forced repeated coordination.

Use stop conditions only for a real boundary:

- a change would require a new product decision;
- a change would broaden persistence, privilege, authority, trust, or
  destructive semantics;
- authoritative data could be destroyed;
- a required secret or matching-native machine is unavailable;
- evidence demonstrates that the accepted upstream mechanism cannot satisfy
  the product outcome.

Do not stop merely because:

- a build takes longer than expected;
- one disposable run fails;
- an artifact must be rebuilt after an authorized source correction;
- a harness defect can be corrected inside the accepted test boundary;
- a turn or context boundary is reached.

At a turn boundary, report the last verified checkpoint and continue. If work
really is blocked, name the exact boundary and the smallest decision or input
needed. "Not ready" is a status, not a reason to idle.

## Ownership lessons from the Anaconda add-on deletion

The removed Soda Anaconda add-on depended on private Anaconda Python APIs, a
custom D-Bus service/interface, GTK spoke state, Glade presentation, task
objects, custom user-list mutation, deployment-tree discovery, custom
persistent-`/var` mounting, and explicit account orchestration. Each seam could
change independently upstream, and failures crossed UI, bus, payload, storage,
and target-root boundaries.

The replacement keeps the product-specific composition narrow:

- stock Anaconda owns interactive storage, networking, bootloader, bootc
  deployment, and native Kickstart account creation;
- protected OEMDRV media owns the temporary answer handoff;
- one fixed `%pre` generates native account directives;
- one bounded `%post --nochroot` finalizer performs only Forgejo initialization
  and final Tailscale handoff;
- first boot performs one native Tailscale enrollment attempt;
- recovery is a fresh installation, not durable workflow machinery.

The SELinuxFS defect does not justify bringing back the add-on or replacing
Anaconda wholesale. Patch the smallest verified upstream seam, guard it against
version drift, and delete the patch when Fedora carries the fix.

## Checklist for the next installer investigation

- [ ] Record commit, architecture, input hashes, and exact package versions.
- [ ] Use a new artifact directory and fresh qcow2.
- [ ] Confirm whether the failure is in live installer, installed target, first
      boot, Tailnet discovery, or post-install acceptance.
- [ ] Identify the last passed boundary and first failed boundary.
- [ ] Read the exact shipped upstream source before extending a private seam.
- [ ] Inspect mount namespace, chroot root, and physical filesystem identity
      separately.
- [ ] Verify side effects, not only exit status.
- [ ] Check upstream task ordering before interpreting copied logs.
- [ ] Keep passwords and keys out of argv, environment values, PTYs, logs, and
      evidence.
- [ ] Prove OEMDRV ejection with QMP before host deletion.
- [ ] Preserve canonical Forgejo repositories after later local failure.
- [ ] Reuse runtime artifacts only when runtime payload ownership is unchanged
      and exact validation passes.
- [ ] Rebuild every changed layer above the changed source.
- [ ] Run installed behavior checks over the product's real Tailnet identity.
- [ ] Record what remains unverified on the sibling architecture.
- [ ] Continue through recoverable failures; stop only at a genuine authority,
      data-safety, infrastructure, or architecture gate.

## Reusable bug-note template

```markdown
## YYYY-MM-DD: concise symptom

### Expected outcome

### Exact artifact and environment

### What happened

### Last passed boundary

### First failed boundary

### Direct evidence

### Hypotheses rejected

### Root cause

### Authoritative owner

### Smallest correction

### Rejected broader fixes

### Verification completed

### Verification still required

### Secret, data-loss, and destructive-action notes

### Rule we will reuse
```
