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

## 2026-09-02: tmpfiles and SELinux left Forgejo PAM access unapplied

### Expected outcome

The installed image grants only the Forgejo service process read access to
`/etc/shadow`: the file is `root:soda-forgejo-shadow` mode `0040`, the `git`
account is not an NSS member of that group, and `forgejo.service` receives the
group through `SupplementaryGroups`. A later primary user should then be able
to authenticate through Forgejo's native PAM source while a workspace account
remains rejected.

### Exact artifact and environment

The failure was reproduced on the native x86-64 image built from candidate B
commit `c3d296e` in a fresh disposable raw-QEMU installation. SELinux was
enforcing and the service process had the expected supplementary group.

### What happened

Wrong-password authentication correctly returned HTTP 401, but the correct
Linux password also returned 401. The installed `/etc/shadow` was
`root:soda-forgejo-shadow` mode `0000`, and PAM's password check reported an
unknown user.

### Last passed boundary

The image contained the dedicated group, the service-only supplementary-group
grant, the intended PAM stack, and the named tmpfiles rule. Forgejo ran in its
expected SELinux domain, and no matching AVC denial was recorded.

### First failed boundary

The intended tmpfiles side effect had not occurred. The global
`systemd-tmpfiles-setup.service` exited with status 73 on an unrelated rule for
the immutable `/usr/local/sbin` path before the installed system reached the
required shadow-file mode.

### Direct evidence

Running `systemd-tmpfiles --create forgejo.conf` manually from the unconfined
administrator domain changed only the intended rule and restored `/etc/shadow`
mode `0040`. The same protected correct-password request then returned HTTP 200
and Forgejo created one active, non-administrator Alice user. Subsequent
`useradd`, `chpasswd`, password locking, and `userdel` probes preserved the
group and mode.

That result was incomplete: invoking the same command from a systemd service
returned success but left mode `0000`. With SELinux temporarily permissive, the
service invocation produced mode `0040`. Disabling SELinux `dontaudit` rules
then exposed the exact enforcing denial from `systemd_tmpfiles_t` to
`shadow_t`: file permissions `{ getattr setattr }`. A test policy granting only
those permissions made the service invocation work under enforcing SELinux.

### Root cause and ownership

The package first relied on the success of the system-wide tmpfiles pass, so a
failure in an unrelated rule prevented Forgejo's package-owned precondition.
Moving the named rule to Forgejo initialization removed that coupling, but the
SELinux policy intentionally prevented the tmpfiles domain from inspecting or
changing shadow metadata. The command's successful exit did not prove its side
effect. Linux/PAM still own password verification; Soda owns only composition
of the explicitly authorized service privilege.

### Smallest correction

The existing root-owned `forgejo-init.service` runs
`systemd-tmpfiles --create forgejo.conf` before its existing initialization and
before `forgejo.service`. The image also installs one SELinux module allowing
`systemd_tmpfiles_t` only `getattr` and `setattr` on `shadow_t`. This reuses the
package's single declarative rule and existing ordering boundary while granting
no content read or write permission. It adds no daemon, service, executable,
identity state, verifier copy, credential path, or generic privilege mechanism.

### Rejected broader fixes

Do not add `git` permanently to a shadow-reading NSS group, add a password
helper or broker, copy shadow records, disable SELinux, make Forgejo unconfined,
allow the tmpfiles domain to read or write shadow contents, or attempt to make
Soda repair every global tmpfiles failure.

### Verification status

Focused image/package tests, both architecture source contracts, policy
compilation, and `just check` pass at production commit `b2faeb3`. The
disposable installed probe proves the exact policy under enforcing SELinux.
Fresh x86-64 artifacts, complete B-to-A-to-B acceptance, and matching-native
AArch64 repetition remain required before release-level completion.

### Rule we will reuse

A declarative rule being present and its command exiting successfully are not
evidence that the side effect occurred. Apply a package-owned tmpfiles invariant
at the existing service initialization boundary, verify installed state, and
when SELinux blocks it, derive the smallest policy from the exact denial rather
than changing privilege owners or disabling enforcement.

## 2026-09-03: real Tea staging retained its native lock file

### Expected outcome

After Tea authenticates a newly added primary human and creates that human's
Forgejo-owned PAT, the privileged publisher accepts only the exact protected
Tea staging shape and copies only the opaque `config.yml` into the new home.

### Exact artifact and environment

The failure occurred in a fresh native x86-64 raw-QEMU installation of commit
`5b1d7be`. Stock Anaconda installation, protected answer-media ejection,
first-boot Tailscale enrollment, and readiness all passed before the first
Add Person operation reached `human-publish`.

### What happened

Tea authenticated successfully and retained protected staging, but the helper
reported `staging directory contains unexpected entries`. No Linux account,
Forgejo user, PAT, or staging state was compensation-deleted.

### Last passed boundary

The unprivileged coordinator passed the password only on stdin, Forgejo's PAM
source accepted the native Linux identity, Tea created its deterministic token,
and Tea wrote its private configuration beneath the actor-owned runtime tree.

### First failed boundary

The privileged helper expected `config/tea` to contain only `config.yml`. The
pinned Tea implementation uses a persistent `config.yml.lock` for its native
configuration locking and intentionally closes without unlinking that file.

### Direct evidence

The exact pinned Tea source shows `GetConfigPath() + ".lock"`, opens that file
with create/read-write mode `0600`, and releases it by unlocking and closing the
descriptor. It does not unlink the lock. The repository's Tea runner fixture
had modeled only `config.yml`, so focused tests had proved a substitute shape.

### Root cause and ownership

Soda owns the privilege boundary that validates and publishes staging, while
Tea owns its configuration representation and locking. The helper's strict
validation was correct in principle; the test fixture and allowlist were
incorrect because they did not represent the exact pinned upstream output.

### Smallest correction

Accept exactly `config.yml` and `config.yml.lock`. Validate both with no-follow,
beneath-only descriptor operations and actor ownership. Additionally require
the lock to be a regular, empty, mode-`0600` file. Continue copying only the
opaque `config.yml`; never parse, copy, or retain the lock in the human home.

### Rejected broader fixes

Do not weaken staging to accept arbitrary Tea files, recursively copy Tea's
configuration directory, remove Tea's lock after publication, patch upstream
locking semantics, parse the PAT, add a token store, or add recovery state.

### Verification completed

The focused race tests now model the real lock and reject extra entries,
nonempty locks, incorrect modes, symlinks, and ownership mismatches. Repository
`just check` passes, and matching-native x86-64 RPM, OCI, installer ISO, and
release-record construction pass at commit `1aafad3`.

### Verification still required

Repeat fresh installed Add Person, workspace Tea copying, full product capture,
and B-to-A-to-B preservation on x86-64. Matching-native AArch64 installation
and acceptance remain a sibling release requirement.

### Secret, data-loss, and destructive-action notes

The failed disposable run's evidence proved the installer password and raw
Tailscale key were absent from retained evidence and installed paths. Cleanup
removed only that run's exact VM and registry state and did not touch the
protected Tailscale key file.

### Rule we will reuse

When a strict boundary consumes upstream-owned filesystem output, tests must
model the exact pinned upstream shape, including native lock artifacts. A
successful mock that omits upstream side effects is not evidence that the
installed producer and privileged consumer agree.

## 2026-09-03: installer-created Tea login lost its Forgejo endpoint after enrollment

### Expected outcome

The installer administrator's Tea configuration should work after first boot,
and its one-time copy into a derived workspace should retain the same human's
native Forgejo identity.

### Exact artifact and environment

The failure occurred on matching-native x86-64 with candidate source
`11dd11a`, image digest `sha256:3ae8f501d37007ace03d2d9eb851f7e0ff3613b99026fd78b8a5876ccc335510`,
and ISO SHA-256 `c45ef73a9787e0a4b666dd67cd984b5cb311e44c8306bf7dc6c9a0456340f31d`.

### What happened

Installation, first-boot Tailnet enrollment, and primary SSH readiness passed.
The first post-install identity check failed because `tea api --login soda
user` tried `http://127.0.0.1:30000`, while the enrolled Forgejo service had
replaced its loopback bind with the Tailnet IPv4 address.

### Last passed boundary

The installer had successfully created and verified the administrator's Tea
login against the sealed transient loopback Forgejo process. The installed
machine then enrolled and became reachable through its Tailnet address.

### First failed boundary

The installed administrator could not use the installer-created, user-owned
Tea configuration after Forgejo restarted with its enrolled address.

### Direct evidence

The retained runner output reported a native connection refusal for
`GET http://127.0.0.1:30000/api/v1/user`. No credential material reached the
evidence directory.

### Root cause

The installer necessarily creates the Tea token before first-boot Tailscale
enrollment, so the only usable endpoint at that time is loopback. The runtime
Forgejo initializer later changed `HTTP_ADDR` from loopback to the single
Tailnet address. One persistent Tea configuration therefore could not span the
two accepted phases.

### Authoritative owner

Forgejo remains the sole HTTP and authentication owner. Linux nftables remains
the appliance ingress owner. Soda owns only their fixed private composition and
the installer-time Tea handoff.

### Smallest correction

After enrollment, keep Forgejo's one native listener available on loopback and
the Tailnet by binding IPv4 and retaining the advertised Tailnet `ROOT_URL`.
The existing nftables rules admit TCP 30000 only from `lo` and `tailscale0` and
reject every other ingress path. No proxy, second listener, configuration
rewriter, credential service, or first-boot workflow is added.

The same failed run also proved that the disposable registry writes its bind
mount as root by default, preventing the unprivileged exact-run cleanup. Run
that disposable container as the invoking UID/GID so its already-scoped
registry tree remains removable without privileged or broad cleanup.

### Rejected broader fixes

Do not rewrite Tea configuration after enrollment, store the password for
first boot, add a proxy, add a runtime bootstrap service, duplicate the token,
or teach Soda to reconcile Forgejo endpoints or credentials.

### Verification still required

Rebuild post-correction images A and B, then repeat fresh x86-64 installation,
Tea identity, Add Person, workspace copying, product acceptance, and B-to-A-to-B
preservation. Matching-native AArch64 remains independently required.

### Secret, data-loss, and destructive-action notes

The runner's retained secret scan passed. The protected Tailscale key was not
read into diagnostics or removed. Only the exact disposable failed-run tree is
eligible for later cleanup.

### Rule we will reuse

An installer-created native client configuration must name an endpoint that
still exists after first boot. When one upstream service can safely cover both
private phases behind the already-authoritative firewall, preserve that one
listener instead of adding credential rewriting or lifecycle machinery.

## 2026-09-03: account removal left failed user-manager units

### Expected outcome

Supported project and human deletion should terminate the selected Linux
accounts and leave the installed host with no failed systemd units.

### Exact artifact and environment

The failure occurred during matching-native x86-64 acceptance with candidate
source `e9b62eb`, image digest
`sha256:2c4b9b28ad5aedc35ef5c3604c8ac200c6491d9e2e73b32b9c02a879cd4bfe94`,
and ISO SHA-256
`22f87f7ec9f240ea3dfef2cbafb17255d2c7417ab2e677500b456485cce0069d`.

### What happened

Installation, native Tea operations, the B-to-A-to-B preservation comparisons,
workspace isolation, and the deletion scenarios completed. Final capture then
found `user@1008.service` and `user@1013.service` failed after their account
processes had been deliberately terminated.

### Root cause

The bounded deletion path terminated logind sessions and then sent UID-scoped
TERM and KILL signals. Killing the remaining per-user systemd manager correctly
stopped the account's processes, but systemd retained the resulting unit failure
after the account was deleted.

### Smallest correction

Remember whether the validated account had an active logind user, verify that
its processes are gone, and reset only that exact `user@<uid>.service` failure
before the final account revalidation and `userdel`. A reset failure leaves the
account and higher-level catalog state intact for a visible retry.

The first implementation assumed that an account which had been active would
still have a loaded failed unit at that point. A fresh installed run with source
`18c76bb`, image digest
`sha256:9e753159b1a6530557e4e7b4e6b42b7f0762afe1822a75283ebe48714c17ef91`,
and ISO SHA-256
`865cd5bc4804051f2ceca1249bcf8b475f97d51c637eb882c1e23e7e725fcb5d`
showed that systemd may instead unload the user manager before the reset. The
exact reset then returned `Unit user@1002.service not loaded.` even though no
failure state remained.

The bounded deletion path now inspects only the exact user manager's
`ActiveState`. An inactive unit needs no reset. A failed unit is reset. If the
unit becomes inactive between inspection and reset, that race is also accepted.
Any other state or a reset that leaves the unit failed remains a visible error,
with the account and catalog entry retained.

### Rejected broader fixes

Do not ignore failed units in acceptance, reset all systemd failures, weaken
process termination, add cleanup state, or add a reconciliation service.

### Verification completed

Focused race tests pass for the exact-unit reset, the already-inactive case,
the unload-during-reset race, unexpected state, and a native reset failure. The
systemd user-manager boundary is kept in its own focused source file, and the
complete repository source gate passes.

### Verification still required

Rebuild both post-correction images and repeat fresh installed x86-64
B-to-A-to-B acceptance. Matching-native AArch64 remains independently required.

### Secret, data-loss, and destructive-action notes

The failed run's secret scan passed. Its evidence was retained, and only the
exact disposable machine and registry state were removed.

### Rule we will reuse

If a supported destructive operation deliberately kills an upstream-managed
unit, remove only the failure state caused by that exact validated target before
the irreversible deletion. Never hide unrelated failed units globally.

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
