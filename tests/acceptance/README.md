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

### Installation and native onboarding

After separately authorized builds, run on both matching architectures:

1. Complete graphical Anaconda account creation, reboot, and verify normal login,
   home ownership, administrator privilege, and cloud-init-disabled ISO startup.
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

### Access

- On a trusted LAN, SSH, Cockpit, Forgejo, and a normal project development
  server are directly reachable.
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
  --tailscale-auth-key-file PATH \
  --administrator-private-key PATH \
  --administrator-public-key PATH \
  --administrator-password-file PATH
```

Use `aarch64` as the evidence-directory leaf on an AArch64 host. The protected
Tailscale file contains one reusable ephemeral guest key. The private key and
password are disposable test credentials; all three secret files must have
mode `0600` or stricter. The ISO uses only the installer. For the QCOW2 LAN fixture, the runner uses
cloud-localds to deliver native cloud-init user-data automatically; install
cloud-localds and openssl on the matching host. Protected fixture files stay in
the disposable work directory and are removed during cleanup. The operator
creates the ISO Linux administrator in Anaconda, logs in, configures the network,
and adds the personal key through Cockpit Accounts. The runner prints only the protected input
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
  --aarch64-release-record PATH \
  --expected-revision EXACT_MAIN_SHA \
  --output PATH \
  --approved-signer SIGSTORE_CERTIFICATE_IDENTITY \
  --oidc-issuer SIGSTORE_OIDC_ISSUER
```

The AArch64 release record is parsed with the strict schema-3 decoder and must
bind the AArch64 run's candidate digest, `linux/arm64` platform, and exact main
revision. Both summaries must name that same revision as their source and suite
revision. The maintained `Native acceptance evidence` workflow accepts only
base64-encoded copies of those three credential-free records, invokes this
command with its exact `main` SHA, signs and immediately verifies the combined
record, and retains only the five small record files for one day. It does not
run QEMU, receive a guest Tailscale credential, publish an image, or create a
release.

The runner QCOW2 fixture covers cloud-init with an ordinary trusted disposable
LAN. The late-enrollment Tailnet, native Cockpit key UI, first-signup, registration
policy, and reboot matrix above still requires separately recorded installed
acceptance; a runner summary alone does not prove those interactive checks.

Run `sudo tests/acceptance/check-native-service-ordering.sh` on each installed
candidate after provisioning, and repeat after reboot and on the cloud-init-disabled
ISO. It inspects the actual Fedora and Forgejo units and boot journal.
The runner generates a separate protected Forgejo owner password and pauses for
native first-owner signup before creating teammate fixtures. It verifies the
owner role and that the Linux password does not authenticate that local account.

The native welcome and separate Cockpit Tailscale page require the installed
checks in [native installation acceptance](../../docs/native-onboarding.md#installed-acceptance).
Browser-authentication and exit-node evidence must exercise the real native
flow; auth-key fixtures and command-unit tests do not substitute for it.
