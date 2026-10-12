# M2-22: Costs: booking, price table and display

Status: ready | Depends on: M2-12 | Milestone: M2

## Goal

After this ticket the server books the consumption an agent adapter reports when it ends a run: `finishRun` with `costs` writes one `cost_entries` row per model, priced from the price table as `computed` when the price set for provider, model and booking time covers every consumed unit, otherwise `adapter_reported` with the adapter's amount, otherwise `unpriced` with amount `0` ("Price missing"); every row is an estimate, carries the actual billing mode of the run and copies of agent, thread, channel, space and session, and amounts of different currencies are never converted. The owner maintains the price table with `listPrices`, `createPrice` and `closePrice` (owner, step-up and audit for changes, source and retrieval time on every row, no overlapping validity, a used row never changes except that it can be closed, and closing never breaks a booking), and the product ships a start file with prices, source and retrieval date that a test checks; the owner enters prices with `createPrice` (loading the file on the host needs a system action that 05 does not have, spec gap). Every reader of a thread gets its costs with `getThreadCosts`: per run and per currency, summed per billing mode and currency without conversion, with token counts always and with amounts of runs on a personal subscription only when `billing.show_personal_estimates` is on. The `billing` port offers the same valid sums per run, thread, session, agent, space and instance for the statistics of M6-05. This is BR-09 and the server part of BR-11 (10 "Tests"), the ruling D-BR-01 and the estimate marking that RK-16 relies on; replacing estimates by reconciled amounts stays with M2-28.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope: "10 with ... costs and reconciliation"; Out of scope; Exit criteria items 2 and 10; Test strategy: fake model (no costs); Red team first paragraph; Risks: "the cost estimate differs from the invoice (label 'Estimate' in M2-22, reconciliation in M2-28)"), block "agents and extensions" rows M2-09 (what exists), M2-10, M2-12 (sessions, see Boundaries), M2-22 (this ticket), M2-28 (reconciliation), M2-33, M2-34 (adapters that report costs); section 12 row M6-05 (`getCostStats` on top of this port); section 10 rows M4-21 and M4-28 (client display); section 14 row M8-06 (cost baseline of imported sessions).
- Dependencies: the header names M2-12 (D-W-103: the booking adds to the session sum and needs the FK of `cost_entries.agent_session_id` on `agent_sessions`). Through it this ticket also builds on M2-09, M2-08, M2-07, M2-06, M2-05, M2-04 and M2-02 and on the M1 tickets M1-07 and M1-08 (configuration schema and reference), M1-10 (meter provider), M1-12 (migrations, `pgtest`, `make migrations-lock`), M1-13 (unit of work), M1-16 (event log), M1-17 (`api/operations.yaml`, strict server), M1-18 (HTTP chain, problem details, `errors[].code` values), M1-19 (rate limits), M1-20 (`Idempotency-Key`), M1-24 (`auditing.Recorder`), M1-25 (registration duty), M1-26 (principals, `kernel/textcheck`), M1-28 (`authorize`, requirement table), M1-31 (owners), M1-35 (step-up), M1-44 (agent API tokens), M1-51 (agents), M1-52 to M1-56 (spaces, channels, operators, threads, messages); all implied, so not in the header.
- `docs/spec/10-backend-policy.md`: 8 "Expected and reported" (row `cost_entries.billing_mode`), 12 "Costs" with 12.1 "Recording", 12.2 "Display", 12.3 "Price table" and 12.4 "Reconciliation with billing" (only to know what M2-28 adds), 14 "Audit trail" (row "Prices"), 15 "Tests" (BR-09, BR-10, BR-11), 17 "Configuration values of this chapter" (`billing.show_personal_estimates`).
- `docs/spec/03-data-model.md` (core contract, read only): 1 "Principles" points 1, 4, 10, 11, 12 and 13; 4.9 "Costs (`billing`)" with `price_table` and `cost_entries` (every column, check, FK, unique, exclusion constraint, index, the paragraphs after the tables); `runs` (`billing_mode_used`, `cost_micros`, `currency`, `cost_is_estimate`, `agent_session_id`, `finished_at`); `agent_sessions` (`cumulative_cost_micros`, `cost_baseline_micros`, `currency`, `cost_is_estimate`, `version`); the sentence "`cost_entries` and `event_log` keep the channel at the time of the booking" in the paragraph on moving threads after `threads`; 6.3 "Lock order"; 8 "Decisions of this chapter" R-DM-04 and R-DM-23.
- `docs/spec/09-agents.md`: 4.7 "Adapter operations" (row `run.finish`), 4.10 "Usage at the end of a run", 5.4 "Live box content" (row "Costs": only for `organization`), 9.3 "Previous sessions", 18.7 "Costs" (the runner reports the difference per run as an estimate).
- `docs/spec/05-actions.md` (core contract, read only): rules 1.2 and 1.10 (system actions); 2.3 "Effective level" point 1 (an owner's personal token has no owner rights); 2.4 (Step-up, Audit); 2.5 "Decision"; 2.6 (no scope for `cost.read.thread`); 2.7 (`cost.read.thread` has effect `read`); table 4.12 rows `cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile`, `cost.reconcile.submit`; R-AZ-01, R-AZ-03, R-AZ-15.
- `docs/spec/06-api.md`: 2 "Basic rules" (rows "No effect", "Integers", "Money", "No `null`", "Unknown values"), 5 "Errors" (points 3, 4 and 7, `errors[].code`), 6 "Pagination" point 6 (unpaginated small lists), 7 "Idempotency", 13 "Versioning, deprecation, compatibility" point 1 (`0.x` until the first release), 16 "Endpoints" group "Stats".
- `docs/spec/07-permissions.md`: 2.10 "Apps"; 4.1 "Decisions and reasons" (`owner_only`, `step_up_not_possible`, `subject_kind_not_allowed`); the world of 11.1 and its row AZ-M34; test rows AZ-P1 and AZ-P4 in 11 "Tests".
- `docs/spec/02-architecture.md`: 3.3 "Commands of the binary" (the command register); 4.2 "Module list" and 4.3 "Ports, events and sources per module" (row `billing`: `Costs` with booking, correction and sums, `Prices`); 4.4 "Allowed dependencies" (`billing` uses nothing; `runs` may use `billing` and `agents`); 4.6 "Actions per module" (row `billing`); 5 "Layers within a module" (rule 5 on metrics); 6 "Code layout"; 8.1 "Flow", 8.2 "Lock order"; 9.2 "404 instead of 403".
- `docs/spec/19-security.md`: 6.1 SEC-IN-1 and SEC-IN-4; 12 SEC-AUD-5 (schema `audit_details.v1`, and its sentence that a ticket needing another free-text form extends the schema).
- `docs/spec/20-privacy.md`: 2 rows `price_table` and `cost_entries` (class `cost`); 9 "Codetermination" row "Costs per person" (in threads only costs per run, never per person).
- `docs/api/openapi.yaml`: operations `getThreadCosts`, `listPrices`, `createPrice`, `closePrice`, `finishRun`; schemas `CostBreakdown`, `RunCost`, `Money`, `BillingMode`, `RunFinish`, `CostReport`, `Price`, `PriceCreate`, `PriceClose`, `PriceList`, `PriceUnit`, `Run`, `RunSummary`, `AgentSession`, `RunStateChangedPayload`, `Problem` (`x-problem-codes`); parameters `ThreadIdPath`, `PriceIdPath`, `IdempotencyKey`; responses `Conflict`, `UnprocessableEntity`.
- ADRs 0008, 0013, 0015, 0017, 0021, 0024, 0049, 0050, 0053, 0055.
- `docs/DECISIONS.md`: D-BR-01 (booking order, no conversion, no amounts for `personal` in the interface), D-BR-02, D-K-D25 to D-K-D28, D-W-100, D-W-103; section 10 for the format of new rows and the rows `D-M2-09-<n>` (the `not_enabled` answer for `costs` that this ticket replaces).
- `docs/RISKS.md`: RK-16 (estimate deviates from the invoice), RK-75 (double booking of imported sessions, owned by M8-06). `docs/security/threat-model.md`: T-AI-07 and T-AI-08 (consumption and abuse of a personal subscription). No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- `docs/observability/signals.md`: 2 "Cardinality"; 4.3 "Agents and runs" row `bothy.run.cost` (attributes `bothy.agent.handle`, `bothy.billing_mode`, `bothy.currency`, `bothy.cost.estimate`).
- `docs/operations/configuration.md`: `billing.show_personal_estimates`, `retention.cost`, `database.tx_retry_max`, `ratelimit.rl.read.per_minute`, `ratelimit.rl.admin.per_minute`, `auth.step_up_max_age`, `audit.deny_aggregate_window`.
- Briefs: M1-12, M1-13, M1-17, M1-18 (`StageValidate` and its `errors[].code` list), M1-20, M1-24 (`Entry`, `Attrs`, value kinds), M1-26 (`textcheck.DisplayText`), M1-28 (requirement table: `read` gives `Level(read)`, `owner` gives `Owner`; app scope table), M1-55 (`chat.Threads.Info`, `ThreadInfo`), and the M2 briefs `M2-05-runs-state-machine.md` (runs migration, `Run` representation, metrics port of `runs`), `M2-09-run-lifecycle.md` (Design note 6: `finishRun`, the end-of-run function, the `not_enabled` answer for `costs`), `M2-10-run-document-live-box.md` (Design note 12: the `/cost` patch and "the rule of the clients and of M2-22"), `M2-12-agent-sessions.md` (Out of scope and Design notes 3 and 14: booking into `cumulative_cost_micros` and the FK of `cost_entries.agent_session_id` are left to this ticket).
- Code on the base branch: `internal/billing` (README and `doc.go` only), `internal/runs` (M2-05 to M2-09: `Finish`, `FinishRun`, the end-of-run function, the metrics port `Metrics`, `adapters/otelmetrics`), `internal/agents` (M1-51, M2-07, M2-12: `Sessions`), `internal/chat` (`Threads.Info`), `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/audit` (details validation), `internal/kernel/txn`, `internal/kernel/clock`, `internal/kernel/errs`, `internal/kernel/textcheck`, `internal/wire` (`configkeys_*.go`, `configchapters.go`), `cmd/bothy`, `architecture/modules.yaml`, `sqlc.yaml`, `api/operations.yaml`, `migrations/`. Merged M1 code can be read with `git show main:<path>`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Note on rule 8: the start file (Design note 13) is product data, not an example. It names provider and models exactly as the pricing page cited in 10 12.3 and the runtimes do; every test fixture still uses invented names (`example`, `model-large`) and `example.com`.

Product requirements implemented here:

11. At the end of a run the server writes one `cost_entries` row per model of `RunFinish.costs`: `computed` when `price_table` has the price for `provider` and `model` at `occurred_at`, else `adapter_reported` with the adapter's amount, else `unpriced` with amount `0` (shown as "Price missing"); `computed` has precedence over `adapter_reported` (10 12.1, 09 4.10, 03 `cost_entries`, BR-09, D-BR-01).
12. A `computed` booking refers to the price set for `provider`, `model` and `occurred_at`, never to a single row; a row used by a booking never changes again except that it can be closed with `valid_to`, so every `computed` booking stays recomputable (03 `price_table`, R-DM-23, 10 12.1).
13. Every entry is an estimate (`is_estimate = true`) until a reconciliation replaces it; amounts of different currencies are never converted and sums stand per currency; the valid sum counts only entries that no other entry supersedes (10 12.1, 03 `cost_entries`).
14. `cost_entries.billing_mode` is the actual mode of the run (`runs.billing_mode_used`); `agent_id`, `thread_id`, `channel_id`, `space_id` and `agent_session_id` are copies taken at booking time (10 8, 03 `cost_entries` and the paragraph on moving threads).
15. Amounts are shown only in mode `organization`; in mode `personal` the reader gets the mode (the badge "Personal") and the token counts, no amounts, unless `billing.show_personal_estimates` is on (10 12.2, BR-11, D-BR-01).
16. `cost.read.thread` needs `read` on the thread, writes no audit entry and uses `rl.read`; without read right the answer is the same `404` as for an unknown thread (05 4.12 and rule 1.2, R-AZ-01, AZ-P4).
17. `price.read` is owner only, without step-up and audit, `rl.read`; `price.manage` is owner only with step-up, audited, `rl.admin`, with `Idempotency-Key`; step-up actions are refused for tokens (05 4.12 and 2.4, R-AZ-03).
18. Every price row has `source_url` and `retrieved_at`; validities of one provider, model and unit never overlap (exclusion constraint with the extension `btree_gist`) (03 `price_table`, 10 12.3).
19. The product ships a start file with prices, source and retrieval date, and a test checks that every row has source and retrieval date; the specification itself names no prices (10 12.3, PLAN row M2-22).
20. A price change is audited with row, source and retrieval time, and audit details keep the forms of `audit_details.v1` (10 14, 19 SEC-AUD-5).
21. Money is `amount_micros` (int64) plus an ISO 4217 `currency`; integers stay below the bound of 06 "Basic rules" (row "Integers"); no JSON `null` (03 principle 4, R-DM-04, 06 2, ADR 0053).
22. `bothy.run.cost` is counted from the booked entries with exactly the attributes of signals.md 4.3 (signals.md, 02 5 rule 5).

## Scope

All paths under `services/server/` unless they start with `docs/`. Names of M2-05 to M2-09 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `migrations/<next number>_billing_price_table.sql` and `migrations/<next number + 1>_billing_cost_entries.sql` (plan: migrations `price_table`, `cost_entries`; Design note 2).
- Port `internal/billing` (next to the existing `doc.go`): interfaces `Costs` and `Prices` and their DTOs (Design notes 1, 5, 7, 8, 10 to 13).
- `internal/billing/domain/report.go` (`ValidateReports`), `price.go` (`ValidatePrice`, `PriceSet`, `Compute`), `booking.go` (`Book`), `sums.go` (valid sums, display rule), `closing.go` (`CloseDecision`), all pure, with tests.
- `internal/billing/domain/startprices/`: `start_prices.json` (embedded with `//go:embed`) and `startprices.go` (`Rows() ([]PriceInput, error)`), with tests (Design note 13).
- `internal/billing/app/costs.go` (`ValidateReports`, `BookRun`, `ThreadCosts`, `Totals`), `prices.go` (`List`, `Create`, `Close`), `ports.go` (repository, clock and configuration interfaces).
- `internal/billing/adapters/postgres/queries/prices.sql`, `queries/cost_entries.sql`, the sqlc output and the repository.
- `internal/billing/adapters/httpapi/`: strict-server methods `GetThreadCosts`, `ListPrices`, `CreatePrice`, `ClosePrice`.
- `internal/billing/adapters/billingtest/` (kept out of the binary like M1-33's `identitytest`): `InsertEntry` (any source, also `reconciled` with `supersedes_id`, for tests of the valid sum) and `RecomputeComputed(ctx) ([]Mismatch, error)` (Design note 4).
- `internal/wire/configkeys_billing.go`: the key `billing.show_personal_estimates` (chapter constant `chapter10` in `configchapters.go` if absent; a parallel ticket adds the same constant with the same name and text).
- `test/integration/costs/`: the cross-module tests of this ticket.

Change:

- `internal/runs` (port, app, adapters): `FinishRun` carries the reports (replace the flag `CostsPresent` of M2-09 by `Costs []CostReport`, a DTO of the `runs` port with the fields of `billing.CostReport`, because a port imports only `kernel` (02 5); the app maps it to `billing.CostReport`); the finish use case books inside its unit of work (Design note 5); the `runs` metrics port gets `CostBooked` and its OpenTelemetry adapter records `bothy.run.cost` (Design note 5 h); `adapters/httpapi` maps `RunFinish.costs` and drops the `not_enabled` answer of M2-09.
- `internal/agents` (port, app, postgres): `Sessions.AddCost(ctx context.Context, cmd AddSessionCost) error` with `AddSessionCost{SessionID ids.ID; AmountMicros int64; Currency string}` (Design note 5 g).
- `internal/kernel/auditing` and `internal/audit/domain`: the value kind `SourceURL` (Design note 14).
- `internal/authz`: only the test `TestCostReadThreadRequirement` (Design note 16, Tests), no code change.
- `internal/wire`: the billing handlers in the embedded strict server, the billing port into `runs`, the config key.
- `api/operations.yaml`: move `getThreadCosts`, `listPrices`, `createPrice`, `closePrice` from `later` to `enabled`; rerun the route generator (M1-17).
- `sqlc.yaml`: entry for `billing`. `architecture/modules.yaml`: the vendor groups the new `billing` adapters need in `adapter_vendors` (as merged modules do for postgres adapters), `depends_on` of `billing` stays empty; regenerate `.go-arch-lint.yml` with `go generate ./tools/archgen`.
- `docs/api/openapi.yaml` (Design note 8 and 9): schemas `CostBreakdown`, `RunCost`, new `CostTotal` and `TokenCounts`, the example of `getThreadCosts`, the descriptions of `RunFinish.costs`, `CostReport`, `PriceCreate`, `PriceClose`, `listPrices` and `getThreadCosts`. Then `make api-lint`, `make api-checks`, `make api-tables` and the server code generation.
- `docs/spec/10-backend-policy.md` (English, like the chapter): 12.1 the booking details of Design notes 4 to 6; 12.2 the server side of the display rule (Design notes 8 and 9); 12.3 the start file (embedded, checked by a test, not loaded by the server), the rules of `createPrice` and `closePrice` (Design notes 11 to 13).
- `docs/spec/09-agents.md` 4.10 (English): the field is `costs` (`CostReport`), `currency` is required, and the values are the consumption of this run (Design note 6).
- `docs/spec/19-security.md` SEC-AUD-5 (English): the member `source_url` (Design note 14).
- `docs/spec/06-api.md` 6 point 6: the price table is one of the small unpaginated lists (Design note 10).
- `docs/operations/configuration.md`: regenerate with `make config-reference` if the schema text changes.
- READMEs of `billing`, `runs`, `audit` and `agents`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M2-22-<n>` for every Ruling below, and in the M2-09 row about `not_enabled` for `costs` the note that `D-M2-22-<n>` replaces it.

This ticket creates no new top-level make target, no event type, no job, no action and no problem code.

## Out of scope

- Reconciliation: `reconcileCosts`, `submitCostReconciliation`, the estimate list `listCostEstimates` (`cost.reconcile.read`, R-AZ-28), the job `billing.reconcile`, entries with `source = 'reconciled'` from a real path, setting `runs.cost_is_estimate` and `agent_sessions.cost_is_estimate` to `false`, BR-10 (M2-28). This ticket only reads such entries correctly (valid sum).
- The aggregate cost API `getCostStats` with `cost.read.aggregate`, cost members in statistics, live frames and the backend view (M6-05, M6-14); they use the port sums of Design note 7.
- The client display: badges "Personal" and "Organization", the labels "Estimate", "Price missing" and "Information, not billed" (M4-21, M4-28; console in M6).
- Subtracting `agent_sessions.cost_baseline_micros` for runtimes that report session sums (M8-06, RK-75; Design note 6, spec gap).
- The table `agent_sessions`, `runs.agent_session_id` and binding sessions (M2-12); this ticket adds to the session sum (Design note 5 g).
- Live cost values during a run through the `/cost` patch (M2-10); cost limits per agent (not specified, spec gap).
- The retention job `retention.cost` (M7-05); `bothy.run.tokens` (no owning ticket, plan gap).
- How adapters compute what they report: SDK (M2-29, M2-30), Hermes Agent adapter (M2-33), Claude-Code-Runner (M2-34).
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M2-22-costs`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(billing): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules, and plain reads without account of the public pricing page cited in 10 12.3 and, only if that page shows display names instead of model identifiers, the model overview of the same documentation site, to fill the start file (Design note 13).
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for M2-05 to M2-09: their merged names win over the expected names here (for example `FinishRun`, the end-of-run function, the metrics port of `runs`); list each difference.
- M2-12 is merged on the base branch (header, D-W-103): `agent_sessions`, `runs.agent_session_id` and the port `agents.Sessions` exist; otherwise stop with `Verdict: BLOCKED dependency M2-12 missing`.
- No real agent runtime, no model call and no third-party service in tests: the adapter in the tests is plain Go HTTP code with an agent API token; booking tests take their prices from `createPrice` or the test helper, never from the start file (only the start file tests read it).
- Never write a price from memory or from another source into the start file; if the pricing page cannot be read in the build environment, stop with `Verdict: BLOCKED start file: pricing page not readable`. The specification and the docs name no prices (10 12.3).
- Time only through `kernel/clock`; tests move a fake clock and never sleep.

## Design notes

1. **Module shape** (02 4.2 to 4.6, 5). `billing` owns `price_table` and `cost_entries`, uses no other module port (02 4.4) and gets everything it copies from its caller. Port `internal/billing`:

    ```go
    // Costs books run costs and reads valid sums (02 4.3: booking, correction, sums).
    type Costs interface {
        ValidateReports(reports []CostReport) error
        BookRun(ctx context.Context, cmd BookRun) (Booking, error)
        ThreadCosts(ctx context.Context, threadID ids.ID) (ThreadCosts, error)
        Totals(ctx context.Context, q TotalsQuery) ([]Total, error)
    }
    // Prices maintains the price table (10 12.3).
    type Prices interface {
        List(ctx context.Context) ([]Price, error)
        Create(ctx context.Context, cmd CreatePrice) (Price, error)
        Close(ctx context.Context, cmd ClosePrice) (Price, error)
    }
    type CostReport struct {
        Provider, Model string
        TokensIn, TokensOut, TokensCacheRead, TokensCacheWrite, TokensCacheWrite1h int64
        AmountMicros *int64 // absent lets the server compute (CostReport.amount_micros)
        Currency string
    }
    type BookRun struct {
        RunID, AgentID, ThreadID, ChannelID ids.ID
        SpaceID, AgentSessionID ids.ID // zero value: none (DM channel, no session)
        BillingMode string              // runs.billing_mode_used
        OccurredAt  time.Time
        Reports     []CostReport
    }
    ```

    `Booking{Entries []Entry; Sums []CurrencySum}` with `Entries` in the order of `Reports` and `CurrencySum{Currency string; AmountMicros int64}` in the order in which a currency first appears in `Entries`. `ValidateReports` wraps the pure `domain.ValidateReports` of note 3, so `runs` can check a request before its unit of work without importing a foreign domain (02 5). `ids.ID` stands for the merged id type of `kernel/ids`. `ThreadCosts`, `List`, `Create` and `Close` serve the own `httpapi` adapter: each use case takes the request principal from the context (`kernel/actor`) and calls `authorize` before anything else (notes 8, 10 to 12). `BookRun` and `Totals` never call `authorize`: `BookRun` runs inside the unit of work of `run.finish`, which `runs` authorized, and every caller of `Totals` (M6-05) authorizes first and passes the visible channels.

2. **Migrations** exactly as 03 4.9, nothing more:
    a. `price_table`: every column with type, null and default of 03; `CHECK (unit IN ('input_mtok','output_mtok','cache_read_mtok','cache_write_mtok','cache_write_1h_mtok','request'))`; FK `created_by -> principals(id) ON DELETE RESTRICT`; unique `(provider, model, unit, valid_from)`; `CREATE EXTENSION IF NOT EXISTS btree_gist` (PostgreSQL marks it trusted, so the database owner role that runs migrations in M1-12 can create it; the migration test proves this, and if it fails stop with `Verdict: BLOCKED` and the error instead of widening a role) and `EXCLUDE USING gist (provider WITH =, model WITH =, unit WITH =, tstzrange(valid_from, valid_to) WITH &&)`; index `(provider, model, unit, valid_from DESC)`. `tstzrange` has the bounds `[)` and an absent `valid_to` is open, which is the validity rule of Design note 4.
    b. `cost_entries`: every column of 03 with `CHECK (billing_mode IN ('personal','organization'))` and `CHECK (source IN ('adapter_reported','computed','unpriced','reconciled'))`; FKs `run_id -> runs(id)`, `agent_id -> agents(principal_id)`, `thread_id -> threads(id)`, `channel_id -> channels(id)`, `space_id -> spaces(id)`, `supersedes_id -> cost_entries(id)`, all `ON DELETE RESTRICT`; `agent_session_id -> agent_sessions(id) ON DELETE RESTRICT`; unique partial index `(supersedes_id) WHERE supersedes_id IS NOT NULL`; indexes `(space_id, occurred_at)`, `(agent_id, occurred_at)`, `(thread_id)`, `(run_id)`, `(is_estimate, occurred_at) WHERE is_estimate`.
    c. Grants (M1-12 D4, least privilege): `price_table` `SELECT, INSERT` and `UPDATE (valid_to)` only; `cost_entries` `SELECT, INSERT` only. Ruling for docs/DECISIONS.md: the column grant makes R-DM-23 hold in the database (only the end of a row can change), and entries are append only because a correction is a new row (03); M2-28 and M7-05 add the grants they need.
    d. Numbering by M1-12 D6; renumber before the merge if another ticket took the number.

3. **Validation of reports** (`domain.ValidateReports`, pure; pointers relative to the list, `runs` prefixes `/costs`; codes from the list of M1-18): `provider` and `model` non-empty after trimming and passing `textcheck.DisplayText` (`required`, `invalid`); every token field and `amount_micros` not negative (`too_small`) and below the integer bound of 06 "Basic rules" (`too_large`); `currency` matches the ISO 4217 form `^[A-Z]{3}$` (`invalid_format`); a second report with the same `provider` and `model` (`invalid`, pointer to the second). Absent token fields count `0`. Ruling for docs/DECISIONS.md: the duplicate rule, because one entry per model is what 10 12.1 books, and the form check of the currency, because 03 principle 4 names ISO 4217 and `bothy.currency` must stay a bounded attribute (signals.md 2).

4. **Price set and computation** (10 12.1 and 12.3, 03 `price_table`, R-DM-23):
    a. The price set `S(provider, model, T)` is every row with equal `provider` and `model` (exact, case-sensitive match) and `valid_from <= T < valid_to` (absent `valid_to` is open). Units map to counts: `input_mtok` to `tokens_in`, `output_mtok` to `tokens_out`, `cache_read_mtok` to `tokens_cache_read`, `cache_write_mtok` to `tokens_cache_write` (5 minutes or no stated duration), `cache_write_1h_mtok` to `tokens_cache_write_1h`. A unit is needed when its count is above `0`. `request` is never used, because neither `CostReport` nor `cost_entries` carry a request count (spec gap).
    b. `computed` when `S` is not empty, all rows of `S` have one currency, and every needed unit has its row in `S`; the entry's currency is that currency. Otherwise `adapter_reported` when the report has `amount_micros` (amount and currency of the report), otherwise `unpriced` with `amount_micros = 0` and the report's currency. The adapter's amount is ignored when the entry is `computed` (10 12.1).
    c. Amount: the sum over the needed units of count times `price_micros`, divided once by one million (prices are per million tokens, unit `*_mtok`), computed exactly with integers (`math/big` or 128-bit arithmetic from `math/bits`) and rounded half up once per entry; a result at or above the integer bound of 06 "Basic rules" is a `400 validation_failed` (`too_large`) for the report. Ruling for docs/DECISIONS.md: completeness, one currency, the unused `request` unit, exact arithmetic and one rounding per entry.
    d. `billingtest.RecomputeComputed` recomputes every `computed` entry from `S(provider, model, occurred_at)` with the same function and reports differences; tests run it after every price change (R-DM-23).
    e. A booking is never recomputed after it is written: an entry booked as `adapter_reported` or `unpriced` stays so when a price for its time is added later. Ruling for docs/DECISIONS.md.

5. **Booking in `finishRun`** (09 4.7 row `run.finish`, 4.10; 10 12.1; M2-09 Design note 6), one unit of work, in this order:
    a. Before the unit of work: `billing.Costs.ValidateReports` on `RunFinish.costs`; an error is `400 validation_failed` with pointers `/costs/<i>/<field>` and changes nothing (06 5 point 4).
    b. In the unit of work, after the run row is locked `FOR UPDATE` and the finish decision of M2-09 is known: a repeat of the same final state books nothing (M2-09 6 b, 06 2 "No effect"), so a retried `finishRun` never books twice. Only a real transition with a non-empty `costs` books. A run without `billing_mode_used` (it never started) and a non-empty `costs` gives `400 validation_failed` with `errors[].code` `invalid` and pointer `/costs`, and the unit rolls back. Ruling for docs/DECISIONS.md: a runtime starts only after `run.start` (09 4.9), so a run that never started consumed nothing, and `cost_entries.billing_mode` needs the actual mode (10 8).
    c. One clock reading `now` is both `runs.finished_at` and `occurred_at` of every entry. Ruling for docs/DECISIONS.md: 10 12.1 books "at the end of a run", and `occurred_at` is the time whose prices apply.
    d. Copies: `agent_id = runs.agent_id`, `thread_id = runs.thread_id`, `channel_id` and `space_id` from `chat.Threads.Info` at this moment (space absent for a DM channel), `agent_session_id = runs.agent_session_id` when set, `billing_mode = runs.billing_mode_used` (10 8, 03).
    e. `billing.Costs.BookRun`: for each report in order, load `S` with `SELECT ... FOR SHARE` (rows in ascending `id`, 03 6.3), apply note 4, insert the entry with a new UUIDv7, `is_estimate = true`, `reconciled_at` and `supersedes_id` absent. Lock order of the whole unit: run row, price rows, session row, then the counters of the unit of work (03 6.3, 02 8.2); deadlocks are retried by the unit of work (`database.tx_retry_max`).
    f. Run columns: `cost_micros` is the sum of the new entries in one currency, `currency` that currency, `cost_is_estimate = true`, written by the end-of-run function of M2-09 in the same update as the transition, so `version` rises once and the one `run.state_changed` carries the booked cost (09 5.4: the collapsed row is built from this event). When the entries have one currency that is the currency; with several, the currency of the first entry (entries follow the order of `costs`, and a `computed` entry has the currency of its price set), and the other currencies stay only in `cost_entries`. Ruling for docs/DECISIONS.md (spec gap: 03 `runs.currency` holds one currency). A finish without `costs`, or with an empty list, books nothing and leaves the cost columns as the live `/cost` patches of M2-10 left them.
    g. Session sum, when the run has a session: `agents.Sessions.AddCost` per currency of the booking, inside the same unit of work: lock the session row `FOR UPDATE`; when `cumulative_cost_micros` and `cost_baseline_micros` are both `0`, set `currency` to the booking's currency first; add the amount only when the currencies are equal, otherwise skip it (no conversion, 10 12.1); raise `version` by one (03 principle 10); write no `agent_session.changed`, because 05 6 names its causes and a booking is none of them. Ruling for docs/DECISIONS.md.
    h. Metrics after the unit of work committed, never inside (a retried unit would count twice, M2-05 note 11): `bothy.run.cost` per entry, value `amount_micros` divided by one million (unit `{currency_unit}`), attributes `bothy.agent.handle` (from the `agents` port as for the other run metrics), `bothy.billing_mode`, `bothy.currency`, `bothy.cost.estimate` (`true`). Use the registry of M1-68 when it is on the base branch, else the meter provider of M1-10 through `internal/runs/adapters/otelmetrics` (as M2-05 note 11).
    i. Runs that the server ends (`stale`, `interrupted`, `cancel_timeout`, `agent_disabled`, `access_revoked` without a later `finishRun` transition) get no entry, because 09 4.10 names `run.finish` as the only source of per-model usage; a later `finishRun` with the same final state changes nothing (note b). Ruling for docs/DECISIONS.md; the loss is part of RK-16 and named in the billing README.

6. **Per-run consumption and the cost baseline.** `CostReport` is "Cost of one model in this run" (openapi), 09 4.10 books the usage of the run, and the runner reports the difference per run (09 18.7); the server books the values as given. 03 `agent_sessions.cost_baseline_micros` says the server books only the part above the baseline when a runtime reports session sums, but the API has no way to mark a report as a session sum and the adapter never sees the baseline (`AgentSession` omits it, M2-12 note 4). This ticket therefore never subtracts the baseline; M8-06 owns the cost baseline of imported sessions (PLAN row M8-06, RK-75) and the spec gap is in the report. Ruling for docs/DECISIONS.md; update 09 4.10 (Scope).

7. **Valid sums and levels** (10 12.1, 03 `cost_entries`): an entry is valid when no entry has `supersedes_id` equal to its `id`. Sums group valid entries by `billing_mode` and `currency`; `amount_micros` is the sum, `is_estimate` is true when any summed entry is an estimate, token counts are summed per field, never converted. `Totals(ctx, TotalsQuery)` with `TotalsQuery{Level Level; ID ids.ID; From, To time.Time; Channels ChannelFilter; ForDisplay bool}`: levels `run` (`run_id`), `thread` (`thread_id`), `session` (`agent_session_id`), `agent` (`agent_id`), `space` (`space_id`), `instance` (all); `From` and `To` a half-open range on `occurred_at` (zero means open); `ChannelFilter{All bool; IDs []ids.ID}` restricts to the visible channels of the caller (`channel_id = ANY`), and a filter that is neither `All` nor has IDs returns nothing (fail closed, 03 principle 12); with `ForDisplay` the display rule of note 8 d hides amounts. `Total{BillingMode, Currency string; AmountMicros *int64; IsEstimate bool; Tokens TokenCounts; Entries int64}`, ordered by billing mode, then currency. Levels use the copies at booking time; only the thread level follows the thread itself. Ruling for docs/DECISIONS.md: a correction entry repeats the token counts of the entry it supersedes, so token sums do not drop after a reconciliation (state it in the billing README for M2-28).

8. **`getThreadCosts`** (05 4.12 `cost.read.thread`, `rl.read`, no audit):
    a. `authorize(actor, cost.read.thread, {kind: thread, id})`: no read right or an unknown thread gives `not_found` with the same status, body and headers (AZ-P4); agents with read through the operator rule are allowed; an app with read on the thread gets `forbidden` `subject_kind_not_allowed` from the scope table of M1-28 (05 2.6 has no scope for this action); an app without read gets the same `not_found` as an unknown thread, because visibility is checked first (07 2.10, AZ-M34).
    b. Read the valid entries with `thread_id = $1` (index `(thread_id)`), whatever channel they were booked in, and group them per run and per currency.
    c. Representation (openapi change in the same commit, owned by this ticket because only `getThreadCosts` uses these schemas; the API is `0.x` until the first release, 06 13 point 1):

    ```yaml
    CostBreakdown:  required [thread_id, totals, runs]
      totals: array of CostTotal   # valid sums per billing mode and currency, display rule applied
      runs:   array of RunCost     # runs of the thread with at least one valid entry, by run_id ascending
    CostTotal:      required [billing_mode, amount_micros, currency, is_estimate]
    RunCost:        required [run_id, billing_mode, tokens, unpriced]
      agent_session_id: Uuid       # absent when the entries have no session
      tokens: TokenCounts          # always, also for personal
      costs: array of Money        # one per currency; absent when amounts are hidden
      unpriced: boolean            # a valid entry of the run has source unpriced ("Price missing")
    TokenCounts:    required [tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, tokens_cache_write_1h], int64
    ```

    `total` (one `Money`) and `RunCost.cost` are replaced, because 10 12.1 requires sums per currency, 10 12.2 hides amounts for `personal` and shows tokens, and "Price missing" needs a marker. Never `triggered_by` or any person: in threads only costs per run (20 9). Ruling for docs/DECISIONS.md; update the example of `getThreadCosts` (white label, `model-large`, `USD`).
    d. Display rule (10 12.2, BR-11, D-BR-01): for a run with `billing_mode = personal`, `costs` is absent and its amounts are left out of `totals`, unless `billing.show_personal_estimates` is true; `billing_mode` and `tokens` are always present. Mode `organization` always has amounts with `is_estimate` ("Estimate" until reconciled). A thread without entries answers `200` with empty `totals` and `runs`.
    e. Ruling for docs/DECISIONS.md: runs are ordered by `run_id` ascending (UUIDv7, the order of the runs), and the list is not paginated because the schema has none (spec gap for very long threads).

9. **Other cost members.** `Run.cost`, `RunSummary.cost`, the run document member `/cost` and `AgentSession.cost` are required `Money` of M2-05, M2-10 and M2-12 and carry the stored values in both modes; clients show amounts from them only for `organization` (09 5.4 row "Costs"), and amounts that the switch `billing.show_personal_estimates` may reveal for `personal` come only from `getThreadCosts` (and later `getCostStats`), where the server applies the switch. Ruling for docs/DECISIONS.md; write this rule into 10 12.2 (spec gap: the switch is a server key that clients cannot read).

10. **`listPrices`** (`price.read`, owner, `rl.read`, no audit): `authorize(actor, price.read, instance)`; a person who is not an owner, and an owner's personal token (05 2.3 point 1), get `403 forbidden` with `reason` `owner_only` from M1-28. All rows, closed ones included, ordered by `provider`, `model`, `unit`, `valid_from`, `id` ascending; `valid_to` absent while open; no `created_by` in the answer (not in `Price`). Ruling for docs/DECISIONS.md: the price table is a small list in the sense of 06 6 point 6 (`PriceList` has only `items`); add it to that list.

11. **`createPrice`** (`price.manage`, owner, step-up, audit, `rl.admin`, `Idempotency-Key` through M1-20):
    a. `authorize(actor, price.manage, instance)` first: a person who is not an owner gives `403 forbidden` (`owner_only`), any token (agent, app, an owner's personal token) gives `403 forbidden` with the reason M1-28 assigns (`owner_only` or `step_up_not_possible`, 05 2.3 point 1, R-AZ-03), an owner without fresh step-up gives `401 step_up_required`; denials are audited by the authz layer (Audit `yes`).
    b. `domain.ValidatePrice` (pointers on the body, codes of M1-18): `provider` and `model` non-empty after trimming and passing `textcheck.DisplayText`; `price_micros` not negative and below the integer bound of 06; `currency` of the form `^[A-Z]{3}$`; `valid_to`, when present, after `valid_from` (`invalid`); `source_url` an absolute `https` URL with a host and without user info or fragment (`invalid_format`); `retrieved_at` not after `now` (`invalid`). Ruling for docs/DECISIONS.md: https only and no future retrieval time, because 03 calls the source "public pricing page" and a retrieval in the future is not a fact.
    c. A row of the same `provider` and `model` (any unit) whose validity overlaps the new one and whose currency differs gives `409 conflict` with `reason` `price_currency_mismatch` (note 4 b needs one currency per price set). Insert with a new UUIDv7 and `created_by` = the actor; an exclusion or unique violation (SQLSTATE `23P01` or `23505`) gives `409 conflict` with `reason` `price_overlap`. A concurrent pair of inserts of different units and currencies can both pass the currency check; note 4 b then treats the set as incomplete, so no wrong `computed` amount can result. Ruling for docs/DECISIONS.md: both reasons and this race rule.
    d. A `valid_from` in the past is allowed: it cannot change a `computed` booking, because such a booking had every needed unit and the exclusion constraint forbids a second row for it (note 4 e covers the others).
    e. Audit in the transaction (note 14) and `201` with the `Price`.

12. **`closePrice`** (`price.manage`, as note 11 a, body `PriceClose{valid_to}`):
    a. Load the row `FOR UPDATE`; unknown `price_id` gives `404 not_found`. `valid_to` not after `valid_from` gives `400 validation_failed` (`invalid`, pointer `/valid_to`).
    b. The row is already closed: the same `valid_to` answers `200` with the row and an audit entry with `no_change: true` (06 2 "No effect"); another value gives `409 conflict` with `reason` `price_closed`.
    c. The new end must not cut off a booking: when a valid or superseded `computed` entry with the same `provider` and `model`, with `occurred_at` at or after the row's `valid_from` and at or after the new `valid_to`, has a count above `0` for the row's unit, answer `409 conflict` with `reason` `price_in_use` (R-DM-23). The partial index `(is_estimate, occurred_at) WHERE is_estimate` serves this query until reconciliation; after it the superseded entry stays in the table.
    d. Concurrency: booking holds the rows of its price set `FOR SHARE` (note 5 e) and `closePrice` takes `FOR UPDATE`, so a close waits for a running booking and then sees its entries, and a booking that waits for a close re-reads the row with its new end (READ COMMITTED) and leaves it out of the set. Test this race (Tests).
    e. Otherwise set `valid_to` (the only column the grant allows, note 2 c), audit, answer `200` with the `Price`. Ruling for docs/DECISIONS.md: the reasons `price_closed` and `price_in_use`, and that a closed row is never reopened or moved (a gap is filled by a new row).

13. **Start file** (10 12.3, PLAN row M2-22):
    a. `internal/billing/domain/startprices/start_prices.json`, schema `start_prices.v1`: `{"schema": "start_prices.v1", "rows": [{"provider", "model", "unit", "price_micros", "currency", "valid_from", "source_url", "retrieved_at"}]}`. Rows: the base prices per unit of the five token units for every model in the main price table of the pricing page cited in 10 12.3 (no surcharges, discounts or tiers, 10 12.3), read once during the build; `source_url` that page, `retrieved_at` the time of reading in UTC, `valid_from = retrieved_at`; `provider` and `model` exactly as the runtimes report them in `CostReport` (09 18.7: the model identifiers of the runtime; take the identifiers from the model overview of the same documentation site when the pricing page shows only display names, and name that page in the report). `Rows()` parses with unknown members rejected.
    b. No loader: the server never reads the file at runtime. Loading it on the host would need a system action (right `system`, Audit `yes`, like `instance.bootstrap`, R-AZ-15) and a row in 02 3.3, and this ticket may not add an action to 05 (P-11). Until a core-contract change adds one, the owner enters prices with `createPrice`.

14. **Audit details** (10 14 row "Prices": row, source, retrieval time; 19 SEC-AUD-5): action `price.manage`, resource kind `instance`, attributes `price_id` (ID), `op` (code `create` or `close`), `valid_from` (time), `valid_to` (time, when set), `retrieved_at` (time), `source_url`, `no_change` (bool, only for a close without change). `source_url` is free text, which SEC-AUD-5 forbids unless a ticket extends it: add the value kind `auditing.SourceURL(u string) Value`, accepted only under the key `source_url` and only for an absolute `https` URL without user info and fragment (validated again in `audit/domain`), and the member in 19 SEC-AUD-5. Ruling for docs/DECISIONS.md.

15. **Configuration.** Register `billing.show_personal_estimates` (boolean, default from 10 17, chapter 10) in `configkeys_billing.go`; the billing `app` gets the value through its configuration port at construction. No other key is new; tests set the key through the configuration loader.

16. **Rights and requirements.** No change in `authz` code (only the requirement test `TestCostReadThreadRequirement`): `cost.read.thread` is `Level(read)` on `thread`, `price.read` and `price.manage` are `Owner` on `instance` in M1-28's table, none of them deferred. No action here raises the rights version or writes `access.changed`. `billing` writes no events: 02 4.7 has no row for it and the module register lists `events: []`.

17. **Rulings for `docs/DECISIONS.md`** (one row each, `D-M2-22-<n>`, with reason): notes 2 c; 3; 4 c; 4 e; 5 b; 5 c; 5 f; 5 g; 5 i; 6; 7 (token counts of corrections); 8 c; 8 e; 9; 10; 11 b; 11 c; 12 e; 14. Rules already in the spec (precedence of `computed`, no conversion, valid sum, display rule, non-overlapping validity) are not repeated there.

## Tests (write them first)

Unit:

- `internal/billing/domain`: `TestValidateReports` (empty provider, control and bidi characters in model, negative tokens and amount, value at the integer bound, lower-case and four-letter currency, duplicate provider and model, absent token fields count `0`); `TestPriceSet` (`valid_from` equal to `T` included, `valid_to` equal to `T` excluded, open end, other model and other case not matched); `TestCompute` (complete set gives `computed` with the exact amount; `tokens_cache_write_1h` priced with `cache_write_1h_mtok` and never with `cache_write_mtok`; a needed unit missing gives `adapter_reported` with an amount and `unpriced` with `0` without one; an adapter amount next to a complete set is ignored; zero tokens with a set give `computed` `0`; a set with two currencies is incomplete; the `request` unit is ignored; rounding half up exactly at `.5` micro; the result at the bound fails); `TestAmountProperty` (random counts and prices: equal to a `math/big` reference with one rounding, never negative); `TestValidatePrice` (every rule of note 11 b); `TestCloseDecision` (open row closed, same end no change, other end on a closed row `price_closed`, end not after start invalid); `TestSums` (two currencies stay apart, two modes stay apart, a superseded entry is left out and its successor counted, `is_estimate` true when any entry is an estimate, token sums); `TestDisplayRule` (BR-11 unit: `personal` without the switch has no `costs` and no total, tokens present; with the switch amounts present; `organization` always with amounts).
- `internal/billing/domain/startprices`: `TestStartPrices_EveryRowHasSourceAndRetrievedAt` (the plan's acceptance test: every row has a non-empty `source_url` and a `retrieved_at`); `TestStartPrices_Valid` (at least one row; every row passes `ValidatePrice`; only the five token units; one currency per provider and model; no overlapping validity of one provider, model and unit; `valid_from` equals `retrieved_at`; unknown members rejected).
- `internal/kernel/auditing` and `internal/audit/domain`: `TestSourceURLValue` (accepted only under `source_url`; `http`, user info, fragment and relative URLs rejected).
- `internal/authz`: `TestCostReadThreadRequirement` (decide with M1-28's fake facts in the world of 07 11.1): Sam with G-4 on Support/Allgemein `allow`; Lotse `allow`; Sam without a grant `not_found`; an app principal with an active installation and a `read` grant on Support/Allgemein `forbidden` `subject_kind_not_allowed` (05 2.6 has no scope for `cost.read.thread`); Tracker (G-6 only on Support/Eskalation) `not_found` like AZ-M34. No integration case uses an app token: app tokens exist only from M2-24, which is not in this ticket's closure (M1-44, `createToken` step 3), so apps are tested with the authz fakes as in M2-16.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, agent tokens and people from the identity test helpers, the world builder of `test/integration` for the world of 07 "Examples", fake clock; Robin is the only owner and the credential holder of the agent Assistent (`personal`) that operates Research/Notizen; Alex has `trigger` on Support (G-1), whose channel Allgemein is operated by Lotse (`organization`); Sam has `read` on Support/Allgemein (G-4) or nothing, per test; prices from `createPrice` with provider `example`, model `model-large` with all five units in `USD`, model `model-small` with only `input_mtok` and `output_mtok`, model `model-eur` with all units in `EUR`, source `https://provider.example.com/pricing`; runs reach `running` through the paths of M2-05 to M2-09):

- `TestMigration_PriceTable` and `TestMigration_CostEntries`: columns, types, nulls and defaults as 03; both checks; every FK; the unique and partial indexes; an overlapping row of the same provider, model and unit fails with `23P01`, an adjacent one (`valid_to` equal to the next `valid_from`) passes; `btree_gist` is installed; `bothy_app` can insert and read, can update `price_table.valid_to`, cannot update any other column of `price_table`, cannot update or delete `cost_entries`; the FK of `agent_session_id` exists.
- `TestFinishRunBooks_BR09`: Lotse finishes with `costs` for `model-large` (all counts above `0`, with an `amount_micros` that differs from the price), for `model-unpriced` with an amount, and for `model-none` without one: three entries `computed` (the computed amount), `adapter_reported` (the adapter's amount and currency) and `unpriced` (`0`); all `is_estimate`, `billing_mode = organization`, `occurred_at = finished_at`, copies of agent, thread, channel and space; `runs.cost_micros`, `currency`, `cost_is_estimate` as note 5 f; `version` raised once; exactly one `run.state_changed`, whose `run.cost` is the booked sum; one `bothy.run.cost` data point per entry with the four attributes after commit (in-memory metric reader).
- `TestFinishRunPartialSet`: `model-small` with cache counts above `0` and an amount gives `adapter_reported`; without an amount `unpriced`.
- `TestFinishRunRepeatBooksNothing`: the same `finishRun(completed, costs)` again answers `200` and adds no entry; parallel identical finishes book once.
- `TestFinishRunCostsRules`: invalid reports give `400 validation_failed` with the pointers of note 3 under `/costs` and leave run and table unchanged; `failed` from `queued` with a non-empty `costs` gives `400` (`invalid`, `/costs`) and the run stays `queued`; a finish without `costs` or with `costs: []` books nothing and leaves `cost_micros`, `currency` and `cost_is_estimate` unchanged; before the finish the test sets non-default values (for example `1234`, `EUR`, `false`) with a direct `UPDATE runs` through `pgtest`'s `SuperConn`, because `patchRun` belongs to M2-10, which is not in this ticket's dependency closure (if M2-10 is merged on the base, set them with `patchRun` on `/cost` instead); the M2-09 answer `not_enabled` is gone.
- `TestFinishRunTwoCurrencies`: `model-large` and `model-eur` in one run give two entries; the run columns hold the first entry's currency only (note 5 f); `getThreadCosts` shows both currencies, never converted.
- `TestSessionCostSum`: two runs in one session raise `cumulative_cost_micros` by their sums, `version` by one each, no `agent_session.changed`; an entry of another currency is not added.
- `TestBookingsStayRecomputable` (R-DM-23): after bookings, `closePrice` at a later end and a new row from then on, then `RecomputeComputed` reports no difference; `closePrice` with an end before a `computed` booking that used the row gives `409 conflict` `price_in_use` and changes nothing; the same end on a row whose unit had count `0` in every booking after it passes.
- `TestCloseRaceWithBooking`: parallel `closePrice` with an end before `now` and a booking at `now` on the same set: either the booking is `computed` and the close fails `price_in_use`, or the close succeeds and the booking is not `computed`; `RecomputeComputed` reports nothing.
- `TestGetThreadCosts_Organization`: Alex reads a Support thread with three runs: `runs` by `run_id`, `costs` per currency, `tokens` summed, `unpriced` true only for the run with an `unpriced` entry, `totals` per mode and currency; no JSON `null`, no person in the body; the body validates against `CostBreakdown`.
- `TestGetThreadCosts_Personal_BR11`: Robin triggers Assistent in Research/Notizen, the adapter starts with `personal` and finishes with costs; Robin's `getThreadCosts` has `billing_mode: personal`, `tokens`, no `costs` and no `personal` total; with `billing.show_personal_estimates` on (a second server instance with that configuration on the same database) the amounts appear with `is_estimate`.
- `TestGetThreadCosts_SupersededExcluded`: a `reconciled` entry inserted through `billingtest.InsertEntry` with `supersedes_id` replaces the estimate in every sum; its `is_estimate = false` makes the sum non-estimate only when no estimate remains.
- `TestGetThreadCosts_Rights_AZP4`: Sam without read gets `404` with the same status, body and headers as for an unknown thread id; Sam with G-4 gets `200`; Lotse's agent token gets `200`; an empty thread gives `200` with empty lists.
- `TestTotals_Levels`: entries in two currencies and both modes across two threads, two channels, two spaces and two agents: `Totals` per run, thread, session, agent, space and instance equal hand sums per mode and currency; a period cuts at `From` inclusive and `To` exclusive; a channel filter keeps only listed channels; an empty filter returns nothing; `ForDisplay` hides `personal` amounts unless the switch.
- `TestListPrices`: Robin gets every row in the order of note 10, a closed row with `valid_to`, an open one without; Alex gets `403 forbidden` `owner_only`; Robin's personal API token gets `403`.
- `TestCreatePrice`: `201` with the `Price`; one audit allow entry with the attributes of note 14; Robin without fresh step-up gets `401 step_up_required`; Alex gets `403` and a deny entry; an overlap gives `409 conflict` `price_overlap`; another currency overlapping the set gives `409 conflict` `price_currency_mismatch`; each rule of note 11 b gives `400 validation_failed` with its pointer; a replay with the same `Idempotency-Key` answers the stored `201` with `Idempotent-Replay: true` and inserts nothing.
- `TestClosePrice`: `200` and an audit entry; the same end again `200` with `no_change: true`; another end `409 conflict` `price_closed`; an unknown id `404`; an end not after the start `400`.
- `TestAZP1_CostRoutes`: the four operations are registered with their catalogue actions and classes and the server starts.

Contract: in the integration tests the responses of `getThreadCosts`, `listPrices`, `createPrice`, `closePrice` and `finishRun` validate against `CostBreakdown`, `PriceList`, `Price`, `Run` and `Problem`; problems carry `code` and, where named above, `reason`.

Red team focus of the milestone (PLAN 8 "Red team": abuse of a personal subscription, confused deputy over the server API) is covered by `TestGetThreadCosts_Personal_BR11`, `TestGetThreadCosts_Rights_AZP4`, `TestCreatePrice` (non-owner, token, step-up) and `TestFinishRunRepeatBooksNothing`; name them in the red team request together with the question whether any surface leaks `personal` amounts when the switch is off.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/billing/... ./internal/runs/... ./internal/agents/... ./internal/kernel/... ./internal/authz/... ./internal/audit/... ./internal/wire/... ./migrations/... ./test/integration/...`
- `make migrations-lock` after creating the migrations, then `make migrations-lint`
- `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`, `make api-tables`
- `make config-reference` then `make config-doc-check`
- `make docs`
- `make check`

## Acceptance criteria

- [ ] `getThreadCosts`, `listPrices`, `createPrice` and `closePrice` are enabled and registered; AZ-P1 passes.
- [ ] The migrations create `price_table` and `cost_entries` exactly as 03 4.9 with the exclusion constraint, the indexes and the grants of note 2.
- [ ] BR-09: a booking with a price gives `computed`, without a price but with an adapter amount `adapter_reported`, without both `unpriced` with `0`; `computed` wins over the adapter amount; every entry is an estimate with the run's actual mode and the copies of note 5 d.
- [ ] A repeated `finishRun` never books twice; invalid reports and reports for a run that never started give `400` and change nothing.
- [ ] Sums per run, thread, session, agent, space and instance count only valid entries and stand per billing mode and currency without conversion.
- [ ] BR-11 (server part): `getThreadCosts` gives no amounts for `personal` runs unless `billing.show_personal_estimates` is on, and always the mode and the token counts.
- [ ] `getThreadCosts` without read right gives the same `404` as an unknown thread (AZ-P4).
- [ ] Price changes need an owner with step-up, are audited with row, source and retrieval time, never overlap, never change a used row except its end, and never break the recomputation of a `computed` booking (R-DM-23).
- [ ] Every row of the start file has source and retrieval date (test).
- [ ] `bothy.run.cost` is recorded per booked entry after commit with the attributes of signals.md 4.3.
- [ ] No JSON `null` in any response of this ticket.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `billing` (port, tables, booking rules, valid sum, display rule, the loss of note 5 i, the note for M2-28 of note 7), `runs` (booking in the finish, cost columns, metric), `audit` (value kind `source_url`) and `agents` (`AddCost`) updated.
- Rulings of note 17 recorded in `docs/DECISIONS.md` section 10 (one row each with reason); the changes to 06, 09, 10, 19 and `openapi.yaml` of the Scope made in the same change.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found; at least: the baseline of note 6, the spec gap of note 13 b (loading the start file on the host needs a system action (right `system`, Audit `yes`, like `instance.bootstrap`, R-AZ-15) in 05 4.12 and a row in 02 3.3; until a core-contract change adds it, the owner enters prices with `createPrice`), the pages used for the start file and the time they were read, and every difference between the names of this brief and the merged code.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
