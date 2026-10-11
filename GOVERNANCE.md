# Governance

## Roles

- **Maintainers** decide what is merged and released, triage issues and keep the gates
  green. They are listed in `.github/CODEOWNERS`. The project starts with one maintainer.
- **Contributors** propose changes through pull requests (CONTRIBUTING.md). Anyone can
  become a maintainer after sustained, reviewed contributions, by agreement of the
  existing maintainers.

## How decisions are made

- Day-to-day changes: a maintainer approves the pull request after `ci-ok` is green.
- Decisions with lasting effect (architecture, public contracts, tool chain, licensing,
  process) are recorded as Architecture Decision Records in `docs/adr/` (MADR format,
  ADR 0001). An ADR starts as `proposed` in a pull request, is discussed there and
  becomes `accepted` or `rejected` when merged. Superseding an ADR needs a new ADR.
- Smaller rulings go into `docs/DECISIONS.md` with date and reason.
- With several maintainers: lazy consensus. Silence for the review period of a pull
  request counts as agreement; an objection needs a reason and an alternative. If
  maintainers cannot agree, the longest-serving maintainer decides and the ADR records the
  dissent.

## Triage of external issues

Every new issue gets `status: triage` from its template. A maintainer then:

1. Checks scope and duplicates. Out of scope or never-list items (docs/spec/01-product.md)
   get `status: declined` with the reason; duplicates get `status: duplicate` and a link.
2. Sets one `type: ...` label, one `area: ...` label and, for accepted work, one
   `priority: ...` label (`priority: high`, `priority: medium` or `priority: low`).
3. Asks for missing information with `status: needs-info`, or marks the issue
   `status: accepted`. Small, well described issues also get `good first issue` or
   `help wanted`.
4. Moves anything that looks like a vulnerability out of public view (SECURITY.md).

The targets below are named values in docs/spec/23-quality.md; a test keeps this table
and that chapter equal.

| Step | Target |
|---|---|
| First maintainer response to a new issue (`triage.first_response_target`) | 14 days |
| Close an issue waiting in `status: needs-info` (`triage.needs_info_close_after`) | 30 days |

## Security

Vulnerability handling follows SECURITY.md. Security fixes may be prepared in a private
fork and merged without the usual public review period.

## Changes to this document

Changes to governance are made by pull request and need the approval of every
maintainer.
