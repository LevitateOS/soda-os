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

## Required scenarios

### Installation and Soda Setup

- One architecture-matched network ISO boots stock graphical Anaconda without
  human OEMDRV or other provisioning media.
- The installed system presents Soda Setup.
- A reusable QCOW2 presents the same Soda Setup state through its console.
- The same bounded Soda Setup operations are available through Cockpit, and
  the setup can always be reopened.
- Dismissal remains unavailable until the Linux/Forgejo administrator,
  password, SSH public key, and either Tailscale or explicit trust of the
  current connection through **Allow access from the local network** exist.
- A supplied Tailscale key is used once and removed.
- No NoCloud, ConfigDrive, cloud-input, public-SSH bootstrap, or alternate
  onboarding path exists.

### Access

- On a trusted LAN, SSH, Cockpit, Forgejo, and a normal project development
  server are directly reachable.
- In a cloud topology, SSH, Cockpit, and Forgejo are reachable through
  Tailscale and rejected from public ingress.
- Tailscale does not block LAN access.
- A normal development-server link works for a teammate over LAN or Tailscale,
  including hot reload, without Soda port or process tracking.

### Identity and Git

- Linux owns one primary account per person; `wheel` alone owns administrator
  status.
- Development occurs only in derived workspace accounts.
- Each supported person receives a matching Forgejo account and registered SSH
  public key.
- Git uses SSH.
- Workspace accounts never become Forgejo users.
- Workspace creation copies only current public authorized keys once.
- Tea and gh are available and require manual, separate authentication in each
  workspace.
- No Tea token/configuration, gh configuration, private key, or credential is
  copied or retained by Soda.

### Projects, tools, and deletion

- Everyone can view and edit the shared project list without a closed metadata
  field contract or membership model.
- Each person-project pair receives an independent UID, home, full clone,
  dependencies, processes, and mutable state.
- A person can remove only their own workspace.
- Only an administrator can remove a whole project; it deletes the shared Soda
  entry and every local workspace, including uncommitted work, while preserving
  the canonical Forgejo repository.
- Person deletion removes workspaces, the Forgejo account, and the primary
  Linux account in that order.
- Injected failures expose exactly what succeeded and remains; an explicit
  retry continues without rollback or hidden workflow state.
- `mise` provides workspace and shared-project tool scopes, shared upstream
  caches, and isolated installed dependencies on Fedora 44 with enforcing
  SELinux.
- Multiple tool choices and optional workspace-specific coding assistants work
  without a closed list or copied credentials.

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
  --tailscale-auth-key-file PATH \
  --administrator-private-key PATH \
  --administrator-public-key PATH \
  --administrator-password-file PATH
```

Use `aarch64` as the evidence-directory leaf on an AArch64 host. The protected
Tailscale file contains one ephemeral, one-use guest key. The private key and
password are disposable test credentials; all three secret files must have
mode `0600` or stricter. The runner creates no onboarding media. It opens the
architecture-native QEMU graphical display for the operator to complete stock
Anaconda and Soda Setup. The runner prints only the protected input
paths, then resumes through native SSH and Tailscale readiness.

The successful run leaves `summary.json` and normalized credential-free
evidence, but removes its exact QEMU processes, disposable loopback registry,
generated keys, passwords, and VM disks. A failed run retains sanitized
diagnostics and reports cleanup failures explicitly.

After matching x86-64 and AArch64 summaries name the same source and suite
revisions, combine and sign them:

```text
go run ./cmd/soda-acceptance record \
  --x86-summary PATH --aarch64-summary PATH \
  --output PATH \
  --approved-signer SIGSTORE_CERTIFICATE_IDENTITY \
  --oidc-issuer SIGSTORE_OIDC_ISSUER
```

This invokes Cosign for signing and immediate verification. Release CI will
consume the signed record as a prerequisite when issue #48 connects that
transport boundary; it does not run QEMU or receive a guest Tailscale
credential.
