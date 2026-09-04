# Public website documentation

This directory contains release-day user documentation synchronized into the
Soda OS website. It describes the stable finished-product journey and does not
expose implementation history, rejected designs, or development status.

Product purpose and ownership come from [`docs/principles.md`](../principles.md).
Accepted behavior comes from
[`docs/architecture-reset.md`](../architecture-reset.md). This directory is a
presentation layer, not an independent source of product requirements.

Published navigation is derived from the directory and file names. `README.md`
remains at the root of this directory and is not published.

Each section is a direct child folder named `NN-Title`, and each page is a
direct child of its section named `NN-slug.md`. The two-digit numeric prefixes
determine section and page order. The text after a section prefix supplies its
navigation title, and the text after a page prefix supplies its route slug.

Each published page begins with exactly one level-one heading containing its
title. Its first paragraph is the page description. Links between handbook
pages use their real relative Markdown paths.

The website synchronizer rejects raw HTML, images, unsafe links, broken local
links, and malformed page structure. It renders a deterministic snapshot; the
deployed website never fetches this repository at runtime.
