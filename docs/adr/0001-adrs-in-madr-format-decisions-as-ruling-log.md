---
status: accepted
date: 2026-10-11
decision-makers: Maintainer
---

# ADRs in MADR format, DECISIONS.md as the ruling log

## Context and Problem Statement

The product is built in milestones, often by unattended build sessions. Whoever changes
something later must know why it is the way it is, and which check holds the decision.
There are two kinds of decisions: significant ones (architecture, contracts, tools,
license, process) and small everyday rulings. Both need a fixed place.

## Decision Drivers

* Decisions must be findable with their reason and alternatives, also for agents.
* Every decision needs a check that holds it in the repository.
* Little effort for small rulings, otherwise they are not written down.
* Reviewable in the pull request like code.

## Considered Options

* MADR 4 for significant decisions plus `docs/DECISIONS.md` as the ruling log
* Only one continuous decision log
* ADRs in the Nygard format (context, decision, consequences)
* Decisions in issues or a wiki

## Decision Outcome

Chosen option: "MADR 4 plus ruling log", because MADR enforces options, drivers and a
Confirmation, and the log keeps small rulings cheap.

Rules: file name `docs/adr/NNNN-title-in-kebab-case.md`, front matter `status` and `date`,
headings as in MADR and text in English (D-W-94, 2026-10-11), at least two real options, one
Confirmation. An ADR starts as `proposed` in the pull request and becomes `accepted` or
`rejected` on merge. An outdated ADR is not deleted but becomes
`superseded by ADR-NNNN`. Small rulings are in `docs/DECISIONS.md` with date and reason;
chapters of the specification keep their own rulings
(for example R-QA-01 in chapter 23).

### Consequences

* Good, because every significant decision shows its alternatives and its check.
* Good, because the index (`docs/adr/README.md`) is generated and does not go stale.
* Bad, because an ADR is more text than a log entry; in return the log stays for
  small things.

### Confirmation

`make docs` calls `tools/docs/adr_index.py --check`: it fails on a wrong
file name, a duplicate number, a missing title or a stale index. Missing front matter
appears as `MISSING` in the index table and stands out in the review. The template is
[ADR 0000](0000-template-madr-4.md).

## Pros and Cons of the Options

### MADR 4 plus ruling log

* Good, because MADR prescribes the fields Decision Drivers and Confirmation.
* Good, because Markdown in the repository is reviewable and versioned.
* Neutral, because two places are maintained; the dividing line is "significant or not".

### Only a decision log

* Good, because minimal effort.
* Bad, because options and the check are usually missing.

### Nygard-style ADRs

* Good, because short.
* Bad, because there is no obligation to list options and a Confirmation.

### Issues or wiki

* Good, because discussion and result are in one place.
* Bad, because not versioned with the code and not checkable in the pull request.

## More Information

MADR: https://adr.github.io/madr/ (retrieved 2026-10-10) [verified]. Procedure for
contributors: `CONTRIBUTING.md`, section "Architecture decisions".
