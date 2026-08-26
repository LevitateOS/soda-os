# Soda API v2 contract

`SodaService` is a private gRPC API served over `/run/soda/sodad.sock`. The
daemon owns validation and must return canonical gRPC status codes; Cockpit and
CLI clients translate these codes without exposing raw internal errors.

## Status codes

- Invalid request fields, malformed UUIDs, and unsafe Git remotes:
  `InvalidArgument`.
- Missing people, projects, worktrees, jobs, or installations: `NotFound`.
- Uniqueness conflicts: `AlreadyExists`.
- Valid requests rejected by current resource or provisioning state:
  `FailedPrecondition`.
- Authenticated callers without access to a resource: `PermissionDenied`.
- Unavailable daemon dependencies, services, or observers: `Unavailable`.
- Unexpected failures that cannot be mapped more precisely: `Internal`.

## Project sources and Git URLs

`ProjectSource.source` is authoritative and exactly one `empty` or `git` branch
must be set. A Git source is accepted only when `domain.ValidateGitRemoteURL`
succeeds before persistence. HTTP and HTTPS remotes containing any URL userinfo
are invalid. `ssh://` remotes may contain a username but never a password.
SCP-like SSH remotes such as `git@example.com:team/project.git` are accepted.
Daemon responses must never project an unvalidated stored Git URL.

## Event stream

`SubscribeEventsResponse.payload` contains either a domain event or a stream
control value. Initial connection and recovery use the explicit
`STREAM_CONTROL_REFRESH` value. `STREAM_CONTROL_UNSPECIFIED` is never a refresh
signal and must be treated as invalid stream data.
