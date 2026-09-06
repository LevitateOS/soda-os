# Contributor development

Work in the canonical checkout and read [agent guidance](../AGENTS.md) and the
[product contract](product-contract.md) before changing behavior. This is an
internal source workflow, not an installation prerequisite for Soda users.

## Tools and source checks

Use the Go version in `go.mod`, `just`, Git, and the pinned Cockpit toolchain.
`cockpit/package.json`, `cockpit/.node-version`, and its lock own frontend
versions; do not duplicate those pins in documentation. The host also needs
librsvg for asset freshness tests and the native utilities used by script tests.

Install Vite+ through `scripts/install-cockpit-toolchain.sh` after reviewing its
host changes. It installs the pinned CLI and configures shell integration;
restart the shell or use its reported PATH. CI uses the pinned setup action.

On Linux, as an ordinary user:

```sh
just check
```

The [justfile](../justfile) is the complete source gate: formatting, complexity,
release/acceptance source checks, frontend install/check/build/test, Go vet,
acceptance race tests, Go tests, and both architecture-static image checks.
Run focused tests while editing; do not replace the final gate with a subset.
For concurrent runtime changes, also run the affected package's race tests.

On macOS, the Linux runtime packages cannot be tested by substituting Darwin
APIs. From a clean committed checkout, run the existing matching-native wrapper:

```sh
scripts/check-native.sh aarch64
```

On x86-64 hardware select `x86_64` instead. The wrapper verifies the local Linux
Docker daemon and integrated Buildx worker, builds the development check image,
and runs unchanged `just check` as unprivileged UID 1000 in a disposable native
Linux container. It uses Go from `go.mod`, the existing frontend installer, Node
from `.node-version`, and source-test dependencies in `tools/check/Containerfile`.
Dependency downloads and Docker cache writes occur; no image is published.

The canonical checkout is read-only. A private exact-commit clone in the
container receives generated output and Linux dependencies, not host
`node_modules`. No host Docker socket, writable source mount, credentials, or
privileged mode is passed to that check container. Source must remain clean at
the same revision afterward. There is no skip flag or persistent success stamp.

## Finding code

[Architecture](architecture.md) maps native responsibilities. Start a browser
interaction at its feature's `store.ts`, then adapter, protocol, and native
command. [Cockpit development](cockpit-development.md) owns the frontend
commands and simulated versus installed browser tests.

`cmd/AGENTS.md` governs all Go code under `cmd`, including tests. Generated
frontend output, dependencies, and `.artifacts/` are not committed. Do not edit
generated files instead of their sources or update locks merely to hide a
failed build.

For artifact work, read [build and release](build-and-release.md) before using
`just rpm`, `oci`, `iso`, or preparation scripts. Those are not source-only
checks, and some wrappers publish to GHCR. Building an image and booting it are
separate operations with separate evidence.

## Debugging and evidence

1. Record the source, exact package/artifact, architecture, and environment.
2. Identify the last passing boundary and first failing one before changing code.
3. Inspect the exact upstream implementation and configuration, not a plausible
   generic explanation. Native defaults and optional features are not mandates.
4. Check actual output state. A successful `restorecon`, tmpfiles invocation,
   service start, or request does not prove its expected side effect.
5. Keep unit tests for inputs, ordering, and failure propagation. Add the missing
   native probe, artifact inspection, or installed observation; do not claim that
   mocks establish filesystem, authentication, or service behavior.
6. Preserve sanitized evidence, including partial success, and clean only exact
   disposable resources. Never send passwords through traced commands or PTYs.

Distinguish source proof, build proof, artifact proof, installation proof,
behavior proof, and full sibling release qualification. A later level does not
follow automatically from an earlier one. New tests must inspect native state,
not a hostname, port, output shape, or identity invented for the harness.

## Documentation changes

Public prose follows the approved release experience, not present readiness.
Keep gaps and evidence in the internal owning document. Follow the
[handbook authoring contract](public/README.md); the website owns its renderer
and generated snapshot. Before deleting a document, preserve unique active
requirements in their owner and update incoming links. Historical observations
remain in pinned Git history, not a line-neutral archive.
