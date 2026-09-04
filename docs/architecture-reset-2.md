# Soda postmortem decision ledger

This document records decisions Vince explicitly confirmed during the postmortem. It is not an implementation plan and does not authorize implementation.

## Review method

- Update this ledger immediately after every confirmed decision or correction.
- Record rejected assumptions as rejected; do not silently remove the history of the correction.
- Distinguish decided product requirements from engineering facts that still need verification.
- Do not move to the next requirements section until the current decision has been recorded here.

## Installation

- The current release journey is: download one completed Soda ISO, boot it,
  complete graphical Anaconda for the installation responsibilities that remain
  there, complete **Soda Setup** after the installed system boots, and use Soda.
- Soda Setup is a temporary workaround, not the intended permanent
  architecture. The long-term goal is one complete installation journey with
  no separate Soda-owned post-install setup.
- This long-term direction does not authorize deleting the current Soda Setup
  instructions before a replacement exists, pretending the workaround is
  already gone, moving every Soda Setup screen into a renamed custom installer,
  or inventing replacement behavior.
- The user must not clone the Soda repository.
- The user must not run internal Soda developer tooling.
- The user must not create or attach a second credential image.
- Anaconda remains graphical, but Soda onboarding must not depend on fragile Anaconda customization.
- For the current release, every onboarding responsibility that can safely be
  moved out of Anaconda is completed through Soda Setup, shared by ISO and
  QCOW2 installations.
- Soda Setup asks the user for the Tailscale authentication key after installation.
- The key is treated as a secret, used once to enroll the machine, and removed after the enrollment attempt.
- Tailscale enrollment does not justify pre-install secret transport, OEMDRV media, repository tooling, or Anaconda customization.
- Soda Setup appears at startup until the user explicitly chooses **Do not show on startup again**.
- Soda Setup remains easy to reopen after it has been dismissed.
- The same onboarding capabilities are available through Cockpit after Cockpit becomes reachable.
- Cockpit is an additional access path, not the only path, because the machine may need initial account and network setup before Cockpit can be used.
- Console and Cockpit presentation must use the same underlying bounded onboarding operations rather than becoming two independent setup products.
- **Do not show on startup again** applies to the entire machine.
- The dismissal control remains unavailable until setup verifies that every required onboarding item is complete.
- Dismissal must be based on the actual required system state, not merely on the user having viewed the screens or checked a box.
- Soda Setup is complete only when the administrator account exists, its
  password is set, its SSH public key is installed, Forgejo is ready for that
  administrator, and the user has either successfully connected Tailscale or
  explicitly selected **Allow access from the local network** for the current
  connection.
- Tailscale enrollment is not required for a machine intended to be used only over its local LAN.
- A machine using a trusted local connection may complete and dismiss Soda
  Setup without Tailscale.
- Tailscale setup remains available later by reopening Soda Setup or through Cockpit.
- Automatic detection of local-versus-cloud use, and any Anaconda integration for that choice, are deferred future work and are not current requirements.
- A future Anaconda experience may allow the installer to enable or disable Tailscale and Forgejo, but that is explicitly deferred and is not part of the current implementation requirement.
- Anaconda retains only the installation responsibilities that cannot safely be moved, such as storage and deployment; the exact boundary must be verified against the shipped Fedora version.
- The current installation plus Soda Setup journey must produce a working Linux
  administrator, SSH access, Forgejo administrator, and Tea capability. It must
  also produce Tailscale enrollment when the user chooses Tailscale; a machine
  using an explicitly trusted local connection may complete Soda Setup without
  it.
- Secrets must not be embedded in published artifacts or retained Soda state.
- Requiring the repository and internal tooling was a release-blocking product failure, not a minor documentation or UX problem.

## Installer ownership

- Keep stock Fedora Anaconda.
- Do not replace Anaconda's storage, networking, bootloader, or bootc installation responsibilities.
- The installer may be network-only.
- It installs an exact Soda OCI image from GHCR.
- Offline installation is not currently required.
- Any Soda-owned integration must exist only to complete the approved user journey.
- If an engineering constraint would change the journey, trust boundary, privileges, persistence, or failure behavior, return to Vince for a decision.

## Cloud image

- Publish a reusable compressed QCOW2 for x86-64 and AArch64.
- Users must not clone the Soda repository or run internal developer tooling to configure the QCOW2.
- The QCOW2 uses the same Soda Setup experience as an ISO-installed machine.
- The current repository-only `soda-image cloud-input` experience is rejected and removed as an end-user product path.
- NoCloud and ConfigDrive provisioning are not product paths.
- Soda does not merge complete or partial cloud-provided setup data with interactive setup.
- There is one current onboarding path, Soda Setup, not separate ISO, NoCloud,
  ConfigDrive, full-data, or partial-data variants.
- Soda supports cloud environments only when the provider offers a usable VM console for Soda Setup.
- Cloud environments without console access are unsupported.
- Soda does not temporarily expose public SSH as an onboarding workaround.
- If the QCOW2 boots without cloud-provided setup data, it must offer Soda Setup instead of leaving an unusable unprovisioned machine.
- The earlier no-datasource behavior that deliberately performed no provisioning and provided no usable login is rejected.
- The QCOW2 and ISO use the same Soda Setup state and operations; Soda must not create separate onboarding products for the two artifact types.

## Runtime product and access

- Soda is cloud-first, not cloud-only.
- In cloud deployments, SSH, Cockpit, and Forgejo are available through Tailscale.
- On a local network, SSH, Cockpit, and Forgejo are directly available over the LAN.
- Tailscale must not disable or replace ordinary LAN access.
- SSH, Cockpit, and Forgejo must not be exposed directly to the public Internet.
- Lightweight client computers connect to a more powerful Soda machine.
- Development remains SSH-first.
- Soda composes Fedora and established upstream tools rather than replacing their native responsibilities.

## Trusted-team model

- Soda is operated by a team whose members trust one another and communicate about shared work.
- All team members are trusted; trust is not limited to administrators.
- Administrator status is a capability boundary for system-wide actions, not a hostile-user threat model for ordinary team members.
- Soda must not assume that a team member intends to sabotage the team or company.
- Soda must not encode coordination, approval, ownership-transfer, archival, preservation, or recovery policy merely because such policy could exist.
- Team decisions remain with the team; Soda must not make those decisions on its behalf.
- Soda-owned policy requires an explicit product requirement, a necessary security or privilege boundary, clear destructive confirmation, or an upstream technical requirement.
- Existing Soda policy must be audited against this trusted-team model; do not remove or retain a policy without identifying its actual authority and purpose.

## Human identities

- Each person has one primary Linux identity.
- Linux owns usernames, passwords, user IDs, groups, home directories, and administrator status.
- Membership in `wheel` means administrator.
- Soda has no separate person database or copied administrator-role system.
- Additional people are not administrators by default.
- Later Linux users are created through stock Cockpit Accounts. Soda must remove
  the later-user **Add Person** flow and must not replace it with another
  onboarding system.
- Administrators can promote people through Cockpit.
- Stock Cockpit provides the user list and the control for promoting a Linux user to administrator; this changes that user's `wheel` membership.
- Promoting a Linux user does not make them a Forgejo site administrator.
- Soda must not build its own user-list or administrator-promotion machinery; stock Cockpit owns that experience.
- Actual development happens only inside project workspaces; the primary account is not a separate development space.

## Forgejo, Git, and Tea

- Soda Setup creates the first Linux administrator and bootstraps the
  corresponding first Forgejo administrator.
- A later Linux user performs one normal first Forgejo login with the same
  Linux username and password. PAM authenticates the Linux credentials, and
  Forgejo automatically creates its own internal profile.
- A later user has no separate Forgejo signup or Forgejo password.
- Soda must not pre-provision a later user's Forgejo profile or personal SSH
  key. After the first login, the user manages their Forgejo public keys through
  Forgejo's native account interface.
- Repositories are created through Forgejo's or GitHub's own interface. Those
  products own repository creation, repository lifecycle, authentication, and
  their native policies.
- After creating a repository, a user adds it to Soda using its SSH clone
  address.
- Soda must remove its **Create Forgejo Project** interface, project-creation
  password handling, repository-creation action and protocol, and Forgejo
  repository-creation API path.
- Soda owns only the shared project catalog and workspace orchestration. It
  must not replace the removed repository-creation path with another Git-host
  abstraction, API, policy, or credential flow.
- Git operations use SSH.
- Remove all Tea-token and Tea-configuration copying into workspaces.
- Do not share one Tea credential across workspaces.
- Each workspace requires its own manual Tea login when Tea is needed there.
- Tea and GitHub CLI (`gh`) are installed automatically in every workspace; they are not optional coding-assistant selections.
- GitHub CLI authentication is manual and separate in every workspace; Soda does not copy or share its login state.
- Soda must not create a credential broker to avoid those logins.
- The separate-login choice favors implementation simplicity and workspace isolation over login convenience.

## Projects and workspaces

- Cockpit has one focused Soda Projects page.
- A user adds an existing Forgejo or GitHub repository to the shared project
  catalog using its SSH clone address. Repository creation is not a Soda
  Projects action.
- The earlier closed project-record rule that allowed only an immutable ID, display name, and Git address was unapproved fine print and is rejected.
- A project has the information required to support the approved product experience, including its human-facing identity and Git repository address, but no closed field list has been approved.
- Project tool definitions live in mise's normal repository configuration and are not duplicated into Soda's project catalog.
- Do not add project fields merely because they might be useful. Any additional product-visible project data requires evidence from an approved experience or an explicit decision.
- Every person can see and edit the shared project list.
- A person can choose **Set up for me** to create their own workspace for a project.
- Each person-project pair has a separate Linux account, home directory, full Git clone, processes, installed dependencies, and private mutable caches. Any tool downloads or caches remain owned by mise or the relevant upstream tool, not by Soda.
- Workspaces use ordinary SSH.
- When a person's workspace is created, Soda copies that person's current public SSH keys into the workspace's `authorized_keys` file.
- Only public keys are copied. Soda never copies or handles the person's private SSH key.
- This is a one-time workspace-creation step and does not require a credential broker, key service, or synchronization system.
- Privileged Soda code must not receive Git credentials.
- When one person runs a development server in a workspace, another Soda user must be able to open it easily, including live-development behavior such as React hot reload.
- The workspace owner must not have to click **Share** first.
- Soda does not need to track, discover, or list running ports or development-server processes.
- The previous statement requiring automatic development-server discovery was a Codex-added assumption and is rejected.
- The intended minimal behavior is ordinary network reachability: the development server can listen on an address reachable through the local LAN or, for cloud installations, through Tailscale. One user can give another user the normal server address and port, and browser features such as hot reload must continue to work.
- The user experience is simply: one user tells a colleague, "open this link," and that link works.
- Each user must be able to remove only their own workspace for one project.

## Development tools

- Soda OS ships mise in workspaces.
- Developers invoke mise directly. Mise owns its interface, commands, configuration, tool installation, version selection, updates, removal, and lifecycle.
- Project tool definitions live in mise's normal repository configuration and travel with the repository. Soda does not copy those definitions into its project catalog or write mise configuration.
- Soda provides no project-creation tool picker, workspace-versus-project scope picker, tool-install action or protocol, tool status view, or coding-assistant installer.
- Soda owns no shared tool-installation directory, shared tool lifecycle, tool cache, downloader, package manager, version manager, profile system, or durable tool state.
- Project removal does not perform Soda-specific cleanup of mise installations or caches. Ordinary workspace deletion removes the workspace's local files; repository-hosted tool definitions remain with the repository.
- Mise and the relevant language package managers own their normal files and behavior. Mise does not replace upstream project dependency managers such as Cargo, Go modules, npm, or Python package tooling.
- Mise does not replace Soda's project catalog, Linux workspace lifecycle, SSH setup, network access, or user authentication.
- Tea and GitHub CLI remain available in every workspace as a separate confirmed product requirement. Their authentication remains manual and workspace-specific, and Soda does not copy credentials between accounts or workspaces.
- No Soda wrapper, replacement package manager, policy layer, durable state, or status protocol may be added around mise.

## Updates and fallback

- Automatic Soda OS updates are disabled.
- An administrator chooses when to update.
- Updates use Fedora's native `bootc` mechanism; Soda does not build its own updater.
- An administrator can deliberately select an earlier exact Soda image.
- Fallback must preserve users, passwords, administrator roles, home directories, Forgejo data, projects, workspaces, Tailscale identity, and SSH access.
- Fallback is not a backup and does not restore data the user already deleted.
- Soda does not create an update database, background update service, or custom recovery engine.

## Release behavior and test cost

- A push to the protected `production` branch is intended to build, verify, sign, and publish one coordinated x86-64 and AArch64 release.
- CI time has a direct monetary cost.
- Release testing must be efficient and effective, not extensive for its own sake.
- The production gate must run the smallest set of tests that provides meaningful evidence and blocks a bad release.
- Scenarios must not be repeated on every release merely because they exist; broader testing belongs in a focused run when the relevant behavior changes.
- Current implementation fact: the acceptance flow runs a full B to A to B image round trip for fallback verification.
- Whether that fallback round trip remains in every production release is under review.
- Cost investigation found that the current release executor builds both the new release and the older fallback release from source on both architectures.
- The older fallback build unnecessarily produces an ISO and QCOW2 even though fallback testing consumes only its OCI image and release record.
- The current executor runs the full source-check suite once for the old release and again for the new release on each architecture.
- The current acceptance run boots four virtual machines per architecture: one ISO-installed machine, one NoCloud QCOW2, one ConfigDrive QCOW2, and one QCOW2 without input data.
- The ISO-installed machine is additionally rebooted twice for the B to A to B fallback round trip.
- Broad product behavior is repeated across architectures even when it is not architecture-specific.
- The workflow reconnects to Tailscale in separate prepare, promote, and upload jobs for each architecture.
- No production release workflow has run yet, so exact billed minutes are unavailable. These are structural duplication findings, not measured timing claims.
- A cost-bounded test matrix must be designed from unique failure coverage and measured runtime; removing only whichever individual test Vince mentions is explicitly insufficient.
- A release artifact is built exactly once, tested as exact bytes and digest, and then promoted and published unchanged. Do not build one copy for testing and another copy for release.
- Current implementation fact: the workflow already follows this build-once rule for the new candidate B.
- For B to A to B fallback testing, A must be the previously published signed Soda OCI downloaded from GHCR, not rebuilt from source.
- Confirmed correction: expensive installation and product testing happens beforehand on user-controlled machines, outside paid release CI.
- Release CI must not rerun the complete acceptance suite after that qualification.
- Correction to the previous interpretation: release CI may build the release artifacts once after user-side qualification; it does not have to publish the files built during user-side testing.
- User-side qualification produces a machine-readable test record, such as JSON, identifying the exact source commit, architectures, tests performed, and successful result.
- That test record must be authenticated, for example with a digital signature, so CI can reject an invented or modified record.
- Release CI verifies the authenticated test record, builds each release artifact once, runs only cheap checks on those exact release artifacts, and publishes those same artifacts unchanged.
- Deep user-side testing certifies the source commit and product behavior. Cheap CI checks certify the identity and basic integrity of the exact release artifacts.
- The record format, signer identity, signing mechanism, expiry or freshness rule, and exact cheap CI checks are engineering design work that must be reviewed; they are not permission to invent a broader attestation service.

## Deletion semantics

- Current implementation fact: project-wide removal authorizes any primary user, not only administrators.
- Current implementation fact: there is no personal **Remove my workspace** action.
- Confirmed correction: each user must be able to remove only their own workspace for one project.
- Confirmed correction: only an administrator may remove an entire project from Soda.
- Removing an entire project permanently deletes every local workspace for that project, including uncommitted work, and removes the project from the shared list.
- Project removal does not delete the canonical Forgejo repository. The Forgejo repository stays.
- The trusted team coordinates and preserves anything it wants to keep before the administrator confirms removal.
- Soda must not add approval, transfer, archival, preservation, or recovery policy around that team decision.
- Administrators are trusted actors. Soda must not design project removal around an imagined administrator intentionally sabotaging the team or company.
- Confirmed correction: removing a person through Soda must also remove that person's Forgejo account so the removed person cannot continue logging into Forgejo.
- The earlier requirement that person removal leave the Forgejo account unchanged is rejected.
- Soda must not invent a repository-transfer, archival, or preservation policy for person removal.
- The team is trusted to coordinate before removing a person and to preserve or move anything it wants to keep.
- Person removal follows the native Forgejo account-removal boundary after the team confirms the destructive action. Engineering must report Forgejo's exact native repository behavior rather than adding Soda policy.
- Confirmed removal order: delete the person's workspaces first, delete their Forgejo account second, and delete their primary Linux account last.
- If any removal step fails, Soda stops, shows exactly which steps succeeded and which did not, and lets an administrator retry the remaining cleanup.
- Soda must not attempt to roll back already completed deletion steps or hide a partial result.

## Decision-discipline rule

- Codex proactivity is dangerous when it converts a requested outcome into unapproved product behavior or machinery.
- Do not infer additional features, systems, tracking, automation, or UX from a broad outcome.
- Proactivity may investigate evidence and surface choices; it may not make product choices on Vince's behalf.
- Separate user requirements, evidence, engineering details, unknowns, and decisions that belong to Vince.
- Explanation may be concise, but product decisions must never be compressed away.
- Do not silently fill an unknown.
- If implementation reveals a new product, trust, security, destructive, operational, or architectural choice, stop and return it to Vince.
- An engineering verification is not an undecided product requirement.

## Human ownership and quality

- Vince's product decisions override conflicting plans, issues, and implementation assumptions.
- Issues track work; they do not define the product.
- Code must remain understandable enough for Vince to own, modify, and delete.
- Remove obsolete machinery instead of renaming or relocating it.
- Do not create abstractions merely to satisfy code-quality metrics.
- Preserve structural and complexity checks.
- Make small, logical commits and preserve unrelated work.
- Test the real installed product, not substitutes.
- Test architecture-specific behavior on matching hardware.
- Do not push, publish, deploy, or create a release without Vince's explicit permission.
- Continue through ordinary engineering failures, but not through authority, safety, or uncontrolled-cost boundaries.

## Forbidden architecture

Soda must not build:

- A general background daemon or runtime API.
- A separate person, administrator, or permissions database.
- A repository mirror or project-membership system.
- A repository-creation UI, protocol, Git-host API wrapper, credential flow, or
  policy layer.
- A credential broker or shared-token system.
- A custom SSH gateway.
- A container controller used as the workspace model.
- A Soda-owned dependency cache, cache service, or downloader.
- A custom operating-system updater.
- A workflow engine, job database, retry queue, compensation system, or reconciliation loop.
- General replacements for Linux, Git, Forgejo, Tailscale, Cockpit, bootc, Podman, Anaconda, or language package managers.

Soda may retain narrowly bounded product integration for:

- The shared Projects catalog and Cockpit page.
- Creating and removing Linux workspace accounts.
- Registering only the initial administrator's SSH key during Soda Setup.
- One-shot privileged operations that perform only those approved actions.

Soda ships mise, but developers use mise directly. Repository-native mise
configuration and mise's own tool lifecycle are not Soda integration surfaces.

Soda may compose the approved product, but it must not turn those tasks into a general management platform.

## Mise replacement audit

- Confirmed product decision: Soda OS ships mise, and mise owns its interface, commands, configuration, development-tool installation, version selection, and complete tool lifecycle. Developers use it directly through normal repository configuration.
- Current implementation fact: the earlier Soda runtime toolchain manager, profiles, `/opt/soda/toolchains` mount, and related state service are already absent. They must stay absent; there is no existing manager to rename or migrate.
- Remove Soda's custom Bun distribution pipeline: the `soda-bun` RPM, its source lock and fetcher, its image-builder code and tests, and its RPM and OCI build dependencies. Bun becomes a mise-managed development tool.
- Replace the immutable image-wide development-tool contract that currently requires Go, Python and uv, Rust, Node and npm, Bun, compilers, build tools, and other development commands on every Soda machine.
- Narrow the image package locks and command manifest to genuine operating-system and product dependencies. Do not remove a package merely because it also appears in the old tool list; exact package retention must follow actual OS and product use.
- Remove Soda's tool picker, tool-install actions and protocol, mise configuration writer, shared-install storage and lifecycle, status reporting, and project-deletion cleanup for that storage.
- Rewrite the image, acceptance, documentation, and release assertions that promise or verify the old broad built-in toolset. Their replacement verifies only that mise is shipped and the rejected Soda-owned tool machinery is absent.
- Tea and GitHub CLI remain automatically available in every workspace because that is a separate confirmed product requirement; mise ownership does not silently remove them.
- Do not replace the removed machinery with a Soda wrapper, package manager, durable state, policy layer, cache owner, or lifecycle service.
- This audit identifies required removal scope only. No Soda repository files have been changed during the postmortem.

## Open consistency audit

These findings are recorded for explicit review. They are not silently resolved decisions.

- **Resolved Tailscale contradiction:** Soda Setup requires Tailscale enrollment
  only when the user chooses Tailscale; a machine may instead trust its current
  local connection with **Allow access from the local network**.
- **Resolved cache contradiction:** Soda owns no shared tool-installation storage, cache, downloader, or lifecycle. Mise and upstream tools own their normal files and caches.
- **Resolved project deletion gap:** administrator-only project removal permanently deletes every local workspace and uncommitted work for that project; the trusted team coordinates beforehand.
- **Resolved partial cloud input gap:** cloud-provided onboarding is not a
  product path; the current ISO and QCOW2 journeys both use Soda Setup. Soda
  Setup remains a temporary workaround on the path to one complete installation
  journey without separate Soda-owned post-install setup.
- **Resolved setup completion gap:** the current Soda Setup checklist is the
  administrator account, password, SSH key, Forgejo readiness, and either a
  successful Tailscale connection or explicit trust of the current local
  connection.
- **Resolved workspace SSH gap:** workspace creation copies the person's current public SSH keys into the new workspace's `authorized_keys`; no private key, broker, or synchronization system is involved.
- **Resolved later-user onboarding gap:** stock Cockpit Accounts creates the
  Linux user; the user's first normal Forgejo login authenticates through PAM
  and lets Forgejo create its internal profile. Soda has no later-user **Add
  Person** flow and does not pre-provision the Forgejo profile or personal key.
- **Resolved workspace-key boundary:** Projects accepts no Forgejo password and
  registers no workspace key. It reports the workspace's outbound public key
  after failed native SSH authentication; the person registers that key through
  the authoritative Git host and retries setup.
- **Resolved repository-creation boundary:** users create repositories through
  Forgejo or GitHub, then add their SSH clone addresses to Soda. Soda retains
  only the shared catalog and workspace orchestration and removes its Forgejo
  repository-creation UI, password handling, protocol, and API path.
- **Resolved Tea availability gap:** Tea and GitHub CLI are installed automatically in every workspace; Tea authentication remains manual and workspace-specific.
- **Resolved tool-installation boundary:** Soda ships mise, but developers use mise directly through its native commands and repository configuration. Soda retains no tool picker, install protocol, configuration writer, shared-install lifecycle, status reporting, or cleanup policy.
- **Resolved destructive person-removal gap:** workspaces are removed first, the Forgejo account second, and the Linux account last; a failure stops the operation and exposes the partial result for administrator retry, without rollback.
- **Resolved release-evidence gap:** CI verifies one signed strict test record for the exact source commit and both architectures; runs source and unit checks once; builds each architecture once; performs only structural OCI, ISO, QCOW2, checksum, identity, signature, and remote-publication checks; and publishes the exact checked outputs unchanged. Deep boot, install, provisioning, product, update, and fallback tests remain outside paid release CI.
- The test record identifies the exact source commit, acceptance-suite revision, architectures, required scenarios and results, fallback image digest, completion time, and approved signer. It is an authenticated claim, not a false claim that CI-built bytes were boot-tested.
- The current release-CI behavior that rebuilds the previous release, repeats `just check` across builds and architectures, creates acceptance-only Tailscale keys, and runs the unattended VM suite must be removed.
- Reuse the existing strict release record, checksums, immutable OCI digest, Cosign bundle, remote asset verification, anonymous image retrieval, and production-branch identity checks. Do not build a Soda attestation service.
- Confirmed CI performance boundary: target 30 minutes wall-clock and run architecture builds in parallel. At 45 minutes, emit a warning identifying the active or slow stage, but keep the release running. There is no hard stop or timeout based on this target.

## Postmortem facts established so far

- A planning-only architecture-reset task drifted into implementation and release preparation.
- A mechanism intended for unattended testing became a mandatory human installation step.
- The public handbook described the mandatory second-image design.
- The installer and local QCOW2 paths depended on internal repository tooling.
- The initial answer-media commit added 1,085 lines, with at least eight related follow-up commits.
- No official GitHub Release or public version tag currently exists.
- Production was not pushed.
- No implementation work was started during this postmortem.

## Review status

Every heading from the earlier requirements list has now been discussed.

Confirmed sections:

- Installation
- Installer ownership
- Cloud installation
- Runtime product and access
- Human identities
- Forgejo, Git, and Tea
- Projects and workspaces
- Development tools
- Updates and fallback
- Human ownership and quality

Still not fully confirmed:

- Deletion semantics are confirmed, including personal workspace removal, administrator-only whole-project removal, retention of the canonical Forgejo repository, and ordered person deletion.
- Release behavior: build once and publish the exact tested artifacts is confirmed, but the final cost-bounded test matrix is not approved.
- Forbidden architecture: the mise boundary is confirmed; no Soda wrapper, package manager, shared tool store, durable tool state, status protocol, or cleanup policy may replace mise's native ownership. Any unresolved audit of other forbidden subsystems must not reopen that boundary.
