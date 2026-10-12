# M2-01: Worker consumers of the event log

Status: ready | Depends on: M1-16 | Milestone: M2

## Goal

After this ticket the roles `worker` and `all` run the worker consumers of the event log
(ADR 0051): each consumer name of `event_consumer_cursors` (`push`, `webhooks`) has one row with
`last_seq`; a round locks that row with `FOR UPDATE SKIP LOCKED`, reads up to
`consumer.batch_size` events after it in `seq` order, lets the consumer's handler decide the
recipients and insert River jobs with `InsertTx`, and advances the cursor in the same transaction.
So every event creates its jobs exactly once, with two or more workers in parallel and after a
crash in the middle of a round. Rounds wake on the same `NOTIFY event_log` as the log tail and
every `consumer.poll_interval`. Every role exports the lag of each consumer (`head_seq` minus
`last_seq`) and the age of its oldest unprocessed event, and the alarm `BothyEventConsumerLag`
fires before a stalled consumer reaches the retention window `sync.event_retention`. This is the
frame P-13 asks for: producers never learn about their consumers (R-AR-08, R-DM-27), the first
production consumer `webhooks` plugs in with M2-26, and `push` follows with M3-19.

## Read first

- `docs/adr/0051-worker-consumers-of-the-event-log-with-a-cursor.md` (all: Decision Outcome
  items 1 to 6, Consequences, Confirmation, More Information with the open point for this
  ticket).
- `docs/spec/02-architecture.md`: 3.2 "Roles of a process" (row "Event consumers `push`
  and `webhooks`"); 4.2 "Module list" (row `sync`); 4.3 "Ports, events and sources per module"
  (`sync` port `Consumers` (worker cursors); do not confuse it with `agents` port `Consumers` of
  the agent consumers); 4.4 "Allowed dependencies" (`notify` and `apps` may use `sync`);
  5 "Layers within a module"; 8.1 "Flow", 8.5 "What never happens in a transaction"; 11.1
  "Writing", 11.2 "Log tail per api process", 11.5 "Worker consumers (push, webhooks)", 11.7
  "Cost of serialization"; 12.1 "Usage", 12.2 "Queues" (paragraph on maintenance
  mode), 12.3 "Job catalogue" (rows `notify.push_send`, `apps.webhook_deliver`,
  `sync.partition_maintenance`), 12.4 "Rules for jobs"; 13 "Configuration loading" items 4 and 5;
  14.4 "Failure of a dependency" (rows PostgreSQL and `LISTEN` connection); 14.5 "Panics";
  15.1 "Start" step 8; 15.2 "Health"; 15.3 "Shutdown" (worker step 2, role `all`); 16
  "Scaling path" (level 3); 19 "Decisions of this chapter" (R-AR-07, R-AR-08, R-AR-09);
  20 "Alignment with the core contract" (row `event_consumer_cursors`); 22 "Configuration values of this chapter" (`consumer.batch_size`, `consumer.poll_interval`,
  `database.listen_reconnect_backoff_cap`, `database.statement_timeout`,
  `shutdown.total_timeout`).
- `docs/spec/03-data-model.md`: 4.10 "Sync and personal data" (tables `event_log` with
  "Retention", and `event_consumer_cursors`); 6.1 "Global event sequence"; 6.3
  "Lock order"; 6.7 "Sync epoch and restore" (item 1: `bothy restore finalize`
  moves `event_consumer_cursors.last_seq` to the head, M7-04); 8 "Decisions of this chapter"
  (R-DM-27).
- `docs/spec/04-sync-protocol.md`: 3.2 "Pattern with a counter row" (items 3 and 4: a reader that
  sees `seq = k` sees every smaller number; no holes), 7.3 "`resync_required`"
  (`min_retained_seq`), 7.5 "Sync epoch" item 4, 7.6 "Consumers in the worker (push, webhooks)",
  15.1 "Connection and cursor" item 2 (an agent consumer starts at `head_seq`; the same idea for
  worker consumers, design note 3), 18 "Configuration values of this chapter"
  (`sync.event_retention`).
- `docs/spec/05-actions.md`: `push.deliver` and `webhook_out.deliver` (internal delivery checks),
  R-AZ-11; the row `settings.updated` in "Durable event types and receive action" (the test
  event type, design note 15).
- `docs/spec/07-permissions.md`: 7.4 "Query pattern" rule 1 (exception list for system reads such as
  the log tail).
- `docs/spec/11-extensions.md`: 6 "Isolation and limits" (last paragraph: a webhook
  subscription behind the retention window is `disabled` with "gap"), 7 "Outgoing webhooks";
  `docs/api/webhooks.md` 5 "Order and duplicates" (what M2-26 builds on this frame).
- `docs/spec/13-push-notifications.md` 12 "Execution in the server" (what M3-19 builds on this
  frame).
- `docs/spec/17-observability.md` 2 "Basic rules" (rules 3, 4, 8, 9), 10 "Tests that hold these rules"; R-OB-06.
- `docs/spec/20-privacy.md` 2 "Data categories per table" (row `event_consumer_cursors`,
  retention class `config`).
- `docs/observability/signals.md`: 1 "Names and units", 2 "Cardinality", 4.2 "Events and delivery" (pattern of `bothy.event.fanout.lag`), 4.3 (row `bothy.agent.consumer.lag`,
  the agent counterpart), 5.3 "Attributes in spans and logs", 7 "Logs".
- `docs/observability/alerting.md`: 1 "Levels and routes", 3.2 "Further alerts" (column
  "Configuration value"; rows `BothyEventFanoutLag`, `BothyAgentConsumerLag`, `BothyBackupStale`),
  4 "Runbook anchors"; `docs/observability/dashboards.md` (row `delivery-sync.json`).
- `infra/observability/prometheus/rules/bothy-alerts.yaml` (rules `BothyEventFanoutLag`,
  `BothyAgentConsumerLag`), `infra/otel/collector.yaml` (`redaction/allowlist.allowed_keys`,
  "Bothy attributes"), `infra/observability/grafana/dashboards/delivery-sync.json`.
- `docs/operations/configuration.md` 4 "Named values of all chapters" (rows `consumer.*`,
  `sync.event_retention`, `file.scan.pending_alert_after` as the naming pattern of an alarm age);
  `docs/operations/upgrade.md` 4 "Rules for migrations" (M3, M8, M12).
- `docs/security/threat-model.md` 5.9 "Worker and jobs", T-AZ-02, T-DS-01.
- `docs/DECISIONS.md` 10 "Rulings of the build milestones" (row format), D-K-A1, D-PN-07, D-W-100.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 8 "M2 Agent platform, extensions and terminal" with "Scope" (ADR 0051), "Test strategy" (chaos: crash of the adapter after the
  commitment, duplicate delivery), "Red team", "Dependencies" and the rows M2-01, M2-07 (agent
  consumers, a different mechanism), M2-26 (first production consumer `webhooks`); rows M3-19 (`push`), M3-24
  (`perf.push_handoff_p95`), M7-04 (`bothy restore finalize`), M7-08 (maintenance mode), M1-04,
  M1-68 and M1-70 (gates that check what this ticket adds); 19 "Decisions of this plan"
  P-09, P-10, P-11, P-13.
- M1 briefs this ticket builds on (M1-16 is the only direct dependency; the others are implied
  through it): `docs/briefs/M1-16-event-log.md` (table `event_log`, `sync.Event`, head as
  `max(seq)`, minimum as `min(seq)`, the tail with its dedicated `LISTEN` connection through
  `pg.DialDedicated`, `ErrGap`, partition maintenance, the appender buffering events in a
  `txn.Key`), `M1-14-background-jobs.md` (`platform/jobs`: `Args`, `Registry`, `Register`,
  `HandlerOptions`, `Inserter.InsertTx`, `Runtime` with roles and `Failed()`,
  `ValidateArgsType`, queue names), `M1-13-unit-of-work.md` (`txn.UnitOfWork.Do`,
  `txn.CurrentPhase`, `txn.Key`, `pg.Tx`, retries on `40P01` and `55P03`, the Semgrep rule that
  forbids `Begin` outside `platform/pg`), `M1-12-postgres-migrations.md` (`migrations/` numbering,
  grants, `pgtest.Main`, `pgtest.New`, `SuperConn`), `M1-11-lifecycle-health.md`
  (`lifecycle.Component`, `lifecycle.StopStep`, `lifecycle.PhaseConsumers`, `Runner.Fail`,
  `wire.Serve` extension points), `M1-10-telemetry-foundation.md` (`internal/platform/otel`,
  `otel/attr`, `TestAttrKeysEqualCollectorAllowlist`, `sloglint`), `M1-15-instance-counters.md`,
  `M1-07-config-schema-loader.md` and `M1-08-config-reference.md`. Read for coordination only
  (not in the dependency closure): `M1-04-ops-config-checks.md` (`alert-thresholds` check and
  `infra/observability/prometheus/tests/bothy-alerts.test.yaml`), `M1-68-core-metrics.md`
  (`internal/platform/otel/metrics` registry), `M1-70-leak-tests.md`
  (`tools/lint/sql_visibility.yaml`, category `log_tail`).
- Code on the base branch, read with `git show` before changing anything: `internal/sync`
  (port, `app`, `domain`, `adapters/postgres`, `adapters/jobs`), `internal/platform/jobs`,
  `internal/platform/pg` and `pgtest`, `internal/platform/lifecycle`, `internal/platform/otel`,
  `internal/kernel/txn`, `internal/kernel/clock`, `internal/wire` (`config.go` with
  `configKeyGroups`, `configchecks.go` with `configChecks`, `configchapters.go`, the role
  assembly), `architecture/modules.yaml`, `migrations/`.

No API operation, no action id, no durable or ephemeral event type and no audit entry is added
by this ticket; `services/server/api/operations.yaml` stays unchanged.

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
10. Specification first: a change to behaviour, API, event, permission or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. The frame lives in the module `sync`, which owns `event_consumer_cursors` and the inbound port
    `Consumers` (02 4.2, 4.3; 03 4.10). Worker consumers run only in the roles `worker` and `all`
    (02 3.2), start after River in start step 8 (02 15.1) and finish their round in worker
    shutdown step 2 (02 15.3).
12. `event_consumer_cursors` exactly as 03 4.10: `name text` with `CHECK (name IN
    ('push','webhooks'))`, `last_seq bigint NOT NULL DEFAULT 0`, `updated_at timestamptz NOT NULL
    DEFAULT now()`, primary key `name`, no foreign key, retention class `config` (20). A new
    consumer name is an additive migration of the check (ADR 0051 item 1, R-DM-27, upgrade.md M8).
13. One round is one transaction: lock the row with `FOR UPDATE SKIP LOCKED` (a locked row ends
    the round at once), read up to `consumer.batch_size` events after `last_seq` in `seq` order,
    decide recipients and insert River jobs with `InsertTx` per event, advance `last_seq`, commit.
    Cursor and jobs commit together, so jobs are created exactly once, also with several workers
    (02 11.5 item 2, ADR 0051 item 2, 03 4.10, 04 7.6).
14. The wake signal is the same `NOTIFY event_log` as for the log tail, plus a poll every
    `consumer.poll_interval` (02 11.5 item 3, ADR 0051 item 3). Losing the `LISTEN` connection
    loses nothing (02 14.4 row `LISTEN`; 04 3.2 items 3 and 4).
15. Producers do not know their consumers: no write path changes (R-AR-08, R-DM-27). A round calls
    nothing outside the process (R-AR-07, 02 8.5); job arguments carry only IDs and sequence
    numbers (02 12.1); consumer jobs check `push.deliver` or `webhook_out.deliver` with current
    permissions before sending (02 11.5 item 4, R-AZ-11), built by the consumer tickets.
16. Several workers need no code change (02 16, level 3: "consumers lock with `SKIP
    LOCKED`").
17. A metric of the lag per consumer (`head_seq` minus `last_seq`) stands in signals.md, and an
    alarm in alerting.md fires before the lag reaches the window `sync.event_retention` (PLAN
    row M2-01; ADR 0051 "More Information").
18. Telemetry only as the catalogue allows: names, units and attributes from signals.md, metric
    attributes from closed sets, constant English log messages with attributes from signals.md
    5.3, no payloads (17 rules 3, 4 and 9; signals.md 2 and 7).

## Scope

All paths under `services/server/` unless noted. Names below are the contract for M2-26, M3-19,
M7-04 and M7-08; where a merged M1 package already has a compatible name, keep it and list the
difference in the report.

Create:

- `migrations/<next free number>_event_consumer_cursors.sql` (goose, numbering rule of M1-12):
  the table of requirement 12; `GRANT SELECT, INSERT, UPDATE ON event_consumer_cursors TO
  bothy_app` (no `DELETE`); no rows are inserted (design note 3). Then `make migrations-lock`.
- `internal/sync/consumers.go` (port, standard library and `internal/kernel` only):
    - `type ConsumerName string` with `ConsumerPush ConsumerName = "push"` and
      `ConsumerWebhooks ConsumerName = "webhooks"`; `func ConsumerNames() []ConsumerName`
      (exactly the set of the check in 03); `func (n ConsumerName) Valid() bool`.
    - `type ConsumerHandler interface { HandleEvent(ctx context.Context, ev Event) error }`
      (`Event` is the stored row type of M1-16); the doc comment states the contract of design
      note 2.
    - `type ConsumerState struct { Name ConsumerName; LastSeq, HeadSeq int64; OldestPendingAt time.Time; UpdatedAt time.Time }`
      (`OldestPendingAt` zero when no event after `LastSeq` exists).
    - `type Consumers interface { Register(name ConsumerName, h ConsumerHandler) error; States(ctx context.Context) ([]ConsumerState, error) }`.
- `internal/sync/domain/consumer.go`: `func PlanRound(lastSeq int64, seqs []int64, minRetained int64) (RoundPlan, error)`
  with `type RoundPlan struct { Process []int64; NewLastSeq int64; LostFrom, LostTo int64 }`
  (`LostFrom` zero when nothing was lost) and `var ErrConsumerGap` (design note 5).
- `internal/sync/app/consumers.go`: `type ConsumerConfig struct { BatchSize int; PollInterval, ReconnectBackoffCap, StatesTimeout time.Duration }`;
  `type ConsumerService` implementing `sync.Consumers`, with
  `Ensure(ctx context.Context) error`, `Round(ctx context.Context, name sync.ConsumerName) (RoundResult, error)`
  and `type RoundResult struct { Outcome RoundOutcome; Processed int; Full bool }` with
  `RoundLocked`, `RoundIdle`, `RoundProcessed`; outgoing ports `ConsumerCursorStore` and
  `ConsumerLogReader` (design notes 3 to 6).
- `internal/sync/app/consumer_runner.go`: `type ConsumerRunner` with
  `Start(ctx context.Context) error`, `Stop(ctx context.Context) error`,
  `Failed() <-chan error`, and the outgoing port `Waker interface { Run(ctx context.Context) error; Wake() <-chan struct{} }`
  (design notes 7, 8 and 10).
- `internal/sync/adapters/postgres/queries/consumers.sql` (sqlc): `EnsureConsumerCursor`,
  `LockConsumerCursor`, `AdvanceConsumerCursor`, `ListConsumerCursors`, `ConsumerEventsAfter`
  (reuse the tail's read query of M1-16 if it has the same shape), `OldestEventAfter`; the
  adapters implementing `ConsumerCursorStore` and `ConsumerLogReader` in
  `internal/sync/adapters/postgres/consumers.go`.
- `internal/sync/adapters/postgres/waker.go`: the `Waker` on its own dedicated connection with
  application name `bothy-consumer-wake` (design note 8), sharing the listening code with the
  tail of M1-16.
- The two gauges of design note 9, either as `Spec`s in `internal/platform/otel/metrics` (if
  M1-68 is merged) or in `internal/sync/adapters/otelmetrics/consumers.go`.
- `internal/wire/configkeys_consumer.go`: `consumerConfigKeys()` with `consumer.batch_size`,
  `consumer.poll_interval` and `consumer.pending_alert_after` (`chapter02`, design note 11).
- Tests of the section "Tests" with their fixtures.

Change:

- `internal/sync/app` (appender of M1-16): an unexported way for the round to ask whether the
  current unit buffered durable events (design note 6).
- `internal/wire`: `configKeyGroups` gains `consumerConfigKeys`; `configChecks` gains
  `consumer_alert_below_retention`; `TestConfigSchemaBuilds` lists the new keys; the role assembly
  builds one `ConsumerService` for every role, registers the gauges in every role, and in `worker`
  and `all` runs `Ensure` and starts the runner after River with a stop step in
  `lifecycle.PhaseConsumers` (design note 10). The list of registered consumers is empty in this
  ticket, with a comment that M2-26 registers `webhooks` and M3-19 registers `push`.
- `internal/platform/otel/attr`: constant for `bothy.consumer.name` (span and log attribute,
  also used on the two gauges).
- `architecture/modules.yaml`: only if the gauges are registered in a `sync` adapter without the
  M1-68 registry, the OpenTelemetry vendor group of M1-10 in `adapter_vendors` of `sync`; then
  regenerate `.go-arch-lint.yml`.
- `infra/otel/collector.yaml`: `bothy.consumer.name` in `redaction/allowlist.allowed_keys`
  ("Bothy attributes").
- `infra/observability/prometheus/rules/bothy-alerts.yaml`: rule `BothyEventConsumerLag`
  (design note 9); `infra/observability/prometheus/tests/bothy-alerts.test.yaml`: its cases.
  M1-04 creates that file and the `alert-thresholds` check; if either is missing at build time,
  write the cases into a new file of that name in the `promtool test rules` format M1-04
  prescribes and name the missing check in the report.
- `infra/observability/grafana/dashboards/delivery-sync.json`: one panel "Worker consumers" with
  both gauges by `bothy_consumer_name`; `docs/observability/dashboards.md` row of that dashboard.
- `docs/observability/signals.md` (4.2 two rows, 5.3 one row), `docs/observability/alerting.md`
  (3.2 row, 4 runbook anchor `bothyeventconsumerlag`).
- `docs/spec/02-architecture.md`: 11.5 (design notes 3, 5, 6, 7, 8), 13 item 4 (the new cross
  check), 14.5 and 15.2 (design notes 7 and 10), 22 (row `consumer.pending_alert_after`); then
  `make config-reference` for `docs/operations/configuration.md`.
- `docs/adr/0051-worker-consumers-of-the-event-log-with-a-cursor.md`: "Confirmation" names the
  tests of this ticket; the last bullet of "Consequences" and "More Information" name the metric
  and the alarm instead of "Open"; the decision stays unchanged; `make adr-index`.
- `docs/security/threat-model.md` 5.9: the two rows of design note 13.
- `tools/lint/sql_visibility.yaml`: only if it exists (M1-70 merged), the exceptions of design
  note 12.
- `internal/sync/README.md`, `services/server/CHANGELOG.md` ("Unreleased"), `docs/DECISIONS.md`
  section 10 rows `D-M2-01-1` to `D-M2-01-9`.

This ticket creates no new top-level Makefile target.

## Out of scope

- The consumer `webhooks` with its handler, `webhook_subscriptions`, `webhook_deliveries`,
  `last_delivered_seq`, the job `apps.webhook_deliver`, the check `webhook_out.deliver` per
  attempt and the test "a permission revocation between round and job prevents the sending" of
  ADR 0051 "Confirmation" for webhooks (M2-26).
- The consumer `push`, the job `notify.push_send`, `push.deliver` (M3-19); the load test
  `perf/push_handoff` with `perf.push_handoff_p95` (M3-24).
- Setting every cursor to the new head in `bothy restore finalize` and detecting a restore
  without it, including a cursor ahead of the head (M7-04).
- Rounds that keep running in maintenance mode `read_only` (M7-08).
- Several api nodes (`cluster.mode = multi`, M7-17); several workers need nothing beyond
  `SKIP LOCKED`.
- Agent consumers with `agent_consumer_cursors` over the gateway and REST (M2-07).
- The metric registry and the gate `signal-check` (M1-68), the operations checks with
  `alert-thresholds` and the Prometheus rule tests (M1-04), the visibility lint AZ-P12 (M1-70):
  this ticket fills in what they check, it does not build them.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M2-01-worker-event-consumers`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(sync): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Register no production consumer: `webhooks` comes with M2-26 and `push` with M3-19. Probe handlers and the probe job kind exist only in test code.
- No real push service, webhook target or other third-party service in any test, and no outgoing network call from a round; nothing in this ticket costs money.
- Prometheus and Alertmanager tools only as the pinned gate tools of M1-04 inside `make check`; no global installation; any local copy of a pinned tool lives in a throwaway folder from `mktemp -d`.
- Never move a cursor by hand outside a test database; a cursor moved past unprocessed events loses them for good.

## Design notes

1. How the M2 section applies. "Scope" names ADR 0051; this ticket is its frame. The chaos
   items of "Test strategy" (crash of the adapter after the commitment, duplicate delivery)
   concern the agent path; the crash and two-worker tests here give the same proof for the worker
   path. The "Red team" of M2 treats webhook targets as a boundary; the frame keeps content out of
   logs and job arguments (note 13) and leaves the permission check per send to the consumer
   jobs (M2-26, M3-19). Exit criterion 10 (red team `JA`) covers this ticket's diff.
2. Port and handler contract (`internal/sync`). `HandleEvent` runs inside the round's
   transaction in phase `work`, once per event in `seq` order. It may read through ports and the
   ambient transaction, insert River jobs through `jobs.Inserter.InsertTx` (in the handler
   module's adapters, because `platform` is allowed only there, 02 5) and write rows of its own
   module. It never calls anything outside the process (R-AR-07), never appends a durable event
   (note 6) and returns an error to roll back the whole round. Only `internal/wire` calls
   `Register`, before the runner starts; a second registration of a name, an invalid name, a nil
   handler or a registration after `Start` is an error at startup. The worker consumers are
   internal: no API, and consuming is no action and writes no audit. A handler may still write, in
   the round's transaction and through `kernel/auditing`, the audit entry of a system action of
   its own module (M2-26: `webhook_out.transition`, 05 R-AZ-27, D-W-104). They are not the agent
   consumers of M2-07, which the `agents` port also calls `Consumers` (02 4.3); keep the
   package-qualified names apart in docs and code comments.
3. Cursor rows (Ruling for docs/DECISIONS.md, `D-M2-01-1`). In roles `worker` and `all`,
   `Ensure` runs before the runner starts and inserts one row per registered name in one unit of
   work: `INSERT INTO event_consumer_cursors (name, last_seq) SELECT $1, COALESCE(max(seq), 0)
   FROM event_log ON CONFLICT (name) DO NOTHING`. A new consumer therefore starts at the head and
   never replays retained events, like the first connection of an agent consumer (04 15.1 item
   2); unregistered names have no row and no lag series. No migration seeds rows: a seeded row
   for a consumer that arrives in a later release (`push`, M3-19) would make it replay every
   event retained since that migration. The column default `0` of 03 stays as written. Two
   workers starting together run the same statement; one row wins. Reading `max(seq)` in a
   statement is safe because a reader that sees `seq = k` sees every smaller number and an
   uncommitted transaction holds larger numbers (04 3.2 item 3).
4. The round, one `txn.UnitOfWork.Do` without `RightsChange`, in this order:
   a. `SELECT last_seq FROM event_consumer_cursors WHERE name = $1 FOR UPDATE SKIP LOCKED`; no
      row means another worker holds it: return `RoundLocked` at once (02 11.5 item 2).
   b. `ConsumerEventsAfter(last_seq, consumer.batch_size)`: every column of `event_log` with
      `seq > last_seq ORDER BY seq LIMIT consumer.batch_size`, mapped to `sync.Event`.
   c. No event: return `RoundIdle` without writing.
   d. `domain.PlanRound` (note 5); on `ErrConsumerGap` return the error.
   e. The handler for each event of `Process`, in order.
   f. The check of note 6.
   g. `UPDATE event_consumer_cursors SET last_seq = $2, updated_at = now() WHERE name = $1`
      with `NewLastSeq`; anything other than one updated row is an internal error.
   h. Commit through the unit of work; result `RoundProcessed` with `Full` when the batch was
      full.
   Locks and timeouts: the cursor row is the only row the round locks, and `SKIP LOCKED` never
   waits, so the round itself cannot deadlock; a `40P01` or `55P03` raised by handler work is
   retried by the unit of work like any unit (M1-13). The reads hold `ACCESS SHARE` on
   `event_log` until commit, so a concurrent detach by `sync.partition_maintenance` waits up to
   `database.lock_timeout` and tries again on its next run (M1-16); `consumer.batch_size` keeps
   rounds short for that reason. `database.statement_timeout`, `database.lock_timeout` and
   `database.idle_in_tx_timeout` apply through M1-13.
5. Gaps, as the pure function `PlanRound` (Ruling for docs/DECISIONS.md, `D-M2-01-2`):
    - Sequence numbers contiguous from `lastSeq + 1`: process all; `NewLastSeq` is the last one.
    - `lastSeq + 1` below `minRetained` and the first number equal to `minRetained` (events
      removed by the retention of 03 `event_log` "Retention"): process from the first number
      and report `LostFrom = lastSeq + 1`, `LostTo = minRetained - 1`. The round logs at error
      level `event consumer lost events to retention` with `bothy.consumer.name` and
      `bothy.event.seq` (the first number processed) and continues. Reason: the events are gone
      and stalling would not bring them back; a webhook subscription detects its own gap through
      `last_delivered_seq` and is `disabled` with "gap" (11 6); the alarm of note 9 fires long
      before this can happen. `minRetained` is the minimum read of M1-16, read in the same
      transaction only when the first number does not follow `lastSeq`; no partition can be
      detached in between, because the round holds `ACCESS SHARE` on `event_log` (note 4).
    - Any other jump (the first number above `lastSeq + 1` although `minRetained` is at most
      `lastSeq + 1`, a first number other than `minRetained`, or a hole inside the batch):
      `ErrConsumerGap`. 04 3.2 items 3 and 4 rule
      holes out, so a hole is a bug. Unlike the tail's `ErrGap` (M1-16), the worker process does
      not exit: the round rolls back, the cursor never passes the hole, the consumer stalls and
      the alarm reports it.
    - `NewLastSeq` never decreases; an empty `seqs` is not a valid input (the round returns
      `RoundIdle` before).
6. No durable events in a round (Ruling for docs/DECISIONS.md, `D-M2-01-4`). Before step g the
   round asks the appender of `sync/app` (same module, its per-unit `txn.Key` buffer) whether the
   unit buffered events; if so it returns an internal error and the unit rolls back. Reason: the
   consumer would read its own output, and the round would take the `event_seq` counter lock and
   lengthen the serialized commit phase (02 11.7); ADR 0051 lets a round insert jobs only.
7. Failures (Ruling for docs/DECISIONS.md, `D-M2-01-3`). A handler error, a panic in a handler
   (recovered into an error, as 02 14.5 does for jobs), a database error and `ErrConsumerGap`
   roll back the whole round: no job, no cursor move. The runner retries on the next wake or
   poll; no event is ever skipped, so one failing event stalls its consumer and the alarm shows
   it. Log the first failure after a success at error level (`event consumer round failed`,
   attributes `bothy.consumer.name` and `error.type`) and the next success at info level
   (`event consumer recovered`), so a database outage, during which the worker pauses (02 14.4),
   does not flood the log. Consumers are independent: a stalled `webhooks` never holds back
   `push`.
8. Runner and wake (Rulings for docs/DECISIONS.md, `D-M2-01-7` and `D-M2-01-8`).
    - One goroutine per registered consumer; one `LISTEN event_log` connection per process for
      all of them, opened with `pg.DialDedicated(ctx, "bothy-consumer-wake")` (M1-16) and kept
      apart from the tail's connection, so in role `all` the tail's ordered loop never depends on
      consumers. A notification wakes every consumer through a channel with capacity one and a
      non-blocking send; extra signals coalesce. The payload (highest `seq`) is not needed.
    - Each consumer also polls every `consumer.poll_interval`. When the `LISTEN` connection is
      lost, the waker reconnects with exponential backoff starting at `consumer.poll_interval`
      and capped at `database.listen_reconnect_backoff_cap`; polling covers the time in between,
      and nothing is lost because the cursor lives in the database. Extract the listening loop of
      the M1-16 tail into one shared type in `internal/sync/adapters/postgres` instead of copying
      it; the tail tests stay green.
    - After a full batch the next round starts at once (drain). After a short batch, an idle
      round, a locked row or a failed round the consumer waits for the next wake or poll.
    - Time comes from `kernel/clock` and injected timers so unit tests control it.
9. Metrics and alarm (Rulings for docs/DECISIONS.md, `D-M2-01-5` and `D-M2-01-6`). The plan row
   asks for `head_seq` minus `last_seq`; `sync.event_retention` is a time window, so the alarm
   needs the age of the oldest unprocessed event as well.
    - signals.md 4.2, two rows (English, like the table):
      `bothy.event.consumer.lag` | Gauge | `{event}` | `bothy.consumer.name` (`push`,
      `webhooks`) | api, worker: `head_seq - last_seq` per row of `event_consumer_cursors`
      (`head_seq` = highest `seq` in the log) | `bothy_event_consumer_lag`; and
      `bothy.event.consumer.pending_age` | Gauge | `s` | `bothy.consumer.name` | api, worker:
      now minus `occurred_at` of the oldest event with `seq > last_seq`; `0` when the
      consumer is at the head. Reason: `sync.event_retention` is a time window |
      `bothy_event_consumer_pending_age_seconds`.
    - signals.md 5.3, one row: `bothy.consumer.name` | `push`, `webhooks` | name of the
      worker consumer (03 `event_consumer_cursors`). Add the same key to the collector
      allowlist and to `otel/attr` in the same change (M1-10 `TestAttrKeysEqualCollectorAllowlist`;
      the allowlist check of M1-04).
    - Both are observable gauges whose callback calls `Consumers.States` with a deadline of
      `database.statement_timeout`: one read of all rows and the head (`max(seq)`), then per row
      `OldestEventAfter(last_seq)` (`SELECT occurred_at FROM event_log WHERE seq > $1 ORDER BY
      seq LIMIT 1`). Lag is `HeadSeq - LastSeq`, floored at zero (a cursor ahead of the head after
      a restore is M7-04's case); the age is `clock.Now()` minus `OldestPendingAt`, floored at
      zero, and zero without a pending event. Every role (`api`, `worker`, `all`) registers the
      gauges, so a stopped or missing worker still shows a growing lag. A callback error drops the
      data point and never blocks (17 rule 4).
    - Declaration: if `internal/platform/otel/metrics` exists (M1-68 merged), declare both as
      `Spec`s there with the attribute set `sync.ConsumerNames()` and record through its
      wrappers. Otherwise register them in `internal/sync/adapters/otelmetrics/consumers.go` with
      the meter of `internal/platform/otel` and the vendor entry of the Scope section, and name in
      the report that M1-68 moves them into its registry as it does for `bothy.egress.blocked`.
    - Threshold key `consumer.pending_alert_after` (place `Server`, type duration), row in 02 22:
      "age of the oldest unprocessed event of a worker consumer before the alert; strictly
      less than `sync.event_retention`" (pattern of `file.scan.pending_alert_after`
      in 12). Choose the default so that a normal worker restart within `shutdown.total_timeout`
      plus a few `consumer.poll_interval` never reaches it, and record the value and the reason in
      `D-M2-01-6`. Cross check `consumer_alert_below_retention` in `internal/wire/configchecks.go`:
      `consumer.pending_alert_after` strictly below `sync.event_retention`, else the start fails;
      the message names both keys and no value; add the check to 02 13 item 4 "Cross conditions
      today".
    - alerting.md 3.2 row: `BothyEventConsumerLag` | page | oldest unprocessed event of a
      worker consumer older than `consumer.pending_alert_after` |
      `` `consumer.pending_alert_after` `` (exactly this cell, the "equal" form that the
      `alert-thresholds` check of M1-04 reads). Runbook anchor `### bothyeventconsumerlag` in
      section 4: is a `worker` (or `all`) process running; logs `event consumer round failed` and
      `event consumer lost events to retention`; database reachable; a handler that keeps failing
      stalls only its consumer, which catches up by itself after the fix; never move the cursor by
      hand.
    - Rule in `bothy-alerts.yaml`: `max by (bothy_consumer_name)
      (bothy_event_consumer_pending_age_seconds{job="bothy/bothy-server"}) > <default of
      consumer.pending_alert_after in seconds>`, a `for` clause like the neighbouring lag alarms,
      `severity: page`, a summary with `{{ $labels.bothy_consumer_name }}` that names the key and
      no number, `runbook_url: "docs/observability/alerting.md#bothyeventconsumerlag"`. The rule
      file is the one place that holds the number, because Prometheus cannot read the server
      configuration; M1-04's check keeps it equal to the default.
    - Why `page`: when a worker consumer stands, pushes and outgoing webhooks stop, and people
      notice that now (alerting.md 1); a threshold well below the window still leaves the instance
      operator the time the plan row asks for before any event is lost.
10. Lifecycle (02 15.1 step 8, 15.2, 15.3; Ruling for docs/DECISIONS.md, `D-M2-01-9`).
    - Roles `worker` and `all`: after River has started, `Ensure`, then the runner as a
      `lifecycle.Component` named `event_consumers`. Role `api` starts no runner (02 3.2) but
      registers the gauges.
    - Stop step `event_consumers` in `lifecycle.PhaseConsumers`, which M1-11 orders before
      `PhaseJobs`: the runner starts no new round and waits for a running round to commit or roll
      back. 02 15.3 gives this step no own budget: it gets the rest of `shutdown.total_timeout`
      (if the merged `StopStep` needs a positive budget, pass `shutdown.total_timeout` and let the
      runner's rule "own budget or remaining total, whichever is smaller" of M1-11 cut it). When
      the budget runs out, the round's context is cancelled and the round rolls back. Either way
      no job is lost or doubled.
    - An unexpected end of the runner (a bug, never a failing round) goes to
      `lifecycle.Runner.Fail`, so the process exits `1` like after an unexpected stop of the
      River client; add the runner to 02 15.2 next to the River client and the round's panic rule
      to 02 14.5.
11. Configuration. `consumer.batch_size` (int, at least 1), `consumer.poll_interval` (duration,
    above zero) and `consumer.pending_alert_after` (duration, above zero) in
    `internal/wire/configkeys_consumer.go` with `chapter02` and the defaults of 02 22; the cross
    check reads `sync.event_retention`, which M1-16 registered. `database.listen_reconnect_backoff_cap`,
    `database.statement_timeout` and `shutdown.total_timeout` stay as M1-11, M1-13 and M1-16
    registered them; `internal/wire` fills `ConsumerConfig` from `consumer.batch_size`,
    `consumer.poll_interval`, `database.listen_reconnect_backoff_cap` and
    `database.statement_timeout` (`StatesTimeout`). Then `make config-reference` and
    `make config-doc-check`.
12. Queries and the visibility lint. `ConsumerEventsAfter`, `OldestEventAfter` and the head and
    minimum reads read `event_log` without the visible-channel predicate by design: like the log
    tail they read every event, and each consumer job checks `push.deliver` or
    `webhook_out.deliver` per receiver before sending (07 7.4 rule 1 exception list, R-AZ-11). If
    `tools/lint/sql_visibility.yaml` exists, add the consumer reads with category `log_tail` and
    this reason; name them in the `sync` README in any case.
13. Security and privacy. Logs of the frame carry only `bothy.consumer.name`, `bothy.event.seq`
    and `error.type`, never payloads; job arguments pass `jobs.ValidateArgsType` (IDs and
    sequence numbers only). Threat model 5.9, two rows: I | Consumer reads the whole
    event log without a permission filter | jobs check `push.deliver` or `webhook_out.deliver`
    before sending (R-AZ-11); read queries on the exception list (07 7.4) | T-AZ-02; and D |
    Consumer stands still, events fall out of `sync.event_retention` | metric
    `bothy.event.consumer.pending_age`, alert `BothyEventConsumerLag` | T-DS-01.
14. Spec text. 02 11.5 gains: start at the head on first registration (note 3), the gap and
    failure rules (notes 5 and 7), no durable events in a round (note 6), one `LISTEN`
    connection per process (note 8). 03 and 04 stay unchanged: the rules fit their text (03 4.10
    keeps the column default `0`; 04 7.6 is satisfied as written). If the build finds that
    the core contract contradicts one of these rules, stop with `Verdict: BLOCKED`.
15. Test data. Append events through the real unit of work and the `sync` appender of M1-16 with
    the catalogue type `settings.updated` (resource `user`, receive action `self`, an audience
    principal), so the tests keep passing once M1-25 validates types against the catalogue. Jobs
    go through a `jobs.Runtime` in role `api` (insert only, so they stay `available`) with the
    test kind `sync.consumer_probe` (args `{Seq int64; Recipient ids.ID}`, queue `default`,
    no-op `Work`), registered in a test registry; the probe handler inserts two jobs per event
    (Alex and Sam as recipients) so a half-done event would show. Count the probe jobs per `Seq`
    in River's job table in `jobs.schema`, as the M1-14 tests do.
16. Rulings for `docs/DECISIONS.md` section 10, one row each with date, decision, reason and
    place: `D-M2-01-1` rows at the head on first registration (note 3); `D-M2-01-2` retention
    loss skipped and logged, other holes fail the round (note 5); `D-M2-01-3` a failing round
    rolls back whole, no event skipped, edge-triggered logs (note 7); `D-M2-01-4` no durable
    event in a round (note 6); `D-M2-01-5` the two gauges and the attribute, exported by every
    role (note 9); `D-M2-01-6` alarm `BothyEventConsumerLag` with severity `page` on
    `consumer.pending_alert_after` and its default, checked below `sync.event_retention`
    (note 9); `D-M2-01-7` one `LISTEN` connection per process, apart from the tail, with its
    backoff (note 8); `D-M2-01-8` drain after a full batch (note 8); `D-M2-01-9` stop step
    without own budget and a runner failure ends the process (note 10).

## Tests (write them first)

Unit:

- `internal/sync/domain`: `TestPlanRound` with a table: contiguous batch; single event;
  retention loss (first number equals `minRetained`, `LostFrom` and `LostTo` set); retention
  loss claimed but the first number differs from `minRetained` (`ErrConsumerGap`); first number
  jumps although `minRetained` is not above `lastSeq + 1` (`ErrConsumerGap`); hole inside the
  batch (`ErrConsumerGap`); `NewLastSeq` equals the last number processed.
- `internal/sync/domain`: `FuzzPlanRound` (native Go fuzz test whose seeds run in `go test`):
  for any `lastSeq`, increasing `seqs` and `minRetained`, a nil error implies that `Process` is
  contiguous, starts at `lastSeq + 1` or at `minRetained`, and `NewLastSeq` is greater than
  `lastSeq`; no input panics.
- `internal/sync`: `TestConsumerNames` (exactly `push` and `webhooks`; `Valid` rejects others).
- `internal/sync/app`: `TestConsumerRegistration` (duplicate name, invalid name, nil handler and
  registration after `Start` fail); `TestRunnerDrainsFullBatches` (fake round results full,
  full, short: three rounds without waiting); `TestRunnerWaitsAfterShortLockedOrFailedRound`;
  `TestRunnerWakeSignalsCoalesce`; `TestRunnerStopStartsNoNewRound` (fake clock, fake waker).
- `internal/wire`: `TestConfigCheckConsumerAlertBelowRetention` (equal fails, greater fails,
  smaller passes; the message names both keys and contains no value); `TestConfigSchemaBuilds`
  with the three keys.

Integration (PostgreSQL 18 via `pgtest`, roles from `migrate --bootstrap`, runtime as
`bothy_app`, data as in design note 15):

- `TestEventConsumerCursorsTable`: columns, types, nullability and defaults as 03 4.10; primary
  key `name`; inserting each of `sync.ConsumerNames()` works and any other name fails with
  SQLSTATE `23514` (the Go set equals the check); `bothy_app` has `SELECT`, `INSERT`, `UPDATE`
  and no `DELETE`; `TestPrivilegesAfterAllMigrations` of M1-12 still passes.
- `TestConsumerEnsureStartsAtHead`: with events committed before, `Ensure` creates the row at
  `max(seq)`; a second `Ensure` (a second worker) changes nothing; events committed before
  `Ensure` create no job, events after it do.
- `TestConsumerRoundJobsAndCursorCommitTogether`: while the probe handler blocks after inserting
  its jobs, a second connection sees neither the jobs nor the advanced cursor; after the commit
  it sees both.
- `TestConsumerRoundLockedEndsAtOnce`: while worker A's round holds the row, worker B's round
  returns `RoundLocked` within a bound far below `database.lock_timeout` and inserts nothing.
- `TestConsumersTwoWorkersExactlyOnce` (plan: "two workers in parallel: every event creates
  jobs exactly once"): two runners, each with its own pool and waker (two worker processes),
  run while several writers append events in units of one to three; a barrier in the probe
  handler forces at least one overlap, so at least one `RoundLocked` is recorded. At the end
  every `seq` has exactly two probe jobs, there is no job for any other number, and the cursor
  equals the head.
- `TestConsumerCrashMidRoundNoDuplicates` (plan: "a crash in the middle of a round creates no
  duplicates"; ADR 0051: "abort before the commit"), subtests: `backend_terminated` (the handler
  records `pg_backend_pid()`, inserts the jobs of some events of the batch and blocks; the test
  ends that backend with `pg_terminate_backend`), `handler_error`, `handler_panic` and
  `context_cancelled`. Each time nothing of the round is committed; a fresh runner (the
  restarted worker) then reaches the head, and every `seq` has exactly two probe jobs.
- `TestConsumerWakesOnNotify` (plan: "`NOTIFY` and polling wake the consumers"):
  `consumer.poll_interval` from the test config is far above the wait bound; one committed event yields its jobs within the
  bound, so only the notification can have woken the round.
- `TestConsumerWakesOnPoll` (plan, same item): with a waker that never signals, a committed event
  yields its jobs within a small multiple of `consumer.poll_interval` from the test config.
- `TestConsumerListenLossLosesNothing`: the test ends the backend with application name
  `bothy-consumer-wake` several times while writers append; the waker reconnects; every `seq`
  has exactly two probe jobs.
- `TestConsumerRetentionLossSkipsAndLogs`: with a small `sync.event_log_partition_size` from the
  test config, a fake clock moved past `sync.event_retention` and the partition maintenance of
  M1-16, older partitions are dropped while the cursor is below `min(seq)`; the round processes
  from `min(seq)`, returns no error, and one log record `event consumer lost events to retention`
  carries `bothy.consumer.name`.
- `TestConsumerHoleFailsClosed`: a row inserted as owner with a `seq` jump makes the round fail
  with `ErrConsumerGap`; cursor and jobs unchanged; one error log record for repeated failures,
  then `event consumer recovered` after the hole is closed by the test.
- `TestConsumerRoundRejectsAppendedEvents`: a handler that appends a durable event makes the
  round fail; no event, no job, no cursor move.
- `TestConsumersIndependent`: `push` with a handler that always fails and `webhooks` with the
  probe handler: `webhooks` reaches the head, `push` stays; only `push` shows a lag.
- `TestConsumerShutdownFinishesRound`: `Stop` during a blocked round starts no new round; a
  round released within the budget commits; with an expired budget the round rolls back, and a
  restarted runner creates every job exactly once.
- `TestConsumerLagGauges` (plan: "metric of the backlog per consumer"): with an SDK manual
  reader, `bothy.event.consumer.lag` equals `max(seq) - last_seq` per consumer and
  `bothy.event.consumer.pending_age` equals the fake clock minus `occurred_at` of the oldest
  event after `last_seq`; both are zero at the head; the only attribute is
  `bothy.consumer.name`.
- `TestConsumerLagGaugesInApiRole`: role `api` starts no runner, yet a cursor behind the head
  shows in both gauges.
- `internal/wire`: `TestWorkerRoleStartsConsumersAfterRiver`, `TestApiRoleStartsNoConsumers`,
  `TestConsumerStopStepPhase` (step `event_consumers` in `lifecycle.PhaseConsumers`, before the
  River stop); the golden file of txn registrations of M1-13 is unchanged.

Alarm (Prometheus rule unit tests in `infra/observability/prometheus/tests/bothy-alerts.test.yaml`,
run by the operations checks of M1-04 inside `make check`; see the Scope if they are missing):

- `BothyEventConsumerLag` fires for a series above the threshold after its `for`, with the label
  `bothy_consumer_name` and the rendered summary; it does not fire at the threshold. The
  `alert-thresholds` check finds the rule number equal to the default of
  `consumer.pending_alert_after` (plan: "an alert in alerting.md that fires before the backlog
  reaches the window `sync.event_retention`"; the start check holds the key below the window).

There is no contract test (no operation, no event type) and no new end-to-end test; the compose
smoke test of M1-73 keeps passing with the empty consumer list. The plan row names no test IDs
(AG-*, BR-*, AZ-*, SEC-*, G*); its four acceptance items are mapped above.

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./internal/sync/... ./internal/wire/... ./internal/platform/... ./internal/kernel/... ./migrations/... ./cmd/...
make migrations-lock
make migrations-lint
make codegen-check
make arch
make register-check
make config-reference
make config-doc-check
make adr-index
make lint
make docs
make check
```

After committing, `git diff --exit-code docs/operations/configuration.md docs/adr/README.md`
shows nothing after `make config-reference` and `make adr-index`.

## Acceptance criteria

- [ ] Two workers in parallel: every event creates its jobs exactly once, and the cursor ends at
      the head.
- [ ] A crash in the middle of a round (terminated backend, handler error, panic, cancelled
      context) creates no duplicate and loses no event.
- [ ] `NOTIFY` wakes a consumer without waiting for the poll, the poll wakes it without
      `NOTIFY`, and losing the `LISTEN` connection loses nothing (02 11.5).
- [ ] Jobs and cursor commit in one transaction; a locked row ends the round at once.
- [ ] `event_consumer_cursors` matches 03 4.10; a registered consumer gets its row at the head.
- [ ] Events lost to retention are skipped and logged; any other hole and any handler failure
      roll the round back.
- [ ] signals.md has `bothy.event.consumer.lag` (`head_seq` minus `last_seq`) and
      `bothy.event.consumer.pending_age` per consumer, and every role exports both.
- [ ] alerting.md and `bothy-alerts.yaml` have `BothyEventConsumerLag` on
      `consumer.pending_alert_after`, which the start check keeps strictly below
      `sync.event_retention`; the rule test fires above and not at the threshold.
- [ ] 02, ADR 0051, threat model, collector allowlist, dashboard, `sync` README, DECISIONS and
      CHANGELOG are updated; the configuration reference is regenerated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `sync` updated (port `Consumers`, table, round, wake, gaps and failures, gauges, the query exception of note 12, the difference to the agent consumers of `agents`); README of `internal/platform/otel` if it lists the attribute constants.
- Rulings of note 16 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- ADR 0051 extended as in the Scope without changing the decision; a new ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason (for example the move of the two gauges into the M1-68 registry), every difference between the M1 names in this brief and the merged code (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
