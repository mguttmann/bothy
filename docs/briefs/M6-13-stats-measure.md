# M6-13: stats: own measurement

Status: ready | Depends on: M2-15, M2-21, M2-22, M3-20, M3-21, M6-11 | Milestone: M6

## Goal

After this ticket the server measures itself for level a of the visualizations (25 3, ruling T7): the cross-cutting port `kernel/measure` has its one method `Record(metric, key, value)`, the module `stats` implements it with in-memory counters per process and key `(minute, metric, channel_id, agent_id, dimension, currency)`, and every measuring point of the closed list of 25 3.2 calls it next to its OpenTelemetry instrument: `messages_created`, `threads_created` (`messaging`), `runs_started`, `runs_finished`, `tool_calls`, `approval_cards`, `invocations_denied`, `sli.run_start`, `sli.turn_duration` (`runs`), `cost` (`billing`), `billing_mode_switches` (`agents`), `sli.push_acceptance` (`notify`), `catchups`, `resyncs`, `slow_consumer_closes`, `sli.delivery_latency`, `sli.reconnect` (`gateway`) and `sli.api_availability` (`platform/httpserver`). SLI thresholds come only from the generated `slibounds.gen.go` of M6-11. Every `stats.flush_interval` each api and worker process writes its counters with an additive upsert into `stats_minutes` (new migration, partitioned by day) in its own short transaction; a measuring point never writes to the database and never blocks the domain path; when the database is unreachable or more than `stats.flush_queue_max` keys are held, the oldest counters are discarded and `bothy.stats.flush.dropped` counts them; an orderly shutdown flushes once. The job `stats.retention` creates partitions ahead and detaches those older than `retention.stats_minutes`, and `bothy config check` rejects a retention shorter than the longest SLO period. VZ-T01, VZ-T02, VZ-T04, VZ-T14 and VZ-T15 are green. Why: the statistics must not read the telemetry stack (R-OB-08), and delivery latency, reconnects and push acceptance exist in no domain table; the live frame, the timeline and the traffic light of M6-14 and M6-15 read these minutes.

## Read first

- `docs/spec/25-visualization.md`: 1, 2 (the diagram: measuring points report twice), 3 (all of it: 3.1 principle, 3.2 the closed metric table with measuring point, `count`, `sum`, `good`, `dimension` and channel or agent per metric, 3.3 bounds, 3.4 writing, 3.5 retention), 4 rule 3 (no counters about people), 11, 12 (VZ-T01, VZ-T02, VZ-T04, VZ-T07, VZ-T14, VZ-T15), 14 (R-VZ-02, R-VZ-04), 15 (`stats.flush_interval`, `stats.flush_queue_max`).
- `docs/spec/03-data-model.md` (core contract, read only): `stats_minutes` (4.15: columns, checks, unique key `NULLS NOT DISTINCT`, indexes, partitioning, retention class `stats`).
- `docs/spec/20-privacy.md`: retention classes (row `stats`, `retention.stats_minutes`), DP-T12.
- `docs/spec/02-architecture.md`: 3.2 (roles `api`, `worker`, `all`), 4.5 (row `kernel/measure`), 11 (lifecycle and shutdown order), 12.3 (jobs table: add `stats.retention`), 13 (configuration schema).
- `docs/observability/signals.md` 4.3b (`bothy.stats.flush.dropped`, `bothy.stats.flush.duration`), and the OTel instruments next to which each measuring point records (rows of `http.server.request.duration`, `bothy.delivery.duration`, `bothy.gateway.connect.duration`, `bothy.gateway.catchup.duration`, `bothy.run.start.latency`, `bothy.push.send.duration`, `bothy.run.cost` and the gateway close counters), `docs/observability/slos.md` 1 and 2 (good and all per SLI; thresholds are bucket boundaries, R-OB-06).
- `docs/spec/09-agents.md` 11.5 (approval cards and their states), `docs/spec/10-backend-policy.md` (I2, I3, `billing_mode_mismatch`), `docs/spec/04-sync-protocol.md` 6.4 (close `4014`), 7.3 (`resync_required` reasons).
- `docs/DECISIONS.md`: ruling T7, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-13-<n>` rows).
- Briefs (merged names win, D-W-100): `M6-11-viz-package.md` (Design note 6: `slibounds.gen.go`), `M1-13-unit-of-work.md` (phase hooks: after commit), `M1-11-lifecycle-health.md` (components, shutdown order), `M1-14-background-jobs.md` (periodic jobs, queues), `M1-68-core-metrics.md` (the metric registry and the measuring points of the OTel side), `M1-63-sync-catch-up.md`, `M1-64-gateway-connection.md`, `M1-65-gateway-delivery.md` (where catch-up, reconnect, delivery and `4014` happen), `M2-09-run-lifecycle.md`, `M2-15-components-interactions.md` (approval cards), `M2-21-billing-switch-i4.md` (switches), `M2-22-costs.md` (valid booking), `M3-19-notify-jobs.md`, `M3-20-apns-adapter.md`, `M3-21-fcm-adapter.md` (provider results), `M1-07-config-schema-loader.md` (`bothy config check`).
- Code on the base branch (all under `services/server/`): `internal/kernel/measure` (package doc only), `internal/stats`, the modules named above, `internal/platform/httpserver`, `internal/platform/jobs`, `internal/wire`, `migrations/`, `architecture/modules.yaml`.

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
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; SLI thresholds only from `slibounds.gen.go`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket; the table `stats_minutes` is built exactly as 03 has it. Everything this ticket adds is English.

Product requirements implemented here:

11. `kernel/measure.Recorder` has exactly one method `Record(metric, key, value)`; a call increments counters in memory and never touches the database, the network or a lock that the domain path can wait on for longer than a map update (25 3.1, R-VZ-04).
12. The metric list is closed: exactly the 18 values of 25 3.2 and the `CHECK` of 03, with their dimension codes; an unknown metric or dimension is a programming error caught by a test, never a row (25 3.2).
13. `good` counts an SLI event as good up to and including the threshold of `slibounds.gen.go`, exactly as the histogram buckets count it (R-OB-06, VZ-T02); `good <= count` always.
14. Domain counters are recorded only after the commit of the transaction that caused them (a rolled back message is not counted).
15. Flush every `stats.flush_interval` per process with the additive upsert of 25 3.4 into its own short transaction; several processes and nodes add into the same row; no coordinator.
16. Loss is bounded and counted: database unreachable or more than `stats.flush_queue_max` keys discards the oldest keys and counts `bothy.stats.flush.dropped` with `bothy.reason`; an orderly shutdown flushes once; a crash loses at most one interval (25 3.4).
17. No people and no content: `stats_minutes` has no column for a person; keys carry only channel, agent, dimension code and currency (25 1, 4 rule 3, DP-T12).
18. Retention: partitions per day, created ahead and detached after `retention.stats_minutes` by the job `stats.retention`; `bothy config check` fails when `retention.stats_minutes` is shorter than the longest SLO period (25 3.5, VZ-T14, VZ-T15).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/kernel/measure/measure.go`: `type Metric string` with the 18 constants, `type Key struct{ChannelID, AgentID ids.ID; Dimension string; Currency string}`, `type Value struct{Count, Sum int64; Good bool}` (or `Count`, `Sum`, `Good` as the counters need), `type Recorder interface{ Record(metric Metric, key Key, value Value) }`, `Nop` for tests and for roles without stats, and the dimension code lists per metric (Design note 1).
- `internal/stats/domain/`: key validation against the closed lists, the SLI good decision from `slibounds.gen.go` (`IsGood(metric, duration)`), minute truncation (Design note 2).
- `internal/stats/app/recorder.go`: the in-memory recorder (sharded maps or one mutex with a bounded critical section; Design note 3), `flush.go` (the flush loop as a lifecycle component of M1-11 with its shutdown flush), `retention.go` (the job use case).
- `internal/stats/adapters/postgres/`: sqlc queries for the additive upsert (one multi-row statement per flush, chunked), partition creation and detaching.
- `internal/stats/adapters/jobs/retention.go`: job `stats.retention` on queue `maintenance`, periodic, role `worker` and `all`.
- Migration `migrations/<next free number>_stats_minutes.sql`: `stats_minutes` exactly as 03 4.15 (partitioned by range on `bucket_start`, the checks, the unique key `NULLS NOT DISTINCT`, the indexes) and the partitions for the current day and `stats.flush_interval` ahead as the job does (Design note 5); `make migrations-lint`, `make migrations-lock`.
- Tests named under "Tests".

Change (measuring points, each next to its existing OTel instrument; Design note 4):

- `internal/messaging`: `messages_created` (dimension = `origin`), `threads_created`, after commit.
- `internal/runs`: `runs_started` (dimension `billing_mode_used`), `runs_finished` (dimension end state, `sum` duration), `tool_calls` (end state, duration), `approval_cards` (`requested`, `approved`, `denied`, `expired`), `invocations_denied` (`i2_denied`, `billing_mode_mismatch`), `sli.run_start`, `sli.turn_duration`.
- `internal/billing`: `cost` (dimension mode, `currency`, `sum` micro-units) on a valid booking.
- `internal/agents`: `billing_mode_switches` (dimension new mode) on `agent.billing_mode.set` and `.set_all`.
- `internal/notify`: `sli.push_acceptance` (dimension provider) on each provider response.
- `internal/gateway`: `catchups`, `resyncs` (dimension reason code), `slow_consumer_closes`, `sli.delivery_latency`, `sli.reconnect`.
- `internal/platform/httpserver`: `sli.api_availability` for every REST response except `/api/v1/gateway`.
- `internal/wire`: the recorder for roles `api`, `worker`, `all`; the flush component in each; the job kind `stats.retention`; configuration keys `stats.flush_interval`, `stats.flush_queue_max`, `retention.stats_minutes` enter the schema (first user; `make config-reference`, `make config-doc-check`); `bothy config check` gains the retention rule (Design note 6).
- `architecture/modules.yaml`: only if a module that records does not yet list `measure` among its kernel ports (it is a kernel port for all, 02 4.5); `make register-check`.
- `tools/opscheck/opscheck.py` check `sli-labels` (M6-11): also compare the `sli.*` values of the Go constants in `measure.go` with the labels (a constant without an SLO or an SLO metric without a constant fails), with a test.
- Docs: `docs/spec/02-architecture.md` 12.3 (row `stats.retention`) and 11 (shutdown flush), `docs/spec/25-visualization.md` 3.4 only where the build proves a detail wrong, `internal/stats/README.md`, `internal/kernel/README.md` (if it lists the ports), `docs/observability/signals.md` 4.3b only if an attribute changes; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M6-13-1` to `D-M6-13-7`.

## Out of scope

- Queries over `stats_minutes`: timeline, traffic light, backend view (M6-14), the live frame and its stream (M6-15).
- Client metrics (`client_metrics.submit`, M4-31) and the statistics queries over domain tables (M6-05).
- Multi-node specifics beyond the additive upsert (M7-17).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-13-stats-measure`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(stats): ...`, `feat(gateway): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs of this ticket carry no person, channel or agent ID. No network calls except package downloads for pinned tools and Go modules; push providers are the fakes of M3-20 and M3-21.
- Time only through `kernel/clock`; tests move a fake clock and call the flush and the job directly, they never sleep for an interval.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Port** (25 3.1; 02 4.5; Ruling for docs/DECISIONS.md). `kernel/measure` is standard library only. `Record` takes the metric constant, the key and the value; `Value.Good` is set by the measuring point from `stats/domain.IsGood` through a small helper in `kernel/measure` that the generated bounds feed at wiring time (`measure.SetBounds(map[Metric]Bound)`), so a measuring point in `gateway` does not import `stats`. The dimension lists of 25 3.2 are constants in `measure.go`; `Record` with an unknown dimension in a test build panics, in production it is dropped and counted under `bothy.stats.flush.dropped` with reason `invalid_key` (add the value to signals.md 4.3b in the same change). Ruling: the bounds are injected because modules must not depend on `stats` (02 4.4).

2. **Keys and minutes.** The minute is the truncated clock time at `Record` (`kernel/clock`, UTC). `channel_id` and `agent_id` per the column "Channel, agent" of 25 3.2: zero means NULL. `currency` only for `cost`. The good decision: a duration exactly equal to the threshold is good (R-OB-06: `le` bucket boundaries are inclusive); for SLIs without a threshold (`sli.api_availability`, `sli.push_acceptance`) good is the outcome of 25 3.2 (`without 5xx`, `accepted`); events that slos.md excludes from "all" (`client_error` for reconnect, `rejected_token` for push, `cancelled` for run start, the gateway path for availability) are not recorded at all.

3. **Recorder and flush** (25 3.4; Ruling for docs/DECISIONS.md). Counters in a map keyed by `(minute, metric, key)`; `Record` takes a mutex for one map update only (or uses sharding; measure with a benchmark that `Record` stays below a microsecond on the build host and report it). The flush component swaps the map under the lock, writes the swapped map outside it in chunks with one `INSERT ... ON CONFLICT (bucket_start, metric, channel_id, agent_id, dimension, currency) DO UPDATE SET count = stats_minutes.count + excluded.count, sum = stats_minutes.sum + excluded.sum, good = stats_minutes.good + excluded.good, updated_at = now()` per chunk, in its own short transaction, never in a domain transaction. On a database error the swapped map is merged back (bounded by `stats.flush_queue_max`); when the merged map exceeds the bound, the oldest minutes are dropped first and counted with reason `queue_full`; a failed write counts its keys with `db_error` only when they are finally dropped. `bothy.stats.flush.duration` per flush. Shutdown: the component flushes once with the shutdown deadline of M1-11 and then stops; keys that do not make it are counted as dropped.

4. **Measuring points** (25 3.2). Each point records next to the existing OTel instrument and with the same outcome classification, in the same function, so the two cannot diverge in their notion of an event. Domain counters (`messages_created`, `threads_created`, `runs_*`, `tool_calls`, `approval_cards`, `invocations_denied`, `cost`, `billing_mode_switches`) record in the after-commit hook of the unit of work (M1-13), never inside the transaction. `approval_cards`: `requested` when a message with a component with `required_capability = approve_actions` is created by an agent during a run; `approved` and `denied` on an `interaction.create` whose choice is `once`, `session` or `always` (approved) or `deny` (denied); `expired` when such a component expires unused (M2-15). `invocations_denied`: `i2_denied` where `agent.invoke` denies before delivery with a reason of I2, `billing_mode_mismatch` where a run fails with that reason. Gateway: `catchups` with the duration when `catchup.complete` is sent; `resyncs` with the reason code of 04 7.3; `slow_consumer_closes` on a close with `4014`; `sli.delivery_latency` when a `dispatch` frame of a durable event is written live (not during catch-up), duration from the event's commit time; `sli.reconnect` from upgrade to `ready` or `resumed`, or to an abort that is not a `client_error`. HTTP: one `sli.api_availability` per REST response, good without `5xx`, excluding the gateway path.

5. **Partitions and retention** (03 4.15; 25 3.5; Ruling for docs/DECISIONS.md). Partition per UTC day named `stats_minutes_YYYYMMDD`. The job `stats.retention` runs at least every `stats.flush_interval` multiplied by a factor you document (or on a fixed schedule of once per hour; choose and document), creates the partitions for today and the next day if missing, and detaches and drops partitions whose day ends before `now - retention.stats_minutes` (`ALTER TABLE ... DETACH PARTITION ... CONCURRENTLY` where PostgreSQL 18 allows it in this context, otherwise plain detach). A flush into a day without a partition creates it on demand in the flush transaction's retry (or the table has a default partition that the job drains; choose one, document it and test it). Retention class `stats` per 20.

6. **Config check** (25 3.5; VZ-T14). `bothy config check` loads `retention.stats_minutes` and the longest `Period` of `slibounds.gen.go` and fails with a message naming both keys when the retention is shorter.

7. **Rulings for `docs/DECISIONS.md`** (section 10): `D-M6-13-1` the port with injected bounds and closed dimensions (note 1); `D-M6-13-2` the good decision and the excluded events (note 2); `D-M6-13-3` recorder, flush, drop order and counting (note 3); `D-M6-13-4` after-commit recording of domain counters (note 4); `D-M6-13-5` approval card and denial classification (note 4); `D-M6-13-6` partitions, the job schedule and the missing-partition rule (note 5); `D-M6-13-7` the config check (note 6).

## Tests (write them first)

Unit:

- `internal/kernel/measure`: `TestClosedMetricsAndDimensions` (18 metrics equal the 03 `CHECK` list, read through a narrow parser of 03 or through the migration file; each dimension list equals 25 3.2); `TestUnknownDimension` (panics in test builds).
- `internal/stats/domain`: `TestIsGoodAtThreshold` (VZ-T02: exactly at, just below and just above each threshold of `slibounds.gen.go`; the same values against the OTel histogram bucket of the same instrument through the manual reader of M1-68 give the same classification).
- `internal/stats/app`: `TestRecorderAggregatesPerMinuteAndKey`; `TestRecordDoesNotBlock` (a flush blocked on a fake database does not delay `Record`; benchmark recorded); `TestFlushDropsOldestOverQueueMax` and `TestFlushDatabaseDown` (VZ-T04: the domain call still succeeds, `bothy.stats.flush.dropped` rises with the right reason); `TestShutdownFlushesOnce`.

Integration (`pgtest`, real modules through `internal/wire`):

- `TestMeasure_TwoProcessesAddUp` (VZ-T01): two recorder instances with the same fake clock record the same keys and flush into one database; each row holds the sum; `good <= count` everywhere.
- `TestMeasure_EveryMetric` (VZ-T01): one scenario per metric of 25 3.2 driven through the real use case or path (message, thread, run start and end, tool step, approval card requested and approved and an expired one, I2 denial, `billing_mode_mismatch`, cost booking, billing mode switch, push send through the fake APNs and FCM providers with an accepted and a rejected token, a catch-up, a `resync_required`, a `4014` close, a live delivery, a reconnect, REST responses with `200` and `500`); after a flush each metric has its row with the expected `count`, `sum`, `good`, dimension and channel and agent columns.
- `TestMeasure_RollbackNotCounted`: a message whose transaction rolls back records nothing.
- `TestMeasure_NoDatabaseWriteInDomainPath`: with the `stats_minutes` table locked by another session (`LOCK TABLE ... ACCESS EXCLUSIVE` held by the test), creating a message and finishing a run return without waiting; the flush waits or fails and counts, the domain does not.
- `TestMeasure_NoPeopleNoContent` (VZ-T07 part, DP-T12): after the scenario, no row and no column of `stats_minutes` contains a principal ID of a person, a canary message text or a canary step title (scan all columns); `privacy.per_person_stats_enabled = true` changes nothing.
- `TestRetention` (VZ-T15): with a fake clock and partitions for several days, the job detaches exactly those older than `retention.stats_minutes` and creates today's and tomorrow's.
- `TestConfigCheck_RetentionShorterThanSLO` (VZ-T14): `bothy config check` fails with `retention.stats_minutes` shorter than the longest SLO period and passes with the default.
- `internal/wire`: the recorder and flush component exist in roles `api`, `worker`, `all`; the job `stats.retention` is registered on queue `maintenance` in `worker` and `all`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/kernel/... ./internal/stats/... ./internal/messaging/... ./internal/runs/... ./internal/billing/... ./internal/agents/... ./internal/notify/... ./internal/gateway/... ./internal/platform/... ./internal/wire/...`
- `make migrations-lint`, `make migrations-lock`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`, `make signal-check`, `make ops-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] VZ-T01, VZ-T02, VZ-T04, VZ-T14 and VZ-T15 are green.
- [ ] Every metric of 25 3.2 has its measuring point next to its OTel instrument; domain counters record after commit only.
- [ ] Measuring points never write to the database (locked-table test); an orderly shutdown writes once; loss is counted.
- [ ] `stats_minutes` is built exactly as 03 4.15, partitioned per day, with the retention job; no people and no content in it.
- [ ] The `sli-labels` check also compares the Go constants with the SLO labels.
- [ ] 02 12.3 and 11, the READMEs, the CHANGELOG and the rows `D-M6-13-1` to `D-M6-13-7` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `stats` and of `kernel` (port list) updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
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
4. The measured cost of `Record` and of one flush, the chosen job schedule and partition rule.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
