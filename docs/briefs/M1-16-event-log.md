# M1-16: Event log with commit-ordered sequence

Status: ready | Depends on: M1-14, M1-15 | Etappe: M1

## Goal

After this ticket every durable event a use case appends is written by the `sync` phase hook into
the partitioned table `event_log` with a gapless, commit-ordered global `seq` taken from
`sequence_counters('event_seq')`, followed by `pg_notify('event_log', <highest seq>)`; one log
tail per api process reads the log without loss, even when the `LISTEN` connection breaks; a job
keeps partitions ahead and detaches expired ones; one envelope type renders events without JSON
`null` for gateway, REST catch-up and webhooks; and `make perf` measures
`perf.event_commit_min_per_s`. This is B8 and ADR 0009 in code: the base for catch-up, gateway
delivery, rights cache updates and agent consumers.

## Read first

- `docs/spec/04-sync-protokoll.md`, "Ereignisse" ("Zwei Arten", "Umschlag (Envelope)" with the
  payload rules), "Globale, commit-geordnete Sequenz" (all), "Nachholen (Catch-up)" ("Cursor",
  "`resync_required`" for `min_retained_seq`), "Zustellgarantie", "Reihenfolge", "Entscheidungen
  dieses Kapitels" (R-SY-15 to R-SY-17), "Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`, "Grundsätze" (items 8, 13), table `event_log`, "Transaktionsregeln"
  ("Globale Ereignis-Sequenz", "Sperrreihenfolge", "Schwärzen bei Löschung", "Benachrichtigung
  der Gateway-Prozesse"), "Aufbewahrungsklassen" (`event_log`), R-DM-15, R-DM-18,
  "Konfigurationswerte dieses Kapitels" (`sync.event_log_partition_size`).
- `docs/spec/02-architektur.md`, "Ablauf", "In-Tx-Ereignisse", "Ereignis-Pfad" ("Schreiben",
  "Log-Tail je api-Prozess", "Kosten der Serialisierung"), "Hintergrundjobs (River)" (job
  `sync.partition_maintenance`), "Fehler- und Timeout-Strategie" ("Ausfall einer Abhängigkeit"
  row `LISTEN`, "Panics"), "Start" step 8, "Leistungsziele", "Offen" (PollOnly), "Konfigurationswerte
  dieses Kapitels".
- `docs/spec/05-aktionen.md`, "Dauerhafte Ereignistypen und Empfangs-Aktion".
- `docs/spec/06-api.md`, "WebSocket-Ops und Ereignisse" (event table), "Abgleich mit dem
  Kern-Vertrag" K2, K3; `docs/api/openapi.yaml` schemas `EventEnvelope`, `EventActor`,
  `EventScope`, `EventResource`, `ActorKind`, `EventPage`.
- `docs/spec/16-betrieb.md`, "Datenbank" ("Rollen": DDL im Betrieb über `SECURITY DEFINER`);
  `docs/operations/upgrade.md`, "Regeln für Migrationen" (M10).
- `docs/spec/17-observability.md`, "Signale" (correlation and `trace_context`), R-OB-07;
  `docs/observability/signals.md`, "Traces" and "Attribute in Spans und Logs".
- `docs/spec/20-datenschutz.md`, rows for `event_log`.
- ADR 0009; ADR 0007 (rule 4: sequential migration numbers); ADR 0052, 0053 and 0063 (they exist;
  this ticket extends their "Confirmation" with the tests named below); `docs/adr/README.md`.
- What earlier tickets built: `internal/kernel/txn`, `platform/pg` unit of work, `pg.Tx`,
  `txn.Registry` and the golden file (M1-13); `platform/jobs` registry, periodic jobs and
  `Inserter` (M1-14); `instance.Counters` with its phase guard and the static check (M1-15);
  `internal/kernel/eventlog` as declared by M1-05; `platform/lifecycle`; the `perf` frame target in
  the `Makefile` (M1-01).
- `docs/PLAN.md` section "Arbeitsweise je Ticket", P-07, P-09, P-10, P-11, P-12, M1 risk "Durchsatz
  am Zähler"; `docs/quality/gates.md`; `CONTRIBUTING.md`; `docs/quality/red-team-gate.md`.

Table: `event_log`. Job: `sync.partition_maintenance`. No API operation and no action id.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: write each test, run it and see it fail (keep the red output for the report), then
   write the code until it passes, then clean up.
7. No en dash or em dash characters in anything you write (code, comments, docs, commit
   messages). Check every changed file with `grep -nP "[\x{2013}\x{2014}]" <file>`; the result
   must be empty.
8. White label: no company names, company or mail domains, person names, host names, real IP
   addresses or paths under a user home directory. Examples use `example.com` and the names Alex,
   Sam, Robin.
9. Configuration values only through named keys that appear in `docs/operations/configuration.md`
   and the config schema in code (Ruling L4).
10. Sequence by the counter row pattern: lock and advance `event_seq` by the number of events as
    the last step before commit, insert the rows with `last - n + 1 .. last`, read
    `permission_version` after the lock, `pg_notify('event_log', <last>)` with only the number
    (04 "Muster mit Zähler-Zeile", 02 "Ablauf" step 3.2, 03 "Benachrichtigung der
    Gateway-Prozesse"). Transactions without events take neither counter nor `NOTIFY`
    (02 "Kosten der Serialisierung").
11. `event_log` exactly as 03 defines it, partitioned by range of `seq` with
    `sync.event_log_partition_size` numbers per partition, no foreign keys, the listed indexes
    (R-DM-15).
12. A partition is detached and dropped when its largest `occurred_at` is older than
    `sync.event_retention`, never the partition holding the current head (03 `event_log`
    "Aufbewahrung"); `sync.event_log_partitions_ahead` partitions are created ahead by the job every
    `sync.partition_maintenance_interval` and once by `bothy migrate up`, never by a migration in
    operation (upgrade.md M10, 02 "Job-Katalog").
13. One tail per api process with its own, non-pooled connection for `LISTEN event_log`, polling
    every `sync.log_poll_interval`, reconnecting with backoff up to
    `database.listen_reconnect_backoff_cap`, reading `seq > local_head ORDER BY seq LIMIT
    sync.tail_batch_size`, delivering in `seq` order from one goroutine (02 "Log-Tail je
    api-Prozess"); losing the `LISTEN` connection loses nothing.
14. One envelope for gateway, REST and webhooks (ADR 0052, 04 "Umschlag"); no JSON `null`
    (ADR 0053, R-SY-16); delivery fields are never stored (R-SY-17).
15. `correlation_id` is the W3C trace ID of the triggering request; `trace_context` is stored only
    when the request was recorded (03 `event_log`, 17 "Signale", R-OB-07).
16. Payload is a JSON object below `sync.event_payload_max_bytes`, contains no secrets and no JSON
    `null`; larger objects are delivered with `truncated: true` (04 payload rules).
17. `make perf` measures `perf.event_commit_min_per_s` on `perf.reference_profile` and fails below
    the threshold; it is not part of `make check` (P-07, ADR 0063).
18. `event_log.actor_kind` (envelope `actor.kind`; `system` without principal; table check of
    R-DM-29) and `event_log.truncated` (envelope `truncated`; the producer builds the reduced payload)
    as 03 defines them; `receive_action` holds one value per type (03 `event_log`).

## Scope

All Go paths are relative to `services/server/`. Use the M1-13, M1-14 and M1-15 APIs as merged; the
names below match their briefs.

1. Migration (goose, SQL): `event_log` per 03 including the precondition columns; `PARTITION BY
   RANGE (seq)`; primary key `(seq)`; unique `(id, seq)`; indexes `(channel_id, seq)`,
   `(space_id, seq)`, `(audience_principal_id, seq) WHERE audience_principal_id IS NOT NULL`,
   `(resource_kind, resource_id)`, BRIN `(occurred_at)`; no default partition; grants
   `SELECT, INSERT` to `bothy_app` (no `UPDATE`, no `DELETE`; redaction adds its column grant in
   M1-58). Two `SECURITY DEFINER` functions owned by `bothy_owner`, `EXECUTE` revoked from
   `PUBLIC` and granted to `bothy_app`, `search_path` pinned to the domain schema plus `pg_temp`:
   - `sync_event_log_ensure_partitions(p_size bigint, p_ahead int, p_head bigint) RETURNS int`:
     reads the existing bounds from the catalog (`pg_inherits`, `pg_get_expr(relpartbound, oid)`),
     and while the highest upper bound is at most `p_head + p_ahead * p_size` creates the next
     partition `[upper, upper + p_size)` (first partition starts at 1) as a standalone table
     `LIKE event_log INCLUDING DEFAULTS`, adds a `CHECK` matching the bounds, `ATTACH PARTITION`
     (lock `SHARE UPDATE EXCLUSIVE` on the parent; the `CHECK` avoids the validation scan), drops
     the `CHECK`; names `event_log_from_<lower bound zero-padded to 19 digits>`; returns the number
     created; idempotent; uses `format('%I')`.
   - `sync_event_log_drop_expired(p_cutoff timestamptz, p_head bigint, p_lock_timeout interval) RETURNS int`:
     for each partition with upper bound `<= p_head` (the partition holding the head is never
     touched) whose `max(occurred_at)` is older than `p_cutoff` (or that is empty), sets
     `lock_timeout` locally, `DETACH PARTITION` (plain form: takes `ACCESS EXCLUSIVE` on the parent,
     bounded by the lock timeout; `CONCURRENTLY` is not allowed inside a function) and `DROP TABLE`;
     returns the number dropped.
   Verify the lock statements against https://www.postgresql.org/docs/18/sql-altertable.html and
   https://www.postgresql.org/docs/18/ddl-partitioning.html and cite them in the module README.
   Name the file with the next free sequential number of the base branch (ADR 0007 rule 4);
   renumber before the final `make check` if another ticket took it.
2. `internal/kernel/eventlog` (keep what M1-05 declared if compatible; final shape):
   - `type ActorKind string` with `ActorUser`, `ActorAgent`, `ActorApp`, `ActorSystem`.
   - `type Event struct { ID ids.ID; Type string; SchemaVersion int; ActorKind ActorKind; ActorPrincipalID ids.ID; SpaceID, ChannelID, ThreadID ids.ID; ResourceKind string; ResourceID ids.ID; ResourceVersion int64; AudiencePrincipalID ids.ID; ReceiveAction string; Payload []byte; Truncated bool; ClientMsgID ids.ID }`
     (zero values mean "absent"; zero `ID` means the appender generates a UUIDv7).
   - `type Appender interface { Append(ctx context.Context, events ...Event) error }`.
3. `internal/sync` (port):
   - `type Event struct` (a stored row: all columns of `event_log`, zero values for NULL,
     `Payload json.RawMessage`, `Redacted bool` from `redacted_at`).
   - `type DeliveryFields struct { IncludeTraceContext bool; InvocationToken string; Redelivered bool }`.
   - `type Envelope struct` with the members of `EventEnvelope` plus `type` and `payload`, JSON tags
     with `omitzero`; `Seq` and `PermissionVersion` are `*int64` so a durable event with
     `permission_version` 0 still carries the field; nested `EnvelopeActor`, `EnvelopeScope`
     (always present, members omitted), `EnvelopeResource`.
   - `func (e Event) Envelope(d DeliveryFields) Envelope`.
   - `type Sink func(ctx context.Context, batch []Event) error`.
   - `type Tail interface { Run(ctx context.Context) error; Head() int64 }`.
4. `internal/sync/domain`: event validation, partition arithmetic, envelope mapping rules (pure).
5. `internal/sync/app`: the appender (validates, then buffers in a `txn.Key` per unit), the phase
   hook `(*PhaseHook).Run` for phase `sync`, the partition maintenance use case, outgoing ports for
   the writer and the partition functions.
6. `internal/sync/adapters/postgres`: sqlc queries (`queries/`), batch insert (sqlc `:copyfrom` or
   one multi-row insert; pick by `perf` result), `pg_notify`, head and minimum reads, calls of the
   two functions; the tail implementation with a dedicated connection (add
   `pg.DialDedicated(ctx, applicationName string)` to `platform/pg` if M1-12 has none; application
   name `bothy-event-tail`).
7. `internal/sync/adapters/jobs`: args type `PartitionMaintenance struct{}` with kind
   `sync.partition_maintenance`, handler, periodic registration (`Interval =
   sync.partition_maintenance_interval`, `RunOnStart = true`, queue `maintenance`).
8. `internal/wire`: `SetHook(txn.PhaseSync, syncHook.Run)`; the appender as `eventlog.Appender`
   for all modules; job registration; the tail in roles `api` and `all` (start step 8, empty sink
   list today; M1-28 and M1-65 add sinks in this order: rights cache, then hub); regenerate
   `internal/wire/testdata/txn_registrations.golden`.
9. `cmd/bothy migrate up`: after River and goose, call the partition maintenance use case once with
   the configured values so a fresh or upgraded database has partitions before `serve` starts.
10. `perf/event_commit/` (build tag `perf`, run by the `perf` frame target of M1-01;
    `make perf` must include it): see Tests.
11. ADRs 0052, 0053 and 0063 exist: add the envelope tests (`FuzzEnvelopeHasNoJSONNull`,
    `TestEnvelope_FieldsMatchOpenAPI`) to the "Confirmation" of 0052 and 0053 and
    `TestEventCommitThroughput` to 0063 without changing the decisions; then `make adr-index`.
12. Docs: `internal/sync/README.md` (status, ports, table, job, partition rules with PostgreSQL
    references); CHANGELOG. The spec already says what this ticket builds (03 `event_log`, 02
    "Job-Katalog" and "Konfigurationswerte dieses Kapitels", 16 "Rollen", upgrade.md M10); change it
    only if the build proves it wrong, in the same change.

## Out of scope

- Catch-up over REST and the snapshot, `resync_required`, cursor frames (M1-63).
- Gateway, hub, rights cache sinks (M1-64, M1-65, M1-28).
- Redaction on delete (M1-58), idempotency keys (M1-20), worker consumers with
  `event_consumer_cursors` (M2-01, P-13).
- Validating event types and receive actions against the catalogue register (M1-25).
- Metrics `bothy.event.appended`, `bothy.db.event_seq.lock_wait`, `bothy.event.fanout.lag` (M1-68).
- The remaining load and chaos tests (M1-72).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-16-event-log`.
- Conventional Commits with scope (for example `feat(sync): ...`) and DCO sign-off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
  The core contract chapters (00, 03, 04, 05) are read only for this ticket.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end. Test containers and listeners bind `127.0.0.1` and port 0 or a random port (P-09,
  P-10).
- Never read or print secrets. No network calls except package downloads for pinned tools and Go
  modules.

## Design notes

- Columns `actor_kind text NULL CHECK (actor_kind IN ('user','agent','app','system'))` (envelope
  `actor.kind`; NULL means no actor; `system` has no principal) with the table check
  `COALESCE(actor_kind IN ('user','agent','app'), false) = (actor_principal_id IS NOT NULL)`, the
  check `receive_action <> 'self' OR audience_principal_id IS NOT NULL`, and `truncated boolean NOT
  NULL DEFAULT false` (envelope `truncated`), exactly as 03 defines them.
- Append validation (domain, before buffering; violations are kind `internal`, they are
  programming errors): unit present and phase `work` or `rights_check` (`txn.CurrentPhase`);
  `Type` matches `^[a-z][a-z_]*(\.[a-z][a-z_]*)+$`; `SchemaVersion >= 1`; `ResourceKind`
  non-empty; `ReceiveAction` non-empty; `ReceiveAction == "self"` requires
  `AudiencePrincipalID`; `ActorKind` in the set or empty; `user`/`agent`/`app` require
  `ActorPrincipalID`, empty and `system` forbid it; `Payload` is a JSON object, contains no
  `null` token at any depth, and is at most `sync.event_payload_max_bytes`. A producer with a
  larger object builds a reduced payload and sets `Truncated` (03 `event_log`, 04 payload rule 2).
- Correlation (adapter, at append time from the append context): if
  `trace.SpanContextFromContext(ctx)` has a valid trace ID, `correlation_id` is its 16 bytes as
  UUID; `trace_context` is the W3C `traceparent` only if the span context is sampled; without a
  valid span context both are NULL.
- Phase hook `sync`: load the buffer; if empty return without touching counter or `NOTIFY`;
  `last := Counters.Advance(event_seq, n)`; `pv := Counters.Current(permission_version)`; insert
  rows in append order with `seq = last - n + 1 ...`; `SELECT pg_notify('event_log', last::text)`.
  `occurred_at` keeps the column default (`now()`, the transaction time). An insert error with
  SQLSTATE `23514` (no partition for the row) is mapped to kind `unavailable` and logged at error
  level with the seq range, never with payloads.
- Head and minimum: head is `max(seq)` of `event_log` (0 when empty). This equals the committed
  counter because the partition holding the head is never dropped (03 `event_log` "Aufbewahrung";
  it avoids reading `Counters` outside a hook). The minimum retained seq is `min(seq)`; M1-63 uses
  it.
- Tail: at `Run` set `local_head` to the head; open the dedicated connection and `LISTEN
  event_log`; loop on notification or timer (`sync.log_poll_interval`); read batches of
  `sync.tail_batch_size` until a short batch; check that seqs continue `local_head + 1` without
  holes, else return `ErrGap` (fatal: gaps cannot happen by 04 "Muster mit Zähler-Zeile" item 3,
  so a gap is a bug and the process must not deliver further); call the sinks in order; advance
  `local_head`. On `LISTEN` connection loss keep polling through the pool and reconnect with
  exponential backoff starting at `sync.log_poll_interval`, capped at
  `database.listen_reconnect_backoff_cap`. A sink error or `ErrGap` ends `Run` with an error; the
  lifecycle ends the process with a non-zero exit code (02 "Panics": no process without fan-out).
- Envelope mapping: `seq` and `permission_version` always for stored events; `actor` present iff
  `actor_kind` set, `actor.id` iff principal set; `scope` always an object; `resource.version`
  omitted when NULL; `audience: "self"` iff `audience_principal_id` set; `redacted: true` iff
  `redacted_at` set; `truncated: true` iff column true; `trace_context` only with
  `IncludeTraceContext`; `invocation_token` and `redelivered` only from `DeliveryFields`;
  `occurred_at` in RFC 3339 UTC. The Go version of the module supports the `omitzero` tag (Go
  1.24 or later); `kernel/ids.ID` must implement `IsZero()` (add it if missing).
- Partition maintenance use case: `head := max(seq)`; `ensure(sync.event_log_partition_size,
  sync.event_log_partitions_ahead, head)`; `drop_expired(clock.Now() - sync.event_retention, head,
  database.lock_timeout)`; runs in a unit of work (no events appended); time from `kernel/clock` so
  tests move the cutoff instead of rewriting rows.
- Configuration: `sync.event_log_partition_size` (03), `sync.event_retention` (04),
  `sync.event_payload_max_bytes` (04), `sync.log_poll_interval`, `sync.tail_batch_size`,
  `sync.partition_maintenance_interval`, `sync.event_log_partitions_ahead`,
  `database.listen_reconnect_backoff_cap`, `database.lock_timeout` (all 02). Defaults come from the
  chapter tables. Register missing keys in the config schema; `make config-doc-check` green.
- Perf threshold: `perf.*` values are load-test thresholds with place `Lasttest` in 02
  "Konfigurationswerte dieses Kapitels" and are never in the server configuration schema. Read
  `perf.event_commit_min_per_s` and `perf.reference_profile` from that table with a small loader in
  `services/server/perf/internal/report` (untagged, unit tested in `make test`; find the section by
  its title, a missing row or an unparseable value fails). M1-72 extends the same package. No
  threshold literal in the test.

## Tests (write them first)

Integration tests use `pgtest` with roles from `migrate --bootstrap` and run as `bothy_app`.

- `TestAppend_RequiresUnitAndPhase`, `TestAppend_Validation` (table over every rule above,
  including payload with nested `null`, oversized payload, `self` without audience, `system` with
  principal).
- `TestSyncHook_NoEventsTakesNoCounterAndNoNotify`: counter unchanged, a listener receives no
  notification.
- `TestSyncHook_ContiguousSeqInAppendOrder`: two units with 1 and 3 events get `1` and `2..4`;
  rows carry the `permission_version` value at commit; the notification payload is the last seq.
- `TestEventLog_ConcurrentWritersGapless` (ADR 0009 Confirmation): many writers, each unit appends
  1 to 3 events; a test `audit` hook (phase after `sync`) sleeps a random time and randomly returns
  an error to roll back after the counter step; a tail with a collecting sink runs concurrently;
  at the end the collected seqs are exactly `1..committed`, every committed event ID once, no event
  of a rolled back unit, events of one unit contiguous.
- `TestTail_ListenConnectionLossLosesNothing`: terminate the backend with application name
  `bothy-event-tail` via `pg_terminate_backend` while writers run; the tail reconnects; contiguous
  result.
- `TestTail_PollsWithoutNotify`: rows inserted through the appender while `LISTEN` is not yet
  established are delivered within the poll interval.
- `TestTail_StartsAtHead`: events committed before `Run` are not delivered.
- `TestTail_GapIsFatal`: a row inserted by the owner with a seq jump makes `Run` return `ErrGap`.
- `TestPartitions_CreatedAhead`, `TestPartitions_Idempotent`, `TestPartitions_RunAsAppRole`.
- `TestPartitions_DetachedAfterRetention`: with a small partition size from test config, fill
  several partitions, move the fake clock past `sync.event_retention`; old partitions are gone,
  the partition holding the head remains, `min(seq)` moved up.
- `TestAppend_MissingPartitionIsUnavailable`.
- `TestMigrateUp_CreatesInitialPartitions`.
- `TestPartitionMaintenanceJobRegistered`: kind, queue `maintenance`, interval from config, args
  pass `jobs.ValidateArgsType`.
- `TestCorrelation_FromTraceContext`: sampled span -> `correlation_id` and `trace_context`;
  unsampled span -> only `correlation_id`; no span -> neither.
- `FuzzEnvelopeHasNoJSONNull` (native Go fuzz test with seeds that run in `go test`): random
  events and delivery fields; the marshalled envelope decodes without any `null` token.
- `TestEnvelope_PermissionVersionZeroPresent`, `TestEnvelope_DeliveryFieldsOnlyWhenSet`,
  `TestEnvelope_TraceContextOnlyForAgents`.
- `TestEnvelope_FieldsMatchOpenAPI`: the JSON member names of `Envelope` and its nested types equal
  the properties of `EventEnvelope`, `EventActor`, `EventScope`, `EventResource` in
  `docs/api/openapi.yaml` plus `type` and `payload`.
- `TestTxnRegistrationsGolden` (M1-13) passes with the new hook line;
  `TestCountersOnlyUsedByPhaseHooks` (M1-15) passes with the sync hook package.
- Perf (`perf/event_commit`, build tag `perf`): `TestEventCommitThroughput` starts PostgreSQL 18
  through `pgtest`, migrates, creates a scratch table as owner, runs writers (flag
  `-perf.writers`, default `4 * GOMAXPROCS`) for a duration (flag `-perf.duration`) where each unit
  updates one scratch row and appends one event; prints `perf.event_commit_min_per_s measured=<n>
  threshold=<n> cpus=<n>` and fails below the threshold. A second subtest runs the same with an
  idle River worker runtime on the same database and prints its number (evidence for the open
  point PollOnly in 02 "Offen"; both numbers go into the report, not into docs).
  `TestReport_FailsBelowThreshold` (untagged, in `perf/internal/report`): a measured value below
  the threshold fails the comparison, at or above passes; the loader reads the real 02 table.

Commands that must pass:

```sh
cd services/server && go test -race -count=1 ./internal/sync/... ./internal/kernel/eventlog/... ./internal/wire/... ./cmd/...
make migrations-lint
make codegen-check
make arch
make adr-index && git diff --exit-code docs/adr/README.md
make config-doc-check
make perf
make check
```

## Acceptance criteria

- [ ] Under many concurrent writers with random delays and rollbacks the tail sees gapless,
      increasing `seq`, every committed event once and nothing rolled back.
- [ ] Losing the `LISTEN` connection loses nothing; polling covers missing notifications.
- [ ] Partitions are created ahead (also by `migrate up`) and detached after
      `sync.event_retention`; the head partition is never dropped; `bothy_app` stays DML only.
- [ ] Envelopes contain no JSON `null`; their members match `EventEnvelope`; `correlation_id` is
      the trace ID.
- [ ] Transactions without events take no counter lock and send no `NOTIFY`.
- [ ] `make perf` measures `perf.event_commit_min_per_s` and fails below it.
- [ ] ADR 0052, 0053 and 0063 name the tests of this ticket and the ADR index is current; README
      and CHANGELOG updated; `make check` green.

## Definition of done

Code, tests and docs are complete: `internal/sync/README.md` current (status, ports, table, job,
partition rules with PostgreSQL references), doc comment on every public symbol (purpose,
parameters, return value, errors, example where useful), comments explain why. ADRs 0052, 0053
and 0063 extended as above; any further decision goes to `docs/DECISIONS.md`. CHANGELOG entry
under "Unreleased" in `services/server/CHANGELOG.md`. The migration has its final sequential number
before the merge.
`make check` green and `make perf` run with its output in the report. Independent review
(`make review` plus a code review over the diff) and the red team gate
(`docs/quality/red-team-gate.md`) over the whole diff with verdict `Urteil: JA` recorded in the
report; findings fixed with a test each and the delta re-checked. Merge locally into the base
branch only if review and red team are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first test run and both perf numbers with the machine's CPU count and memory), what is not
proven (for example that the threshold holds on `perf.reference_profile` if this machine is not
that profile), and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
