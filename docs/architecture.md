# Implementation architecture

This is a map of the current source, not a second product specification.
[The product contract](product-contract.md) owns release-day behavior;
[acceptance](../tests/acceptance/README.md) distinguishes implemented checks
from qualification. Public documentation must not inherit internal readiness
caveats or obsolete mechanisms.

## Source owners

| Owner | Start here |
| --- | --- |
| Command construction and CLI boundary | `cmd/`; follow its local `AGENTS.md` |
| Catalog and user-facing Projects coordinator | `internal/projects`, `internal/projects/catalog` |
| Workspace creation, keys, clone, inspection | `internal/projects/workspace` |
| Primary-person selection and deletion ordering | `internal/projects/people` |
| Native account lookup, descriptor-safe homes, process termination | `internal/linuxhost` |
| Local CI accounts, provider clients, systemd listeners | `internal/runners` |
| Verified published release selection and bootc operations | `internal/updates` |
| Tailscale identity projection | `internal/tailnet` |
| Browser pages and feature stores | `cockpit/src`; see [frontend development](cockpit-development.md) |
| Native execution and file locks | `internal/process`, `internal/filelock` |
| Image, installer, release production | `internal/build/{image,installer,release}` |
| Identity, selected platform inputs, locks | `distro/`, `internal/config` |
| Shipped files | `packaging/`, grouped by owning RPM or artifact |
| Installed product verification | `internal/acceptance`, `tests/acceptance` |

## Projects boundary

Cockpit uses `/usr/libexec/soda/soda-projects` for fixed JSON operations.
`internal/projects/protocol.go` describes that interface; catalog entry/edit
values are shared with the catalog owner rather than translated into another
record. The coordinator derives caller identity from Linux and uses the narrow
workspace helper for accepted privileged mutations. No daemon or control socket
is involved. Git executes under the workspace identity, not as root with a
person's Git credentials.

The catalog is `/var/lib/soda/catalog/projects.json`; the catalog owner validates
immutable IDs and credential-free SSH addresses while preserving additional JSON
metadata. Its native lock is `/run/lock/soda/projects.lock`. Workspace operations
also use `/run/lock/soda/workspace-operations.lock`; these are ephemeral locks,
not workflow state. Browser listing reports derived account existence without
reading private homes or making Git-host calls.

Workspace classification uses native Linux group/association information under
the workspace owner. Consume returned usernames and actual homes instead of
reimplementing derivation in the browser. Setup preflights personal authorized
keys, prepares an account and outbound key, and performs native Git cloning.
A failed clone retains the account/key for explicit retry. Read-only `inspect`
checks the caller's existing keys and checkout; it creates nothing, does not
contact the Git host, refresh the index, or run filters. Read failures do not
establish absence. Inspection proves neither clean working-tree state nor full
historical Git object integrity.

Removal uses `removal-inspect` with a fixed action/target. Its preview includes
native usernames, UIDs, homes, associations, catalog presence, and a scope
revision. Execution takes `expected`, reauthorizes, and compares identities
under the operation lock; project removal also locks the catalog. Revisions
are stateless comparisons, not saved approvals or credentials. Cosmetic metadata
changes do not redefine repository identity.

`linuxhost.DeleteAccounts` terminates the selected accounts sequentially and
reports confirmed, uncertain, and unattempted results. Only the exact selected
user-manager failure can be reset; unrelated systemd failures are not cleared.
The helper supplies structured stdout receipts even for incomplete deletion;
stderr alone cannot reconstruct outcomes. A changed or missing failed identity
requires native inspection of its old data before a fresh task, not an automatic
cleanup or retry. See [the user procedure](public/50-Operate/40-data-safety-and-removal.md).

## Host integration

The runtime ships mandatory interactive welcome through
`/etc/profile.d/soda-console-welcome.sh`; non-interactive transports stay quiet.
The ISO Kickstart disables cloud-init and adds Cockpit's firewall allowance;
QCOW2 retains standard Fedora cloud-init. Inspect native service ordering after
provisioning and reboot, rather than inferring it from individual unit files.

Forgejo runs as `git`, with `/etc/forgejo/app.ini` and writable
`/var/lib/forgejo`. The shipped template selects SQLite, a native repository
root, HTTP TCP 30000, and external OpenSSH TCP 22 (`START_SSH_SERVER = false`).
`forgejo-init` seeds missing configuration and secrets, runs native migrations,
and adds the Soda PAM source. It does not create a site administrator or derive
Forgejo roles from `wheel`. Native PAM rejects workspace identities.

The Forgejo service alone receives `soda-forgejo-shadow` through systemd
`SupplementaryGroups`; `git` is not a permanent NSS member. Its named tmpfiles
rule and narrowly scoped SELinux policy allow the required shadow metadata
setup. Verify actual mode/group and real PAM login under enforcing SELinux;
a successful global tmpfiles pass or absence of logged AVCs is not sufficient.

Forgejo initialization advertises the static hostname until a reachable Tailnet
identity is available, preferring MagicDNS and otherwise Tailnet IPv4. HTTP binds
IPv4 across local interfaces; the provider/firewall controls ingress. On observed
connection, Cockpit calls `forgejo-init refresh-tailnet`: matching DOMAIN,
SSH_DOMAIN, and ROOT_URL cause no write/restart; stale values restart the service
through its existing inactive oneshot initializer. It never waits for cloud-final.
Tailscale's browser adapter uses native state, authentication streams, and
preferences. No enrollment or address-reconciliation daemon is added.

Local Runners stores descriptors and native provider state beneath
`/var/lib/soda/runners`, using dedicated accounts and `soda-runner@.service`.
Its narrow helper owns local changes; provider configuration holds long-lived
registration material where the provider needs it. The browser never caches
submitted tokens. Native provider deregistration and local removal are separate.

[Branding](branding.md) owns native Cockpit/Forgejo asset paths and configuration
precedence. These are image assets, not another application server.

## Soda Updates implementation

`cmd/soda-updates` is root-only, synchronous, not setuid: `status` and `check`
emit JSON; `download` and `apply` stream native progress. The runtime RPM supplies
the command/page and requires bootc, Skopeo, and Cosign. Bootc owns pending and
booted state; reload observes it rather than recovering a browser workflow.

Checks use GitHub `releases/latest`, require a published stable semantic version,
one host-architecture schema-3 record and bundle, and verify the fixed production
workflow identity. Version, platform, source, channel, and exact GHCR digest must
agree. Image signature, provenance, and anonymous Skopeo identity inspection
follow. Temporary record files are removed. An absent release or failed
verification must not be reported as an up-to-date result.

Download and Apply reverify the selected published version, not the latest
release at that later moment. Download uses `bootc switch --download-only` and
checks the exact target and download-only state. It rejects incompatible native
state including overlays, queued rollback, existing staged deployment, downgrade,
or unsuitable image identity. Apply rereads/reverifies the target, uses
`bootc switch --from-downloaded`, verifies activation, then requests normal reboot.

Bootc 1.16.10 has no atomic expected-target activation argument. The ephemeral
`/run/soda-updates.lock` serializes Soda mutations, not native administrator
commands. Before/after checks cannot eliminate that race. Users must coordinate
administration; a failed post-activation check prevents Soda's reboot request
but may leave changed pending state. No compensation rollback is attempted.
[Updates and fallback](public/30-Use-Soda/60-updates-and-fallback.md)
owns the supported user sequence.

## Release gaps and required evidence

These are internal qualification tasks, **not public release caveats**:

- The acceptance runner cannot yet supply `installed-onboarding-observations`,
  `trusted-lan-access`, or `public-ingress-rejection`. Its successful itinerary
  cannot qualify a release. Preserve the [coverage requirements](../tests/acceptance/README.md#run-reports-versus-qualification-schema-2).
- Exercise the cloud console → native Tailscale browser login → private Cockpit
  sequence, including Scaleway disk import, user-data, password login, and public
  ingress rejection. No cloud deployment was exercised by documentation work.
- Verify the release backup/restore journey on disposable systems, including
  coherent account/configuration/data restoration, Forgejo-native restore,
  UID/GID and SELinux preservation, and isolated Tailnet identity handling.
- The initializer/template uses OpenSSH 22. Welcome text and acceptance fixtures
  still mention opening 2222. Reconcile those runtime instructions before release;
  a harness forwarding port must not become a new appliance listener by accident.
- The initial Forgejo PAM change needs installed first/later-user login, explicit
  administrator creation and promotion, and preserved existing configuration.
- Full signed-release discovery, download, restart, and account-preserving
  fallback need matching-native evidence. Earlier x86-64 overlay previews proved
  status/elevation and clean refusal, not a real upgrade; old missing-Cosign
  observations do not describe the current source-built package.
- Frontend simulations, native RPM payload checks, installed page operations,
  accessibility/user review, and release screenshots are distinct evidence.
  Runners/Tailscale/Updates UX follow-up and both architectures' native browser
  runs remain tracked in [Cockpit development](cockpit-development.md).
- Build/probe history and architecture-specific remaining work live in
  [build and release](build-and-release.md); WSL has its own future-only
  [research scope](research/wsl.md).

Public guides describe the approved outcomes these tasks must establish. If a
mechanism cannot deliver one, return that exact gap for a decision; do not
publish fabricated steps or silently narrow the promised product.
