Soda OS image deployment stays a native bootc responsibility. Soda defines the
supported outcome and builds the appliance image; it does not add a parallel
runtime update system.

## Product contract

Linux administrators explicitly inspect, stage, activate, and fall back between
exact Soda image references using native bootc operations. The automatic update
timer is disabled. Soda has no update scheduler, release-discovery client,
translated deployment database, background updater, custom update page, or CLI
wrapper.

Normal image replacement and supported fallback must preserve current primary
and workspace accounts, passwords, groups, administrator membership, homes,
clones, the project catalog, Forgejo state, Tailscale identity, and other
machine-specific state.

Supported fallback creates a new deployment from an earlier exact Soda image
through the same native switch path used for image selection. Direct `bootc
rollback` is unsupported because it can restore an older deployment's `/etc`
rather than preserve the machine's current account state.

Publication is separate from installed operation. Images belong in the
registry by immutable digest; installer assets belong in releases. The website
may eventually link to those releases, but it is never update authority.

## Current implementation

The image masks the automatic bootc update timer and retains the native bootc
commands. Soda's former runtime updater, discovery client, state translation,
API, and update UI have been removed.

Native x86-64 A-to-B-to-A-to-B image selection has preserved current mutable
Linux, workspace, catalog, Forgejo, Tailscale, and SSH state while moving to an
earlier exact image and then forward again. Matching-native AArch64 must repeat
the current path before release-level completion.

There is currently no public production release, signed artifact set, or
published update digest. The custom pre-reset GitHub publication client remains
a deletion target, and the final maintained publication boundary is not yet
implemented. Local unsigned candidate artifacts and acceptance records are
development evidence, not a channel for installed systems.

The final acceptance workflow and removal of the temporary health-only runtime
shell also remain open milestones. Until public releases exist, this page is a
description of the accepted lifecycle and current evidence rather than an
operator runbook.
