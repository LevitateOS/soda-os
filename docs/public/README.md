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

`manifest.json` defines public section order, routes, titles, descriptions, and
source files. Issue numbers, commits, evidence paths, release readiness, and
implementation history do not belong in the public manifest or pages.

Each published page:

- contains no level-one heading because the website supplies its title;
- uses relative links to other handbook pages;
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
