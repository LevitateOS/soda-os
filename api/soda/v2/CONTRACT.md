# Soda API v2 residual contract

> [!IMPORTANT]
> This file describes the temporary two-RPC daemon surface that remains while
> the architectural reset is in progress. It is implementation evidence, not a
> target API or compatibility promise. The accepted architecture removes this
> generic gRPC control plane; see [the architectural
> reset](../../../docs/architecture-reset.md) and [issue
> #39](https://github.com/LevitateOS/soda-os/issues/39).

`SodaService` remains a private local gRPC service over
`/run/soda/sodad.sock`. Its current protobuf surface contains exactly these
methods:

- `Health`
- `GetHostStatus`

The identity, administrator-role, SSH-device-key, project, membership,
worktree, provisioning-job, and Forgejo projection/provisioning RPCs and
messages have been deleted. Linux, Forgejo/Git, the three-field project
catalog, the narrow synchronous workspace operation, stock Cockpit, and direct
OpenSSH now own those responsibilities; they must not be reintroduced through
this API.

## Temporary responsibility split

- `Health` reports the residual `sodad` process identity and version. The
  daemon, local socket, protobuf/gRPC transport, generated code, and remaining
  generic CLI plumbing are residual control-plane infrastructure owned by
  issue #39.
- `GetHostStatus` returns the currently sampled service, firewall, network,
  CPU, load, uptime, memory, and filesystem projection. This copied telemetry
  surface is owned for deletion by issue #34 after the stock Cockpit switch.

The pre-reset update RPCs and translated update path have been deleted. Linux
administrators use native `bootc` operations for exact-digest image selection
and account-preserving fallback.

These later issues own deletion of their complete residual slices. Their
temporary presence does not make telemetry, update translation, the daemon,
the control socket, protobuf, or gRPC part of the accepted Soda architecture.

## Current RPC behavior

`Health` returns `status = "ok"`, `service = "sodad"`, and the running Soda
version.

`GetHostStatus` returns `Unavailable` when host telemetry is not configured or
cannot be sampled. Otherwise it returns the current telemetry projection in
`HostStatus`.
