# Backups and restoration

Protect mutable account, repository, workspace, and application data, and prove you can restore it before relying on a backup.

An immutable OS image is replaceable; your current data is not. Git protects
only commits that reached another copy. Image fallback preserves current data
rather than recovering deleted files. Soda uses native filesystem, VM/provider,
and application backup tools—there is no separate Soda backup service.

## Decide what must survive

| Scope | Include and preserve |
| --- | --- |
| Linux identity/configuration | Current `/etc`, passwords/groups, UID/GID relationships, SSH host keys, authorized keys, service/firewall configuration, custom mounts |
| Homes/workspaces | Actual homes recorded by Linux, full clones, unpushed commits, untracked files, workspace Git keys, private tool state, project-local data |
| Shared project catalog | `/var/lib/soda/projects`, together with the Linux accounts/homes it describes |
| Forgejo | `/etc/forgejo` and `/var/lib/forgejo`, including database, repositories, LFS, attachments, application secrets, and SSH integration |
| Tailnet identity | Native `/var/lib/tailscale` state if restoring the same machine identity; never activate two copies concurrently |
| Runners | Needed `/var/lib/soda/runners` state and actual runner homes/work directories, or a documented provider re-registration plan |
| Project applications | Databases, container volumes, user/system service configuration, and data paths outside homes |
| Recovery information | Verified release record/digest, architecture, storage layout/mounts, installed application versions, backup time, encryption-key access |

Use `getent passwd` and native storage inspection to identify real homes and
mounted data. Do not assume that copying `/home` follows symlinked or mounted
storage, or that every application writes there. Preserve numeric owners,
permissions, symlinks, extended attributes, ACLs, and the SELinux context required
by your restore tool. A filesystem archive without identity relationships can
restore files under the wrong users.

These backups contain passwords/hashes, private keys, and tokens. Encrypt them,
restrict access, keep a copy outside the Soda machine, and protect recovery keys
separately. Choose retention and frequency against the work you can afford to lose.

## Take a consistent backup

A straightforward whole-machine route is a **powered-off VM/disk snapshot**:

1. Notify users and let jobs finish. Have developers preserve work, and stop
   project writers and local runners through their native controls.
2. Shut the machine down normally. Confirm it is stopped before snapshotting.
3. Use the VM/provider or disk tool to capture all boot and data volumes as one
   consistent set, not just the boot volume. Copy/export the set to independent
   protected storage according to that tool's native procedure.
4. Record the backup time, machine architecture, included volumes, and exact
   release. Restart the original only after capture is complete.

A snapshot on the same disk/account is not independent protection from that
storage's loss. Check the provider's actual consistency and export guarantees;
calling a running-machine snapshot a backup does not make application data consistent.

If using **file/application backups** instead, coordinate a native maintenance
window and use each application's documented backup method. For Forgejo, follow
[its backup and restore guide](https://forgejo.org/docs/latest/admin/backup-restore/)
for the bundled version and configuration above. Quiesce writes and preserve its
database and repositories as one consistent set; do not copy a live database
file and assume consistency. Back up project databases with their own native
procedures, including credentials/configuration required for restoration.

The file-backup plan must include Linux identities and all data paths, not just
Forgejo's dump. Record what is intentionally reconstructed, such as disposable
runner clients or mise caches, and keep native restoration instructions with the
backup. Never put recovery secrets in the shared project catalog.

## Restore a whole machine

1. Select a known-good complete backup and verify its integrity with the tool
   that created it. Confirm the target architecture and storage capacity.
2. Keep the original off, or keep the restore isolated from both the production
   LAN and Tailnet. Duplicate SSH, Tailnet, Forgejo, and runner identities must not
   become active simultaneously.
3. Restore **all volumes in the recorded set** through the native VM/disk tool;
   attach them with the recorded boot/mount layout. This is a data restore to the
   backup time, including the accounts/passwords that existed then.
4. Boot using a console and test the identity, services, and files below before
   allowing users or CI jobs to write. Keep public cloud service ingress closed.
5. Retire the original machine's conflicting identity before bringing the restored
   server onto the normal route. Re-enroll or re-register services only through
   their native procedures when the recovery plan intentionally replaces identity.

For a file-level rebuild, first install the matching verified Soda release in an
isolated machine, then restore native identity/configuration and dependent data
with the recorded application procedures. Restore owners, modes, mounts, and
required SELinux labeling before starting writers. Do not overwrite selected
password/account files on a running production host or mix a database from one
backup time with repositories from another.

## Test before accepting recovery

Use an isolated, matching-architecture restore and verify:

- expected booted image, disks/mounts, account UID/GID ownership, passwords,
  administrator access, and primary/workspace SSH;
- the catalog and existing workspace clones, including an unpushed test commit
  and untracked test file from the backup;
- Forgejo sign-in, repository browsing/clone, keys, and required LFS/attachments;
- project databases and private data with application-specific checks;
- intended firewall/Tailnet access and no duplicate identities;
- runners only after deliberate registration/identity review, before accepting jobs.

Record the tested backup, steps, results, and restore time. A successful archive
command is backup evidence, not recovery evidence. Repeat after changes to storage,
applications, credentials, or the backup method.
