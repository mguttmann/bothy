# Module `billing`

Status: the booking of run costs, the valid sums, the thread costs and the price table built (M2-22); the reconciliation with the invoice follows with M2-28, the aggregate cost statistics with M6-05.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Price table, cost postings, totals per run, thread, space, agent, reconciliation.

## Code

- `internal/billing` (`billing.go`): the port. `Costs` (`ValidateReports`, `BookRun`, `ThreadCosts`, `Totals`) and `Prices` (`List`, `Create`, `Close`) with the DTOs `CostReport`, `TokenCounts`, `BookRun`, `Booking`, `Entry`, `CurrencySum`, `ThreadCosts`, `RunCost`, `CostTotal`, `Money`, `TotalsQuery`, `Level`, `ChannelFilter`, `Total`, `Price`, `CreatePrice`, `ClosePrice` and the constants of `cost_entries.source` and of the billing modes.
- `domain/` (pure, no clock, database or other module): `report.go` (`ValidateReports`, `ValidCurrency`, `MaxAmount`), `price.go` (`PriceSet`, `Compute`, `ValidatePrice`, `ValidSourceURL`, the units), `booking.go` (`Book`), `sums.go` (`Valid`, `Sum`, `Breakdown`, `ShowAmounts`), `closing.go` (`CloseDecision`).
- `domain/startprices/`: the start file `start_prices.json` (embedded) and `Rows`, `Parse` (see "Start file").
- `app/`: `Service` implements both ports; `Config` binds `billing.show_personal_estimates` (`wire.BillingConfig`); `ports.go` holds the outgoing `Repository` with `NewEntry`, `TotalsFilter` and `ErrPriceOverlap`.
- `adapters/postgres/`: `queries/prices.sql`, `queries/cost_entries.sql`, the sqlc output in `sqlcgen/` (entry in `services/server/sqlc.yaml`) and `Repository`. Locks and writes run only inside a unit of work; SQLSTATE `23P01` and `23505` of `price_table` become `app.ErrPriceOverlap`; every other database error is internal and carries no SQL text or value.
- `adapters/httpapi/`: `Billing`, the strict-server handlers of `getThreadCosts`, `listPrices`, `createPrice` and `closePrice`; they convert and pass errors unchanged. The `Idempotency-Key` of the price changes is handled by the middleware of the chain (M1-20).
- `adapters/billingtest/`: test helpers, kept out of the binary (see "Tests").
- `internal/wire/module_billing.go` builds the module in the start step chat before runs (`startBilling`, `NewBilling` in `internal/wire/billing.go`) and mounts its handlers; `startRuns` hands `billing.Costs` to `runs`; `internal/wire/configkeys_billing.go` registers the key.

## Port (root package `internal/billing`)

- Inbound: `Costs` (posting, correction, totals), `Prices`.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

`ValidateReports` wraps the pure rules so `runs` checks a `finishRun` body before its unit of work without importing a foreign domain (02 5). `BookRun` and `Totals` never call `authorize`: `BookRun` runs inside the unit of work of `run.finish`, which `runs` authorized, and every caller of `Totals` (M6-05) authorizes first and passes the visible channels. `ThreadCosts`, `List`, `Create` and `Close` take the principal from the context (`kernel/actor`) and call `authorize` before anything else.

## Tables and migrations

- `migrations/00071_billing_price_table.sql`: `price_table` exactly as 03 4.9, the extension `btree_gist` and the exclusion constraint over `tstzrange(valid_from, valid_to)` per `provider`, `model` and `unit` (bounds `[)`, an absent `valid_to` is open), the unique key `(provider, model, unit, valid_from)` and the lookup index. `bothy_app` may read and insert and update only `valid_to` (D-M2-22-1).
- `migrations/00072_billing_cost_entries.sql`: `cost_entries` exactly as 03 4.9 with the checks of `billing_mode` and `source`, every foreign key `ON DELETE RESTRICT` (also `agent_session_id -> agent_sessions(id)`, left to this ticket by M2-12), the partial unique index on `supersedes_id` and the indexes per space, agent, thread, run and open estimate. `bothy_app` may read and insert only: entries are append only, a correction is a new row. M2-28 and M7-05 add the grants they need.

## Booking (`BookRun`, in the unit of work of `finishRun`)

The full order of the finish is in [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md) 12.1; `runs` owns the steps around the booking (see the runs README).

- **Reports** (`domain.ValidateReports`, pointers relative to the list, `runs` prefixes `/costs`): `provider` and `model` non-empty without control or bidi characters (`required`, `invalid`); token counts and `amount_micros` not negative (`too_small`) and below `2^53` (`too_large`); `currency` of the form `^[A-Z]{3}$` (`invalid_format`); a second report of the same `provider` and `model` is `invalid` (D-M2-22-2). Absent token counts are `0`.
- **Price set** (`domain.PriceSet`): every row with exactly equal `provider` and `model` that covers `occurred_at`, read `FOR SHARE` in ascending `id` (`LockPriceSet`).
- **Source and amount** (`domain.Compute`): `computed` when the set is not empty, has one currency and has a row for every unit whose count is above `0` (the unit `request` is ignored); the adapter's amount is then ignored. Otherwise `adapter_reported` with the report's amount and currency, otherwise `unpriced` with `0` and the report's currency. The amount is computed with `math/big`: the sum of count times `price_micros`, divided once by one million, rounded half up once per entry; at or above `2^53` the report is `400 validation_failed` (`too_large`) (D-M2-22-3). A booking is never recomputed later (D-M2-22-4).
- **Entries**: one per report in the order of the reports, with a new UUIDv7, `is_estimate = true`, no `reconciled_at` and `supersedes_id`, and the copies the caller passes (agent, thread, channel and space at booking time, session, `billing_mode = runs.billing_mode_used`, `occurred_at` = the end of the run). `Booking.Sums` holds one sum per currency in the order in which the currency first appears; `runs` writes the first into the run columns and adds each to the session sum (D-M2-22-7, D-M2-22-8). Currencies are never converted.

## Valid sum and levels

An entry is valid when no entry has `supersedes_id` equal to its `id` (03 4.9). Sums group valid entries per billing mode and currency: `amount_micros` is the sum, `is_estimate` is true when any summed entry is an estimate, token counts are summed per field. `Totals` gives them for the levels `run`, `thread`, `session`, `agent`, `space` and `instance`, in a half-open period on `occurred_at` and restricted to the caller's visible channels; a channel filter that is neither `All` nor has IDs returns nothing (fail closed, 03 principle 12). Levels use the copies at booking time; only the thread level follows the thread itself. With `ForDisplay` the display rule hides `personal` amounts.

## Display rule

`getThreadCosts` (`cost.read.thread`, `Level(read)` on the thread, no audit, `rl.read`): an unknown and an invisible thread give the same `404`. It reads the valid entries of the thread, whatever channel they were booked in, per run (by `run_id` ascending, unpaginated) and per currency. For a `personal` run `costs` is absent and its amounts are left out of `totals`, unless `billing.show_personal_estimates` is on; `billing_mode` and `tokens` are always present, `unpriced` marks "Price missing", and no person is named (D-M2-22-12, D-M2-22-13). The other cost members (`Run.cost`, `RunSummary.cost`, `/cost`, `AgentSession.cost`) carry the stored values in both modes; only `getThreadCosts` (later `getCostStats`) applies the switch (D-M2-22-14).

## Price table

- `listPrices` (`price.read`, owner, no step-up, no audit): every row, closed ones included, ordered by `provider`, `model`, `unit`, `valid_from`, `id`; unpaginated (D-M2-22-15).
- `createPrice` (`price.manage`, owner with step-up, audited, `rl.admin`): authorize, `domain.ValidatePrice` (`source_url` an absolute `https` URL without user info or fragment, `retrieved_at` not after now; D-M2-22-16), `409 price_currency_mismatch` for an overlapping row of the same provider and model in another currency, insert, `409 price_overlap` on an exclusion or unique violation (D-M2-22-17).
- `closePrice` (`price.manage`): authorize, lock the row `FOR UPDATE` (`404` for an unknown id), `domain.CloseDecision` (end not after start `400`; a closed row with the same end `200` and an audit entry `no_change`; another end `409 price_closed`), `409 price_in_use` when the new end cuts off a `computed` booking that counted the row's unit, then set `valid_to` (D-M2-22-18). A closed row is never reopened or moved.
- Audit of both (allow entries in the transaction): action `price.manage`, resource `instance`, attributes `price_id`, `op`, `valid_from`, `valid_to` when set, `retrieved_at`, `source_url` (value kind `auditing.SourceURL`, D-M2-22-19) and `no_change`. Denials are audited by `authz`.

## Start file

`domain/startprices/start_prices.json` (schema `start_prices.v1`) holds the base prices of the five token units for the models whose identifiers the model overview of the provider's documentation names, with the pricing page as `source_url`, its time of reading as `retrieved_at` and `valid_from = retrieved_at` (D-M2-22-20). It is embedded and checked by tests; the server never loads it, because loading it on the host needs a system action that 05 does not have (spec gap). Until one exists the owner enters prices with `createPrice`. Booking tests never read it: they take their prices from `createPrice` or the test helper.

## Losses and notes for later tickets

- **Server-ended runs book nothing** (D-M2-22-9): a run that ends `stale`, `interrupted`, `cancel_timeout`, `agent_disabled` or `access_revoked` without a later `finishRun` transition gets no entry, because `run.finish` is the only source of the usage per model (09 4.10). The consumption of such a run is lost; this is part of RK-16 (the estimate deviates from the invoice) until the reconciliation of M2-28 covers it.
- **No baseline** (D-M2-22-10): reported values are booked as the consumption of the run; `agent_sessions.cost_baseline_micros` is never subtracted (M8-06, RK-75).
- **For M2-28** (D-M2-22-11): a correction entry (`source = 'reconciled'`, `supersedes_id` on the estimate) repeats the token counts of the entry it supersedes, because the valid sum excludes superseded entries and token sums would otherwise drop. M2-28 adds the grants it needs (`cost_entries` has no `UPDATE` for `bothy_app`) and sets `runs.cost_is_estimate` and `agent_sessions.cost_is_estimate`.

## Tests

- `domain/domain_test.go`: `TestValidateReports`, `TestPriceSet`, `TestCompute`, `TestAmountProperty` (against a `math/big` reference with one rounding), `TestBook`, `TestValidatePrice`, `TestCloseDecision`, `TestSums`, `TestDisplayRule`.
- `domain/startprices/startprices_test.go`: `TestStartPrices_EveryRowHasSourceAndRetrievedAt`, `TestStartPrices_Valid`, `TestStartPrices_UnknownMemberRejected`.
- `migrations/billing_test.go`: `TestMigration_PriceTable`, `TestMigration_CostEntries` (columns, checks, keys, indexes, `btree_gist`, the exclusion constraint and the grants).
- `test/integration/costs/`: the cross-module tests over PostgreSQL 18 through `internal/wire` and the real HTTP chain (booking in `finishRun`, repeats, the cost rules, two currencies, the session sum, recomputation and closing races, `getThreadCosts` per mode and rights, the price operations).
- `adapters/billingtest`: `InsertEntry` writes an entry of any source, also `reconciled` with `supersedes_id`, for the tests of the valid sum; `RecomputeComputed` recomputes every `computed` entry from its price set with `domain.Compute` and reports each `Mismatch` (R-DM-23).

## Dependencies

May use the ports of these modules: none.

Reason: cost rows carry copies of thread, channel, space and agent (03); no lookup needed.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`price_table`, `cost_entries`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile`, `cost.reconcile.read`, `cost.reconcile.submit`.

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
