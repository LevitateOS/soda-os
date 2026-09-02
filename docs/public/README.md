# Public website documentation

This directory owns the public wording synchronized into the Soda OS website.
It is a presentation layer, not an independent source of product requirements.

Product purpose and ownership come from [`docs/principles.md`](../principles.md).
Accepted behavior and boundaries come from
[`docs/architecture-reset.md`](../architecture-reset.md). Current implementation
claims must be supported by the current source, tests, and implementation
documentation. When those sources disagree with this directory, correct the
public wording rather than treating it as authority.

`manifest.json` defines page order, routes, descriptions, evidence paths, and
related issues. Each Markdown page deliberately contains no level-one heading;
the website supplies its title from the manifest. Every page distinguishes the
accepted **Product contract** from the **Current implementation** at the source
revision recorded by the website snapshot.

The website synchronizer rejects raw HTML, images, unsafe links, broken local
documentation links, and missing required sections. It renders and commits a
deterministic HTML snapshot so the deployed website never fetches this
repository at runtime.
