---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team B (API)
informed: all teams
---

# Error format RFC 9457, idempotency, pagination

## Context and Problem Statement

Clients work offline with an outbox and repeat requests (04-sync-protocol.md,
section 13). They must tell errors apart by machine (retry, step-up, discard,
reload) and load long lists page by page, without permission filters revealing quantities. Which
error format, which idempotency mechanism and which pagination apply to all operations?

## Decision Drivers

* One format for REST, gateway errors and tools; a standard instead of a custom design.
* Every write request can be repeated without duplicates (04, goal 2).
* A retry after `401 step_up_required` or `429` must work with the same ID.
* Lists reveal nothing about invisible objects (R-AZ-01, P2).
* Generators of all four languages must be able to type errors and pages.

## Considered Options

* Errors: RFC 9457 Problem Details with a `type` of its own per code.
* Errors: custom format `{"error": {"code", "message"}}`.
* Idempotency: client ID in the body for messages and interactions, `Idempotency-Key` for all
  other POST and PATCH.
* Idempotency: only `Idempotency-Key` for everything.
* Pagination: opaque cursor without a total count.
* Pagination: offset and limit with a total count.

## Decision Outcome

Chosen options: "RFC 9457", "Client ID plus Idempotency-Key", "Cursor without a total count",
because they follow standards, map the rules from 04 without exception and reveal no quantities.

Rulings (docs/spec/06-api.md, sections 5 to 8):

* `application/problem+json` with `type` as a relative URI `/api/v1/problems/{code}`, `code`,
  `correlation_id`; extensions `errors[]`, `current`, `reason`, `hint`. Every instance delivers
  the description of a code under this path.
* `400` for schema and rule violations, `422` only for `idempotency_key_reuse`, `409` for
  state conflicts, `412` for a failed `If-Match`.
* Messages: `client_msg_id`; interactions: `client_interaction_id`; other POST and PATCH:
  `Idempotency-Key` (UUIDv7). Only final responses are stored, not `401`, `429`
  and `5xx`.
* Cursors are bound to filter and sort order, keyset-based, without a total count. Thread history
  via `thread_seq`, events via `seq`.

### Consequences

* Good, because HTTP libraries and proxies know Problem Details and clients branch on `code`.
* Good, because the outbox can repeat every request blindly.
* Good, because keyset pages are stable and cheap under load.
* Bad, because relative `type` URIs deviate from the recommendation "absolute URIs" in RFC 9457; a
  later switch would be a breaking change.
* Bad, because without a total count the interface shows no "Page 3 of 12"; this is intended.
* Bad, because two idempotency paths exist; the reason is the permanent unique index for
  messages (04, 5.1), which has no expiry date.

### Confirmation

Lint (G1): every operation documents 4xx responses from `components.responses` with
`application/problem+json`; examples are checked against schemas. Schemathesis (G7) checks status,
media type and schema of every response and custom rules: problem format on every error,
`Correlation-Id` in every response, the same `404` for foreign and missing IDs. Integration tests
check retries with the same ID after a timeout, step-up and `429`.

## Pros and Cons of the Options

### RFC 9457

* Good, because a standard with a media type and extension rules (ignore unknown fields).
* Neutral, because `title` is English; clients translate via `code`.

### Custom error format

* Good, because shorter.
* Bad, because every tool must learn it anew; no media type to tell it apart.

### Client ID plus Idempotency-Key

* Good, because messages stay unique across days and reinstallations (04, 5.1).
* Bad, because two mechanisms in the docs.

### Only Idempotency-Key

* Good, because one mechanism.
* Bad, because a key with an expiry (`sync.idempotency_key_ttl`) does not protect outbox entries
  that are days old from duplicates.

### Cursor without a total count

* Good, because stable with new rows and without a second query.
* Bad, because no jump to an arbitrary page.

### Offset and limit with a total count

* Good, because familiar.
* Bad, because shifts with new rows; total counts over filtered sets are expensive and
  reveal sizes.

## More Information

* RFC 9457: https://www.rfc-editor.org/rfc/rfc9457.html (retrieved 2026-10-10) [verified].
* draft-ietf-httpapi-idempotency-key-header, draft -07 expired:
  https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/ (retrieved
  2026-10-10) [verified].
* RFC 9110, section 13.1.1 (If-Match, 412): https://www.rfc-editor.org/rfc/rfc9110.html
  (retrieved 2026-10-10) [verified].
