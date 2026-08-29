# Soda API v2 contract

`SodaService` is a private gRPC API served over `/run/soda/sodad.sock`. The
daemon owns validation and must return canonical gRPC status codes; Cockpit and
CLI clients translate these codes without exposing raw internal errors.

## Status codes

- Invalid request fields, malformed UUIDs, and unsafe Git remotes:
  `InvalidArgument`.
- Missing people, SSH device keys, projects, worktrees, jobs, or installations:
  `NotFound`.
- Uniqueness conflicts: `AlreadyExists`.
- Valid requests rejected by current resource or provisioning state:
  `FailedPrecondition`.
- Authenticated callers without access to a resource: `PermissionDenied`.
- Unavailable daemon dependencies or services: `Unavailable`.
- Unexpected failures that cannot be mapped more precisely: `Internal`.

## Project sources and Git URLs

`ProjectSource.source` is authoritative and exactly one `empty` or `git` branch
must be set. A Git source is accepted only when `domain.ValidateGitRemoteURL`
succeeds before persistence. HTTP and HTTPS remotes containing any URL userinfo
are invalid. `ssh://` remotes may contain a username but never a password.
SCP-like SSH remotes such as `git@example.com:team/project.git` are accepted.
Daemon responses must never project an unvalidated stored Git URL.

## Personal workspace access

People do not contain SSH keys. A person may have multiple named
`SshDeviceKey` resources; labels are unique for that person and fingerprints
are globally unique. `CreateProject.initial_person_ids` must contain at least
one unique, existing person. Every membership owns exactly one internal
`Worktree`, presented to people as a personal workspace. Additional named
worktree creation is not part of this API.

Project SSH authorization is derived entirely from memberships, personal
workspaces, and active device keys. The project account selects the project;
the authenticated device-key fingerprint identifies the person.

## Manual OS updates

The OS update RPCs are administrative operations exposed only through the
privileged local daemon socket. Cockpit must additionally require a Soda
administrator session before calling them.

`CheckOSUpdate` resolves the signed paired release index once and returns an
exact `ghcr.io/levitateos/soda-os@sha256:...` identity only after signature,
`linux/arm64`, and state-schema-3 verification. `StageOSUpdate` accepts only an
exact Soda repository digest and independently repeats those checks before a
download-only bootc switch. `GetOSUpdateStatus` projects the booted and staged
deployments directly from `bootc status`; Soda does not persist a second
deployment record. `ActivateOSUpdate` requires `confirm_reboot = true` and is
the only OS update RPC permitted to request a reboot.
