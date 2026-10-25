# Module `billing`

Status: the booking of run costs, the valid sums, the thread costs and the price table built (M2-22); the reconciliation with the invoice built (M2-28); the aggregate cost statistics follow with M6-05.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Price table, cost postings, totals per run, thread, space, agent, reconciliation.

## Code

- `internal/billing` (`billing.go`): the port. `Costs` (`ValidateReports`, `BookRun`, `ThreadCosts`, `Totals`, `Reconcile`, `Estimates`) and `Prices` (`List`, `Create`, `Close`) with the DTOs `CostReport`, `TokenCounts`, `BookRun`, `Booking`, `Entry`, `CurrencySum`, `ThreadCosts`, `RunCost`, `CostTotal`, `Money`, `TotalsQuery`, `Level`, `ChannelFilter`, `Total`, `Price`, `CreatePrice`, `ClosePrice`, `ReconcileInput`, `ReconcileEntry`, `ReconcileResult`, `EstimatesQuery`, `Estimate`, `EstimatePage`, the in-transaction event `CostsReconciled` with `RunCostTotal` and `SessionCostTotal`, and the constants of `cost_entries.source` and of the billing modes.
- `domain/` (pure, no clock, database or other module): `report.go` (`ValidateReports`, `ValidCurrency`, `MaxAmount`), `price.go` (`PriceSet`, `Compute`, `ValidatePrice`, `ValidSourceURL`, the units), `booking.go` (`Book`), `sums.go` (`Valid`, `Sum`, `Breakdown`, `ShowAmounts`), `closing.go` (`CloseDecision`), `reconcile.go` (`Request`, `RequestEntry`, `Entry`, `Violation`, `UnmatchedReason`, `Share`; `Validate`, `InPeriod`, `Match`, `Split`; M2-28).
- `domain/startprices/`: the start file `start_prices.json` (embedded) and `Rows`, `Parse` (see "Start file").
- `app/`: `Service` implements both ports and `ApplyReconciliation`, the use case of the job (`apply.go`); `reconcile.go` and `estimates.go` are the two reconciliation operations; `Config` binds `billing.show_personal_estimates`, `api.page_size_default` and `api.page_size_max` (`wire.BillingConfig`); `ports.go` holds the outgoing `Repository` with `NewEntry`, `TotalsFilter`, `CurrencyTotal`, `EstimatesFilter` and `ErrPriceOverlap`, the `ReconcileScheduler` and the `Cursors`.
- `adapters/postgres/`: `queries/prices.sql`, `queries/cost_entries.sql`, `queries/reconcile.sql` (M2-28), the sqlc output in `sqlcgen/` (entry in `services/server/sqlc.yaml`) and `Repository`. Locks and writes run only inside a unit of work; SQLSTATE `23P01` and `23505` of `price_table` become `app.ErrPriceOverlap`; every other database error is internal and carries no SQL text or value.
- `adapters/httpapi/`: `Billing`, the strict-server handlers of `getThreadCosts`, `listPrices`, `createPrice`, `closePrice`, `reconcileCosts`, `submitCostReconciliation` and `listCostEstimates`; they convert and pass errors unchanged; `Cursors` is the cursor codec of the HTTP layer for `listCostEstimates`. The `Idempotency-Key` of the price changes and the reconciliations is handled by the middleware of the chain (M1-20).
- `adapters/jobs/`: the job `billing.reconcile` (`Reconcile{RunIDs}`, `Handler`, `Register` on the queue `default` without a periodic entry) and `Scheduler`, the `ReconcileScheduler` over `jobs.Inserter.InsertTx`, bound once the job runtime exists.
- `adapters/billingtest/`: test helpers, kept out of the binary (see "Tests").
- `internal/wire/module_billing.go` builds the module in the start step chat before runs (`startBilling`, `NewBilling` in `internal/wire/billing.go`), mounts its handlers, registers the job `billing.reconcile` in every role and binds its scheduler to the job runtime; `startRuns` hands `billing.Costs` to `runs`; `internal/wire/txn.go` subscribes `runs` to `billing.CostsReconciled`; `internal/wire/configkeys_billing.go` registers the key.

## Retention (M7-05)

`adapters/postgres.Retention` is the participant of billing in the job `retention.cost`:
`cost_entries` after `occurred_at` (newest first, so a correction goes before the entry it
supersedes) and closed `price_table` rows after `valid_to`, each plus `retention.cost`. An open
price row has no end time.

## Port (root package `internal/billing`)

- Inbound: `Costs` (posting, correction, totals, reconciliation, estimates), `Prices`.
- Emits in-tx events: `CostsReconciled` (the job `billing.reconcile`; `runs` subscribes, M2-28).
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

`ValidateReports` wraps the pure rules so `runs` checks a `finishRun` body before its unit of work without importing a foreign domain (02 5). `BookRun` and `Totals` never call `authorize`: `BookRun` runs inside the unit of work of `run.finish`, which `runs` authorized, and every caller of `Totals` (M6-05) authorizes first and passes the visible channels. `ThreadCosts`, `List`, `Create` and `Close` take the principal from the context (`kernel/actor`) and call `authorize` before anything else.

## Tables and migrations

- `migrations/00072_billing_price_table.sql`: `price_table` exactly as 03 4.9, the extension `btree_gist` and the exclusion constraint over `tstzrange(valid_from, valid_to)` per `provider`, `model` and `unit` (bounds `[)`, an absent `valid_to` is open), the unique key `(provider, model, unit, valid_from)` and the lookup index. `bothy_app` may read and insert and update only `valid_to` (D-M2-22-1).
- `migrations/00073_billing_cost_entries.sql`: `cost_entries` exactly as 03 4.9 with the checks of `billing_mode` and `source`, every foreign key `ON DELETE RESTRICT` (also `agent_session_id -> agent_sessions(id)`, left to this ticket by M2-12), the partial unique index on `supersedes_id` and the indexes per space, agent, thread, run and open estimate. `bothy_app` may read and insert only: entries are append only, a correction is a new row (M2-28 inserts its reconciled entries with these grants). M7-05 adds the grants it needs.

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
- `createPrice` (`price.manage`, owner with step-up, audited, `rl.admin`): authorize, `domain.ValidatePrice` (`source_url` an absolute `https` URL without user info or fragment, `retrieved_at` not after now; D-M2-22-16), `409 conflict` with reason `price_currency_mismatch` for an overlapping row of the same provider and model in another currency, insert, `409 conflict` with reason `price_overlap` on an exclusion or unique violation (D-M2-22-17).
- `closePrice` (`price.manage`): authorize, lock the row `FOR UPDATE` (`404` for an unknown id), `domain.CloseDecision` (end not after start `400`; a closed row with the same end `200` and an audit entry `no_change`; another end `409 conflict` with reason `price_closed`), `409 conflict` with reason `price_in_use` when the new end cuts off a `computed` booking that counted the row's unit, then set `valid_to` (D-M2-22-18). A closed row is never reopened or moved.
- Audit of both (allow entries in the transaction): action `price.manage`, resource `instance`, attributes `price_id`, `op`, `valid_from`, `valid_to` when set, `retrieved_at`, `source_url` (value kind `auditing.SourceURL`, D-M2-22-19) and `no_change`. Denials are audited by `authz`.

## Start file

`domain/startprices/start_prices.json` (schema `start_prices.v1`) holds the base prices of the five token units for the models whose identifiers the model overview of the provider's documentation names, with the pricing page as `source_url`, its time of reading as `retrieved_at` and `valid_from = retrieved_at` (D-M2-22-20). It is embedded and checked by tests; the server never loads it, because loading it on the host needs a system action that 05 does not have (spec gap). Until one exists the owner enters prices with `createPrice`. Booking tests never read it: they take their prices from `createPrice` or the test helper.

## Reconciliation (M2-28)

The reconciliation with the provider's invoice data runs as an app outside the server (`plugins/cost-reconciler`, ADR 0055): the admin key of the provider exists only there. The server has no configuration key, column, job argument, log field or code path for it (`TestServerConfig_HasNoReconcilerKeys`, `.semgrep/bothy-go-no-model-credentials.yaml`).

- **Operations:** `reconcileCosts` (`cost.reconcile`, owner in a session with a fresh step-up, audited, `rl.admin`; app, agent and personal tokens fail the requirement `Owner` with `403 forbidden`, reason `owner_only`) and `submitCostReconciliation` (`cost.reconcile.submit`, only an app with the granted scope `costs:reconcile`, no step-up, audited, `rl.admin`) both call `Costs.Reconcile` with their action. `listCostEstimates` (`cost.reconcile.read`, an owner in a session or the app, no step-up, no audit, `rl.read`) calls `Costs.Estimates`.
- **Request (one unit of work):** authorize first (authz audits a denial); `domain.Validate` (`invalid` at `/period_end` and at `/entries/<i>/run_id` of a repeated run, `too_large` at `/entries/<i>/amount_micros`; the schema checks a negative amount, the currency form and an empty provider); the exclusive advisory lock `pg_advisory_xact_lock(hashtextextended('billing.reconcile:' || run_id, 0))` per run in ascending order, one statement each; the valid entries of the runs in the half-open period in a new statement; `domain.Match` per entry in request order; for each matched run that is no no-op one reconciled successor per valid entry in the period with the share of `domain.Split`, the copies and tokens of the replaced entry, its `occurred_at`, the request entry's currency, `reconciled_at` from the clock and `supersedes_id`; the job `billing.reconcile` with the written runs ascending; the audit entry with the period, `entry_count`, `reconciled_count`, `unmatched_count` and `total_micros_<currency>` per currency (D-M2-28-3). Only inserts: `bothy_app` keeps `SELECT, INSERT` on `cost_entries`.
- **Job `billing.reconcile`** (queue `default`, never periodic, args the run IDs only): `ApplyReconciliation` takes the shared form of the same locks in ascending order, then the exclusive advisory lock `billing.session:<session_id>` of each agent session of these runs in ascending order (`BookRun` of a run with a session holds the shared form until its unit commits, so the job's absolute session total never misses a booking and a later booking adds to it), sums the valid entries of each run and of each agent session of these runs (all periods), leaves a run or session with mixed currencies unchanged (logged by ID) and publishes `CostsReconciled`. A repeated job finds the same state and changes nothing.
- **Estimates:** per run of the given agents with valid or replaced entries of mode `organization` in the range: provider, currency, `estimate_micros` (the estimate entries whose successor, if any, is not an estimate) and `reconciled` (every valid entry reconciled); runs with mixed currencies are left out; keyset by `run_id` with a cursor bound to the principal, the range and the agents.
- **Queries without a visible-channel predicate:** every query of `queries/reconcile.sql` is instance-level: the writes name runs after `authorize` allowed the action on the instance, the job reads the runs its request named, and the estimates read is evaluated on the instance (R-AZ-28, 07 7.4 rules 1 and 3) and names no channel, thread or content. `cost_entries` is no content table, so the gate `sql-visibility` does not list them.

## Losses and notes for later tickets

- **Server-ended runs book nothing** (D-M2-22-9): a run that ends `stale`, `interrupted`, `cancel_timeout`, `agent_disabled` or `access_revoked` without a later `finishRun` transition gets no entry, because `run.finish` is the only source of the usage per model (09 4.10). The consumption of such a run is lost; this is part of RK-16 (the estimate deviates from the invoice) until the reconciliation of M2-28 covers it.
- **No baseline** (D-M2-22-10): reported values are booked as the consumption of the run; `agent_sessions.cost_baseline_micros` is never subtracted (M8-06, RK-75).
- **For M2-28** (D-M2-22-11): a correction entry (`source = 'reconciled'`, `supersedes_id` on the estimate) repeats the token counts of the entry it supersedes, because the valid sum excludes superseded entries and token sums would otherwise drop. M2-28 needs no further grant (it only inserts) and sets `runs.cost_is_estimate` and `agent_sessions.cost_is_estimate` through its job.

## Tests

- `domain/domain_test.go`: `TestValidateReports`, `TestPriceSet`, `TestCompute`, `TestAmountProperty` (against a `math/big` reference with one rounding), `TestBook`, `TestValidatePrice`, `TestCloseDecision`, `TestSums`, `TestDisplayRule`.
- `domain/startprices/startprices_test.go`: `TestStartPrices_EveryRowHasSourceAndRetrievedAt`, `TestStartPrices_Valid`, `TestStartPrices_UnknownMemberRejected`.
- `migrations/billing_test.go`: `TestMigration_PriceTable`, `TestMigration_CostEntries` (columns, checks, keys, indexes, `btree_gist`, the exclusion constraint and the grants).
- `test/integration/costs/`: the cross-module tests over PostgreSQL 18 through `internal/wire` and the real HTTP chain (booking in `finishRun`, repeats, the cost rules, two currencies, the session sum, recomputation and closing races, `getThreadCosts` per mode and rights, the price operations).
- `domain/reconcile_test.go`: `TestValidateReconcile`, `TestMatch`, `TestSplit`, `TestSplitProperty` (M2-28); `adapters/jobs`: `TestReconcileJob`.
- `test/reconcile/` (M2-28): the integration cases of the reconciliation over the real server and the end-to-end test with the app (see its README).
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
| `billing.reconcile` | `cost.reconcile` or `cost.reconcile.submit`; never periodic | `default` |
| `retention.cost` (participant, M7-05) | periodic every `retention.job_interval` | `maintenance` |

## Notes for the build

- Amounts in micro-units with a currency; corrections only as a new row with `supersedes_id` (03, 4.9).
- With `billing_mode = personal`, amounts are for information only.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)

## Statistics source (M6-05)

`billing.StatsSource` (`stats.go`, implemented by `app.Stats` over
`adapters/postgres/queries/stats.sql`) serves `getCostStats`: `CostStats` sums the valid entries
(without a successor) of the visible channels per bucket of `occurred_at`, group (space, agent or
none), billing mode and currency, and `CostCurrencies` names the currencies of a range. Entries of
mode `personal` count only when `billing.show_personal_estimates` is on (10 12.2).
