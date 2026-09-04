# Development tools

Use `mise` to install and select tools for one workspace or share the project configuration with every workspace.

Soda supplies the operating-system capabilities needed to host workspaces.
`mise` owns development-tool discovery, downloads, versions, activation, and
project configuration.

## Prerequisites

- Connect as the derived workspace user.
- Change to the repository under `$HOME/Projects/REPOSITORY`.
- Decide whether the tool is personal to this workspace or shared by the
  project.
- Review project configuration before trusting or executing it.

## Choose the scope

When Projects offers initial tool choices, select as many as the work requires.
Those choices are conveniences, not a closed list.

- Choose **my workspace** for a personal tool or version that should affect
  only your derived account.
- Choose **this project** for a tool and version the team should share across
  workspaces for this project.

Any project user may add shared project tools. Soda does not add a separate
approval or membership system around `mise`.

## Inspect the active configuration

From the repository directory:

```sh
cd "$HOME/Projects/REPOSITORY"
mise config ls
mise ls
```

Review an existing `mise.toml` before trusting and activating it. It is project
code and may define environment behavior or tasks.

## Add a shared project tool

Use `mise use` from the repository and commit the resulting project
configuration when the team should share it:

```sh
cd "$HOME/Projects/REPOSITORY"
mise use TOOL@VERSION
git diff -- mise.toml
git add mise.toml
git commit
```

Teammates then activate the configured tools from their own workspace:

```sh
mise install
```

Follow [mise getting started](https://mise.jdx.dev/getting-started.html) for
supported tool names, versions, configuration, activation, and trust behavior.

## Add a personal workspace tool

Keep personal configuration out of the shared project file. Use the personal
scope offered by Projects or a `mise.local.toml` ignored by Git, following the
upstream `mise` configuration guidance.

Check the affected files before committing so a personal choice does not
accidentally become project policy.

## Shared downloads and private installations

For a shared project tool, Soda gives `mise` one project-scoped storage location
that every workspace for the project can use. The selected tool is stored once,
while `mise` and the relevant package ecosystem own its layout, download cache,
locking, and concurrency.

Project dependencies, virtual environments, build output, and other mutable
development state remain private to each workspace. Soda does not define a
cache format or run a cache service or dependency downloader.

## Coding assistants

Coding assistants are personal workspace tools. Select or install an assistant
for the workspace that will use it, then authenticate inside that workspace.
Do not store assistant credentials in shared project configuration or copy them
between workspace homes.

## Expected result

The project records shared tool intent and stores the shared tool once. Each
workspace activates it with private dependencies and mutable state, while
personal tools remain private.

## If something fails

Run `mise doctor` and `mise config ls`, then follow the diagnostic guidance for
the tool backend in the upstream documentation. Do not work around a shared
permission problem by copying another user's installation or credentials.

## Next step

Return to [Connect and develop](30-connect-and-develop.md), or read
[Administration](../40-Operate-Soda-OS/10-administration.md) if you operate the
machine.
