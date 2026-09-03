# Soda OS 0.5.0 release operations

Soda releases have three independent outputs:

1. GHCR holds one bootc OCI image per architecture. Its exact manifest digest
   is the installed system's update authority.
2. GitHub Releases holds a network installer ISO and checksum per architecture.
3. GitHub Releases holds a compressed reusable QCOW2 and checksum per
   architecture.

The raw QCOW2 is a matching-native build artifact, not a download. `aarch64`
and `x86_64` are equal sibling targets: each host builds, signs, publishes,
installs, and validates only its own architecture. `just check` is a source
contract check, never sibling-architecture artifact evidence.

## Identity and immutable outputs

`distro/soda.toml` is the reviewed source of Soda identity. The `0.5.0`
release uses `release/0.5.0`, tag `v0.5.0`, and these single-platform tags:

```text
ghcr.io/levitateos/soda-os:0.5.0-aarch64
ghcr.io/levitateos/soda-os:0.5.0-x86_64
```

Each is created once. Exact references are authoritative:

```text
ghcr.io/levitateos/soda-os@sha256:<aarch64-digest>
ghcr.io/levitateos/soda-os@sha256:<x86_64-digest>
```

There is no `latest`, `stable`, `edge`, moving version tag, or multi-platform
index. Source-revision candidates are
`sha-<full-source-revision>-<architecture>` and are not cleaned up
automatically after failure.

## Native artifacts

Each matching-native builder consumes its reviewed architecture-owned locks and
produces the OCI archive, network ISO, raw QCOW2, compressed QCOW2, checksums,
and schema-3 release record:

```sh
ARCH=x86_64 # use aarch64 only on a matching-native AArch64 host
just check
just rpm "$ARCH"
just oci "$ARCH"
ARCHIVE=".artifacts/images/soda-os-0.5.0-${ARCH}.oci.tar"
just iso "$ARCH" "$ARCHIVE"
just qcow2 "$ARCH" "$ARCHIVE"
just record "$ARCH" "$ARCHIVE" \
  ".artifacts/images/SodaOS-0.5.0-${ARCH}.iso" \
  ".artifacts/images/SodaOS-0.5.0-${ARCH}.qcow2" \
  ".artifacts/images/SodaOS-0.5.0-${ARCH}.qcow2.zst"
```

The network ISO contains no embedded Soda payload. Its Kickstart names one
exact, anonymously retrievable GHCR digest. A record binds that digest, Fedora
base reference, source revision, RPM inventory, ISO, raw QCOW2, and compressed
QCOW2 checksums.

Protected OEMDRV remains the installer input boundary. It is mode `0600`, is
ejected before installation proceeds, and is removed only after QMP proves the
tray is open.

## Fixed publication boundary

`soda-release` is an operator-side wrapper around Git, Skopeo, Cosign, and
GitHub CLI. It is not installed in Soda OS and creates no runtime release
service, release index, credential store, retry queue, or workflow state.

```text
soda-release image-stage --architecture ARCH --archive PATH
soda-release image-promote --architecture ARCH --record PATH
soda-release record-sign --architecture ARCH --record PATH
soda-release draft --notes-file PATH --aarch64-record PATH --x86_64-record PATH
soda-release upload --architecture ARCH --iso PATH --qcow2-zst PATH --record PATH --record-bundle PATH
soda-release publish --aarch64-record PATH --x86_64-record PATH
```

`image-stage` publishes and verifies an immutable source-revision candidate.
`image-promote` refuses an existing version tag, keylessly signs the exact
digest, attaches an SLSA provenance predicate, verifies both, then creates the
version tag. The predicate records source revision, architecture, Fedora base
digest, runtime-lock checksum, RPM inventory, and ISO/QCOW2 checksums.

`record-sign` creates `<record>.sigstore.json` and immediately verifies it.
Image and record signing require a GitHub Actions workflow identity. `draft`
requires both records and notes containing both exact image digests. `upload`
accepts exactly six assets per architecture:

```text
SodaOS-0.5.0-<architecture>.iso
SodaOS-0.5.0-<architecture>.iso.sha256
SodaOS-0.5.0-<architecture>.qcow2.zst
SodaOS-0.5.0-<architecture>.qcow2.zst.sha256
soda-os-0.5.0-<architecture>.release.json
soda-os-0.5.0-<architecture>.release.json.sigstore.json
```

`publish` refuses to change a draft unless all twelve assets, both signed
records, both immutable version tags, anonymous exact-digest pulls, image
signatures, SLSA attestations, release notes, and the remote release-branch
revision agree. It runs only `gh release edit --draft=false --latest`; it never
overwrites, deletes, compensates for, or repairs a partial remote result.

OIDC is short-lived authentication only. GHCR stores OCI images, signatures,
and attestations; GitHub Releases stores downloadable assets and signed records.

## Protected CI

`.github/workflows/ci.yml` is read-only source verification.
`.github/workflows/release.yml` runs only on `release/**`, rejects a branch
whose suffix is not the reviewed Soda version, and serializes release runs
without cancellation.

Native jobs use GitHub-hosted x86-64 or AArch64 orchestrators, then join the
Tailnet through the SHA-pinned Tailscale action and workload identity
federation. The identity has `id-token: write`, the configured client ID and
audience, and `tag:soda-release-ci`; it does not use a reusable Tailnet key or
attach a persistent GitHub self-hosted runner.

The hosted runner reaches only a matching tagged builder through Tailscale SSH
as `soda-release-ci`. The root-owned login shell is
`soda-release-executor`, which accepts only `prepare`, `emit-record`, and
`finalize`, deriving every path from run ID, attempt, source SHA, and
architecture. It accepts no caller path or arbitrary command.

Each acceptance VM gets a new one-use ephemeral `tag:soda-ci-guest` key.
`scripts/release-create-vm-auth-key.sh` exchanges the GitHub OIDC JWT for a
short-lived Tailscale API token, creates the key, prints it only to stdout, and
deletes local token material. The workflow pipes it directly to the executor;
it is never an argument, environment value, artifact, evidence file, or repo
file.

The native-host administrator separately provisions this account and root
configuration before enabling the workflow:

```text
account: soda-release-ci
password: locked
home: private; no personal GitHub, Codex, or SSH state
sudo/linger/cron/user services: none
groups: docker and kvm only
login shell: root-owned soda-release-executor
config: /etc/soda-release-ci.conf
```

The configuration supplies only the root-owned release workspace, read-only
Soda repository URL, and immutable `SODA_RESET_BASE_SHA`. Docker membership is
root-equivalent on a shared host; that is an explicit accepted risk, not an
isolation claim. GitHub environments/rulesets, GHCR visibility, the Tailscale
federated identity and ACLs, and host account creation are external operations
that repository commits do not perform.

## Acceptance and updates

The native `prepare` phase builds post-reset fallback A and release B, stages
both candidates, creates network/QCOW2 artifacts, and runs the sole public
installed-product workflow:

```sh
tests/acceptance/unattended.sh run
```

It proves installation, Tailnet enrollment, onboarding, Projects, direct SSH,
immutable tools, zero obsolete control plane, and B-to-A-to-B preservation.
NoCloud, ConfigDrive, and no-datasource QCOW2 scenarios are architecture-native
release evidence too.

Installed administrators choose an exact published GHCR digest:

```sh
sudo bootc status
sudo bootc switch --download-only ghcr.io/levitateos/soda-os@sha256:<digest>
sudo bootc switch --from-downloaded
sudo systemctl reboot
```

Fallback uses the same sequence with an earlier exact Soda digest. Direct
`bootc rollback` is unsupported. Soda has no runtime release discovery,
automatic download, activation, or update service.
