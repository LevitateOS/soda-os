# Public website documentation

This directory contains the public Soda OS handbook. It describes the finished
product that owners, administrators, and developers use. The handbook is a
presentation of accepted product behavior, not an independent source of product
requirements.

Product purpose and ownership come from [`docs/principles.md`](../principles.md).
Accepted behavior and boundaries come from
[`docs/architecture-reset.md`](../architecture-reset.md). Repository source and
tests provide implementation evidence; they do not narrow the intended product
described here.

Published navigation is derived from the directory and file names. `README.md`
remains at the root of this directory and is not published.

Each section is a direct child folder named `NN-Title`, and each page is a
direct child of its section named `NN-slug.md`. The two-digit numeric prefixes
determine section and page order. The text after a section prefix supplies its
navigation title, and the text after a page prefix supplies its route slug.

Each published page:

- begins with exactly one level-one heading containing its title;
- places its page description in the first paragraph after that heading;
- links to other handbook pages through their real relative Markdown paths;
- explains unfamiliar terms at first use;
- identifies who performs a task and what they need;
- gives concrete steps and the expected successful result; and
- puts safety and irreversible data-loss information beside the relevant
  action.

Conceptual pages may use a responsibility table or a short guided journey in
place of procedural steps. Public wording describes launch behavior directly;
it does not split pages into product-contract and implementation-status
sections.

The website renders a deterministic snapshot of these sources. Publication is
a separate operation performed only after the shipped product and handbook
agree.
