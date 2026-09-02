Soda OS keeps general machine administration in Linux and stock Cockpit. Its
one focused Projects page exists only for the product-specific catalog and
workspace lifecycle.

## Product contract

Every primary Linux account can authenticate to Cockpit. Linux `wheel`
membership determines administrator status; Soda keeps no separate role or
registration record.

Stock Cockpit owns overview, metrics, services, logs, accounts, terminal,
storage, networking, browser sessions, TLS, PAM authentication, and privilege
elevation. Soda adds branding and one Projects page for:

- catalog discovery and exact catalog-entry changes;
- native empty Forgejo repository creation;
- **Set up for me**;
- destructive project removal; and
- administrator-only Soda-aware human deletion.

Project removal deletes every validated local workspace for that project before
removing the catalog entry last. It never deletes the canonical repository.
Soda-aware human deletion removes the selected person's validated local
workspaces before deleting their primary Linux account last. It never deletes
Forgejo accounts or repositories.

Generic Cockpit account deletion or direct `userdel` is an ordinary,
non-cascading Linux action. Soda does not watch for it or repair its effects.

## Current implementation

The image ships Fedora's stock Cockpit host packages, Soda branding, and the
static `soda-projects` package. The Projects UI executes a per-request
coordinator as the authenticated primary user. Root-required catalog and Linux
mutations pass through one exact-path polkit action and a narrowly parameterized
synchronous helper.

The helper exposes no arbitrary command, path, UID, process selector, generic
account API, Forgejo administration, or container management. It stores no
credential, job, retry, rollback, or reconciliation state.

The UI requires typed confirmation for destructive project and human removal
and states which local data will be permanently lost. Focused tests cover
administrator authorization, preflight behavior, process and account deletion,
catalog-last project removal, and primary-last human deletion. Final installed
multi-user destructive and partial-failure scenarios remain to be consolidated
in acceptance automation.

A temporary local health-only `sodad` and `sodactl health` surface remains in
the current image. It is not used by Cockpit or Projects and is scheduled for
deletion with the residual control-plane shell.
