---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team apps and design
consulted: Team quality and repository
informed: all teams
---

# Design tokens as one source with a generator

## Context and Problem Statement

Colors, type, spacing, motion and density appear in the Flutter apps, in the web console, on the
docs site, on the status page and in previews. The requirement calls for a colorful, distinctive
design with a light and a dark theme, AA contrast and without a generic AI look. How does that
stay the same in all places, and how do "AA" and "no generic look" become verifiable instead of
an intention?

## Decision Drivers

- one source for all outputs (Dart, CSS, docs, preview)
- an open standard instead of an own format
- contrast as a test, not as a review
- make the ban list measurable where possible
- no supply chain in the gate, runs offline
- generated files verifiably up to date

## Considered Options

1. W3C DTCG tokens (format 2025.10, resolver 2025.10) plus a custom generator in Python without
   dependencies
2. W3C DTCG tokens plus Style Dictionary (Node) with custom formats
3. Figma as the source with a token plugin, export into the repo
4. Hand-written Dart theme, CSS maintained separately

## Decision Outcome

Chosen option: "DTCG plus a custom generator", because the format is a stable report of the W3C
Design Tokens Community Group (Format, Color and Resolver Module 2025.10, "This specification is
considered stable", https://www.designtokens.org/tr/2025.10/format/, retrieved 2026-10-10
[verified]), the resolver maps theme, density and motion without copies
(https://www.designtokens.org/tr/2025.10/resolver/, retrieved 2026-10-10 [verified]) and the
actual work is custom anyway: `ThemeExtension` classes, a contrast gate with declared pairings,
palette guards, a preview with embedded fonts. A generator in the Python standard library runs
with `uv` without a package download and has unit tests.

### Consequences

- Good, because every output is created from one source and `tokens.py check` finds deviations.
- Good, because AA contrast for every declared pairing in light and dark is a hard test.
- Good, because parts of the ban list (blurple, gray without a tint, beige, purple and
  terracotta accents, gradients) are tests.
- Good, because colors are written in OKLCH (even lightness steps) and are rejected outside of
  sRGB.
- Bad, because the generator knows only the used subset of the specification (it rejects
  `$extends` and JSON Pointer references in token files).
- Bad, because designers without JSON knowledge need a way to the tokens (DTCG-capable tools can
  read the files; an import is not part of this decision).

### Confirmation

- `uv run tools/design-tokens/tokens.py check` in the local gate: structure, contrast, guards,
  freshness of all generated files.
- Unit tests of the generator (`tools/design-tokens/tests`).
- Schema check against the official JSON schemas of the DTCG (command in
  `tools/design-tokens/README.md`).
- Golden tests in `packages/design_system` use only the generated classes.

## Pros and Cons of the Options

### DTCG plus a custom generator

- Good, because a standard format, no dependencies, gates that fit exactly.
- Bad, because custom code to maintain (small, tested).

### DTCG plus Style Dictionary

- Good, because a widespread tool with many formats.
- Bad, because a Node dependency tree in the gate; the needed formats (`ThemeExtension`, contrast,
  guards, preview) would still be custom code.

### Figma as the source

- Good, because design in the familiar tool.
- Bad, because an account at a vendor as the source of truth; not reproducible in the repo;
  checks only after export.

### Hand-written Dart theme

- Good, because no tool.
- Bad, because CSS, docs and preview drift apart; contrast only through review.

## More Information

- Source: `design/tokens/`, rules: `design/design-rules.json`, tool: `tools/design-tokens/`.
- Design and ban list: [docs/design/README.md](../design/README.md).
- Contrast formula according to WCAG 2.2 with the sRGB threshold 0.04045
  (https://www.w3.org/WAI/GL/wiki/Relative_luminance, retrieved 2026-10-10 [verified]).
