# Module `billing`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Price table, cost postings, totals per run, thread, space, agent, reconciliation.

## Port (root package `internal/billing`)

- Inbound: `Costs` (posting, correction, totals), `Prices`.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

## Dependencies

Must not use the ports of any other module.

Reason: cost rows carry copies of thread, channel, space and agent (03); no lookup needed.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`price_table`, `cost_entries`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile`, `cost.reconcile.submit`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `billing.reconcile` | `cost.reconcile` or periodic, if enabled | `default` |

## Notes for the build

- Amounts in micro-units with a currency; corrections only as a new row with `supersedes_id` (03, 4.9).
- With `billing_mode = personal`, amounts are for information only.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)
