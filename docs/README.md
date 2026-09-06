# Documentation map

Soda has two audiences and one accepted product contract. The public handbook
ships on release day and describes the finished product; internal documentation
records how we build it and what still needs proof. Missing implementation is
release work, not a reason to weaken public instructions.

## Maintained documents

| Start with | Responsibility |
| --- | --- |
| [Product contract](product-contract.md) | Accepted purpose, release behavior, and native ownership |
| [Public handbook](public/10-Start-here/10-index.md) | Installation, use, development, and operation |
| [Handbook authoring](public/README.md) | Source structure and publishing contract |
| [Screenshot brief](screenshot-capture.md) | Unpublished capture requests and image acceptance |
| [Development](development.md) | Contributor tools and source checks |
| [Architecture](architecture.md) | Current code, integrations, state, and release gaps |
| [Cockpit development](cockpit-development.md) | Frontend ownership, interaction rules, browser verification |
| [Build and release](build-and-release.md) | Native artifact preparation, publication, and evidence boundaries |
| [Acceptance](../tests/acceptance/README.md) | Required scenarios, runner coverage, qualification blockers |
| [Branding](branding.md) | Artwork, native integrations, regeneration, and validation |
| [WSL research](research/wsl.md) | Future x86-64 Windows investigation, not a supported installation guide |
| [Agent guidance](../AGENTS.md) | Work authorization, quality, and decision discipline |

Each procedure has one owner. Link to public user procedures from engineering
references, and to native upstream manuals where their responsibility begins.
Do not copy product requirements, command inventories, or implementation status
into every document. Package-local documentation stays beside its package.

## Historical evidence

The renewal removes superseded plans and bulk logs from the working tree, not
from Git history or retained evidence storage. These pinned records preserve
observations under their original contracts; they are not current instructions:

- [Pre-renewal documentation at c84a60e](https://github.com/LevitateOS/soda-os/tree/c84a60e/docs).
- [Incident notes and exact native candidate identities](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/bug-notes.md).
- [Postmortem decision ledger](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/architecture-reset-2.md).
- [Candidate investigation](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/native-iso-candidate-investigation.md).
- [Native build/probe evidence](https://github.com/LevitateOS/soda-os/blob/c84a60e/docs/release-operations.md).

When recording new evidence, use the owning issue or acceptance output. Include
source/artifact identity, architecture, last passed and first failed boundary,
actual side effect, remaining proof, and exact-resource cleanup. Do not recreate
a chronological all-purpose bug log or turn an old run into current qualification.
