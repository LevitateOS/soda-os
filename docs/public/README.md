# Public website documentation

This directory contains release-day user documentation synchronized into the
Soda OS website. It describes the stable finished-product journey and does not
expose implementation history, rejected designs, or development status.

The [product contract](../product-contract.md) owns approved release-day behavior.
Current code and tests do not narrow that contract: missing implementation and
unverified mechanisms belong in internal release work, never public readiness
caveats. This directory presents accepted behavior, not speculative features.

Published navigation is derived from the directory and file names. `README.md`
remains at the root of this directory and is not published.

Each section is a direct child folder named `NN-Title`, and each page is a
direct child of its section named `NN-slug.md`. The two-digit numeric prefixes
determine section and page order. The text after a section prefix supplies its
navigation title, and the text after a page prefix supplies its route slug.

Each published page begins with exactly one level-one heading containing its
title. Its first paragraph is the page description. Links between handbook
pages use their real relative Markdown paths.

When a task leaves Soda's responsibility, link to the authoritative upstream
documentation at that point in the instructions. Keep Soda-specific guidance
focused on the product boundary and do not reproduce upstream manuals or
invent Soda wrappers for native tools.

The website synchronizer rejects raw HTML, unsafe/broken page and heading links,
and malformed page structure. It renders a deterministic snapshot; the deployed
website never fetches this repository at runtime.

## Screenshots

Keep real release-interface captures under `assets/cockpit/` or `assets/forgejo/`.
Use lowercase names with hyphens/underscores and `.png`, `.jpg`, `.jpeg`, or `.webp`.
The synchronizer checks signatures and paths; SVG, remote/data URLs, symlinks,
query strings, fragments, and references outside `assets/` are rejected.

Place a Markdown image after the description, beside the step it clarifies:

```markdown
![Projects showing a confirmed workspace and its SSH connection action](../assets/cockpit/projects.png)
```

Alt text describes the relevant visible information, not just “screenshot” or
a filename. Add a normal caption paragraph when interpretation needs context;
do not repeat the entire alt text. Inspect the real image for readability,
control names, and credentials before accepting draft alt text. Missing captures
stay in the unpublished [capture brief](../screenshot-capture.md), never as broken
links or invented interface illustrations in the handbook.

Commit prose and assets together. The website sync copies referenced images,
hashes source bytes, imports them through Vite's asset pipeline, and removes
obsolete generated copies. Run snapshot freshness/integrity and responsive,
light/dark browser checks there. Source assets remain here; generated copies
are never edited. File-format checks do not replace visual or accessibility review.
