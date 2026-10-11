---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Documentation site generator

## Context and Problem Statement

The specification, the ADRs and the guides are Markdown in the repository. They should be
readable as a site, with search, navigation and Mermaid diagrams, and the build should act
as a gate that finds broken references. Material for MkDocs has been in maintenance mode
since 2025-11-05; its author is working on Zensical
(https://squidfunk.github.io/mkdocs-material/blog/2025/11/05/zensical/, retrieved
2026-10-10) [verified]. Material 9.7.5 limits MkDocs to versions before 2.0
(https://squidfunk.github.io/mkdocs-material/changelog/, retrieved 2026-10-10) [verified].

## Decision Drivers

* Strict build as a gate: missing pages and broken links fail.
* Mermaid, search, light and dark mode without our own development.
* Python toolchain like the other repository tools (one uv lock).
* A planned, low-risk switch when the successor is mature.

## Considered Options

* MkDocs 1.6 with Material 9.7, pinned, switch to Zensical as a planned step
* Zensical right away
* Docusaurus
* Hugo
* Only the Markdown rendering of GitHub

## Decision Outcome

Chosen: "MkDocs with Material, pinned", because it is stable today, `--strict` checks
references and Zensical reads `mkdocs.yml` natively (https://zensical.org/docs/get-started/,
retrieved 2026-10-10) [verified], so the switch stays possible without reworking the content.
Versions are in `tools/uv.lock`; Renovate proposes updates. The switch to Zensical
will be its own pull request as soon as Zensical 1.0 has been released and the features in use
(search, Mermaid, navigation, light/dark) have been proven in the build of this repository.

End of support for Material: with version 9.7.0 of 2025-11-11 its author commits to "critical
bug fixes and security updates for Material for MkDocs for 12 months at least"
(https://squidfunk.github.io/mkdocs-material/changelog/, retrieved 2026-10-11) [verified]. The
commitment therefore ends on 2026-11-11 at the earliest [derived]; the source names no fixed end
date. On 2026-10-11 Zensical was at 0.0.69, status "Alpha"
(https://pypi.org/project/zensical/, retrieved 2026-10-11) [verified]. Triggers for the switch,
whichever comes first:

1. Zensical 1.0 is released (the regular case above).
2. After 2026-11-11 a security vulnerability in Material becomes known that affects the delivered
   site (search, theme JavaScript), and there is no fix. Then the site switches to the Zensical
   version current at that time, even before 1.0, if `make docs` and `make linkcheck` are green
   with it; otherwise the affected feature is turned off until the switch is possible.
3. A Python or MkDocs version that the repository needs is no longer supported by Material.

Until then Material stays pinned; the build itself runs only in our own CI and processes only
content of this repository.

Further rulings: system fonts instead of web fonts (no request to third-party servers when
reading), anchors in GitHub style (`pymdownx.slugs.slugify`) so that links work the same on
GitHub and in the site; the site is published only in the public state (`docs.yml`). The API
contracts (`docs/api/`) are in the site as files; there is no embedded Redoc or Swagger
interface in M0: both are JavaScript applications that, when embedded, either need a CDN or
would have to be maintained as additional assets, and the API still changes often during
planning. A statically generated reference (`redocly build-docs`, available in the pinned
Redocly CLI) is its own ticket as soon as the API is stable.

### Consequences

* Good, because `make docs` is a strict gate today.
* Bad, because Material gets no more fixes after the end of the security commitment (2026-11-11
  at the earliest); the switch is planned, the triggers are listed above.
* Bad, because Mermaid is loaded in the site via JavaScript from a CDN; this affects only
  reading the site, not the build.

### Confirmation

`make docs` (part of `make check`) builds with `mkdocs build --strict`; the navigation in
`mkdocs.yml` references every page that must exist, and a missing page makes the build fail.
`make linkcheck` additionally checks links and anchors of all Markdown files outside of
`docs/`.

## Pros and Cons of the Options

### MkDocs with Material, pinned

* Good, because mature and with a strict mode.
* Bad, because in maintenance mode.

### Zensical right away

* Good, because the designated successor.
* Bad, because version 0.x; on the day of retrieval the website and PyPI named different
  versions (PyPI 0.0.69) [verified, https://pypi.org/project/zensical/, retrieved 2026-10-10].

### Docusaurus

* Good, because widespread and active.
* Bad, because a second toolchain (React, Node) only for docs.

### Hugo

* Good, because a single binary and fast.
* Bad, because Mermaid, search and strict link checking would be extra work.

### Only GitHub rendering

* Good, because nothing to build.
* Bad, because no search, no navigation and no gate for broken references.

## More Information

Check of the switch: ticket in the plan; basis: planning basis R-Doku-Site.
