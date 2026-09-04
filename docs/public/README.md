# Public website documentation

This directory contains release-day user documentation synchronized into the
Soda OS website. It describes the stable finished-product journey and does not
expose implementation history, rejected designs, or development status.

Product purpose and ownership come from [`docs/principles.md`](../principles.md).
Accepted behavior comes from
[`docs/architecture-reset.md`](../architecture-reset.md). This directory is a
presentation layer, not an independent source of product requirements.

`manifest.json` defines page order, routes, descriptions, evidence paths, and
related issues. Public Markdown pages contain no level-one heading because the
website supplies the title from the manifest. Keep existing routes stable.

The website synchronizer rejects raw HTML, images, unsafe links, broken local
links, and malformed page structure. It renders a deterministic snapshot; the
deployed website never fetches this repository at runtime.
