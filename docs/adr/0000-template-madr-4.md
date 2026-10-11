---
status: template
date: 2026-10-11
decision-makers: Maintainer
---

# Template (MADR 4)

This file is the template for new architecture decisions. Create a copy as
`docs/adr/NNNN-title-in-kebab-case.md` with the next free number, text in English (D-W-94),
headings as below (MADR 4, https://adr.github.io/madr/, retrieved 2026-10-10).
Then run `make adr-index`. Procedure: [ADR 0001](0001-adrs-in-madr-format-decisions-as-ruling-log.md).

Front matter: `status` is `proposed`, `accepted`, `rejected`, `deprecated` or
`superseded by ADR-NNNN`; `date` is the day of the last change; `decision-makers`
names roles, not names of people.

## Context and Problem Statement

Which problem do we solve, in two to four sentences? What forces a decision
now? References to chapters and sources (URL, retrieved on, marker
[verified]/[derived]/[assumed]).

## Decision Drivers

* Driver 1, for example a requirement or an invariant
* Driver 2

## Considered Options

* Option 1
* Option 2 (at least two real options)

## Decision Outcome

Chosen option: "Option 1", because it is the only option that meets drivers 1 and 2.

### Consequences

* Good, because ...
* Bad, because ... (and how we deal with it)

### Confirmation

Which test, which gate or which check keeps the decision true in the repository?
Without a Confirmation an ADR is not finished.

## Pros and Cons of the Options

### Option 1

* Good, because ...
* Neutral, because ...
* Bad, because ...

### Option 2

* Good, because ...
* Bad, because ...

## More Information

Follow-up questions, time of the review, related ADRs.
