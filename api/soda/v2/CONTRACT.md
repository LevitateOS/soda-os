# Soda API v2 residual health contract

> [!IMPORTANT]
> This file describes the temporary one-RPC daemon surface that remains while
> the architectural reset is in progress. It is implementation evidence, not a
> target API or compatibility promise. The accepted architecture removes this
> generic gRPC control plane; see [the architectural
> reset](../../../docs/architecture-reset.md) and [issue
> #39](https://github.com/LevitateOS/soda-os/issues/39).

`SodaService` remains a private local gRPC service over
`/run/soda/sodad.sock`. Its current protobuf surface contains exactly one
method:

- `Health`

The identity, administrator-role, SSH-device-key, project, membership,
worktree, provisioning-job, and Forgejo projection/provisioning RPCs and
messages have been deleted. Linux, Forgejo/Git, the three-field project
catalog, the narrow synchronous workspace operation, stock Cockpit, and direct
OpenSSH now own those responsibilities; they must not be reintroduced through
this API. The translated host-status and telemetry messages have also been
deleted. Stock Cockpit and ordinary Linux interfaces read generic system,
service, journal, storage, and network state directly from their native owners.

## Temporary responsibility split

- `Health` reports the residual `sodad` process identity and version. The
  daemon, local socket, protobuf/gRPC transport, generated code, and remaining
  generic CLI plumbing are residual control-plane infrastructure owned by
  issue #39.

The pre-reset update and telemetry RPCs and their translated state have been
deleted. Linux administrators use native `bootc` operations for exact-digest
image selection and account-preserving fallback. Administrators use stock
Cockpit or ordinary Linux tools for host inspection.

Issue #39 owns deletion of the residual Health shell. Its temporary presence
does not make the daemon, control socket, protobuf, or gRPC part of the accepted
Soda architecture.

## Current RPC behavior

`Health` returns `status = "ok"`, `service = "sodad"`, and the running Soda
version.
