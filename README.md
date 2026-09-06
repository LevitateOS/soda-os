# Soda OS

![Soda OS](assets/branding/source/soda-logo-horizontal.svg)

Soda OS combines Fedora bootc, OpenSSH, Cockpit, Forgejo, Tailscale, and mise
into a human-owned remote development system. Use a powerful computer as your
remote environment or additional capacity; give a trusted team one shared
foundation with a separate Linux workspace and clone per developer-project pair.

Connect directly over a trusted LAN or privately through Tailscale in the cloud.
Keep your preferred editor, terminal, and client. AArch64 and x86-64 are equal
image/installer targets. WSL2 on x86-64 Windows is planned for a future release,
not a launch download.

## Use Soda

The [public handbook](docs/public/10-Start-here/10-index.md) describes the approved
release-day product, not this checkout's readiness. Start with installation and
first connection, then use Projects to create your own workspace. Cockpit owns
administration; Forgejo or your external Git host owns repositories and collaboration.

## Develop Soda

- [Documentation map](docs/README.md): product, implementation, operations, evidence.
- [Product contract](docs/product-contract.md): accepted outcomes and native ownership.
- [Contributor development](docs/development.md): tools and the full Linux source gate.
- [Architecture](docs/architecture.md): current source owners and release gaps.
- [Build and release](docs/build-and-release.md): separately authorized artifact work.
- [Acceptance](tests/acceptance/README.md): matching-native checks and qualification.
- [Agent guidance](AGENTS.md) and [Go command standard](cmd/AGENTS.md): working rules.

Run `just check` in the documented Linux environment. Generated assets and build
outputs stay untracked. Source checks, artifact builds, installed behavior, and
release qualification are distinct evidence; none implies publication approval.
