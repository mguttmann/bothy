# Package `platform/jobs`

Status: built (M1-14). Normative are [02-architecture.md](../../../../../docs/spec/02-architecture.md)
"Background jobs (River)", "Roles of a process", "What never happens in a transaction",
"Panics", "Start" and "Shutdown" (R-AR-09), [upgrade.md](../../../../../docs/operations/upgrade.md)
rule M11 and ADR 0007 rule 8; where this file differs, the chapter wins.

## Purpose

The only package of the server that imports River (`github.com/riverqueue/river`,
`riverdriver/riverpgxv5`, `rivermigrate`; MPL-2.0). The architecture gate enforces it: the vendor
group `river` of `architecture/modules.yaml` is open to `platform-jobs` alone. Nothing of River
appears in the exported API, so a breaking change of River (0.x) touches this package only.

## Code

| File | Content |
|---|---|
| `args.go` | `Args`, `Job`, `Handler`, `HandlerOptions`, the queue names, `ValidateArgsType` |
| `registry.go` | `Registry`, `NewRegistry`, `Register`, `AddPeriodic`, `PeriodicJob`, `ArgTypes` |
| `worker.go` | the wrapper around every handler: span, panic, cancellation, retry rule |
| `runtime.go` | `Config`, `ValidateSchema`, `Role`, `Runtime` (`New`, `Inserter`, `Start`, `Stop`, `Failed`), `Inserter`, `InsertOptions`, `InsertResult` |
| `insert.go` | `InsertTx` in the ambient transaction and the checks all three operations share |
| `unique.go` | `InsertUniqueTx` (River unique jobs by args and state), `PendingTx` and `LatestPendingTx` |
| `migrate.go` | `Migrate`, `MigrationStatus`, `ValidateMigrations`, `CheckEmbeddedMigrations` |
| `jobstest/` | test helper: River's migrations on a `pgtest` database |
| `internal/fakemodule/` | stand-in module with the test arguments and handlers |

## Roles

| Role | River client | Runs jobs | Periodic jobs, leader election, maintenance |
|---|---|---|---|
| `api` | insert-only (`Workers`, no `Queues`), never started | no | no |
| `worker` | full client, started | yes | yes |
| `all` | one full client, started | yes | yes |

Every role builds the same `Registry` in `internal/wire`, so an api process accepts exactly the
kinds a worker runs. A worker whose registry is still empty does not start River (River refuses a
client without workers, and there is nothing to run). The first handler is
`sync.partition_maintenance` (M1-16). `Registry.Periodic` and `Registry.Queue` report what was
registered, for the tests of `internal/wire`.

At the start of `bothy serve` every role checks that River's migrations in `jobs.schema` are
complete (`ValidateMigrations`); otherwise the start ends with the hint to run
`bothy migrate up`. When the River client of a started runtime stops on its own, `Failed`
delivers an error and `internal/wire` ends the process with exit code 1 (02 "Health").

## Queues

| Queue | Content | Parallelism |
|---|---|---|
| `default` | short domain jobs | `jobs.queue.default.max_workers` |
| `files` | scan, thumbnails, transcription | `jobs.queue.files.max_workers` |
| `delivery` | push, webhooks | `jobs.queue.delivery.max_workers` |
| `maintenance` | cleanup, partitions, checks | `jobs.queue.maintenance.max_workers` |
| `privacy` | export and erasure | `jobs.queue.privacy.max_workers` |

`HandlerOptions.Queue` empty means `default`; any other name fails `Register`.

## Rules for job authors

1. Arguments are a struct whose exported fields are only `ids.ID`, `[]ids.ID` or `int64`; an empty
   struct is allowed. Strings, byte slices, maps, interfaces, pointers, nested structs and
   unexported fields fail `Register` (`ValidateArgsType`), so a bad type stops the start. Every ID
   must be a real UUIDv7: arguments that do not survive River's JSON encoding (a zero `ids.ID`)
   fail `InsertTx` and `AddPeriodic`.
2. `Kind()` returns `<module>.<name>` in lower case with underscores (`files.scan`), where
   `<module>` is the module of the argument type's package (the segment after `/internal/`).
   Each kind is registered once.
3. Insert with `Inserter.InsertTx` inside a unit of work, in the work phase (or a rights phase):
   the job becomes visible with the commit and vanishes with a rollback. Outside a unit, in the
   phases `sync` and `audit`, for an unregistered kind or arguments of another type the insert is
   an internal error.
4. State that must exist once per key (an open window, a throttle) is a unique job (M3-19):
   `Inserter.InsertUniqueTx` inserts unless a job of the same kind with equal arguments stands in
   a state that is not finalized (`available`, `pending`, `retryable`, `running`, `scheduled`);
   then it returns `Inserted = false` and the `ScheduledAt` of the standing job. Equal arguments
   are the fields tagged `river:"unique"` when the argument type tags any field, else all fields
   (River's unique jobs by args); queue and time do not count. `Inserter.PendingTx` reads that
   standing job (its `ScheduledAt`, `ok = false` for none) in the ambient transaction with the
   same comparison. `Inserter.LatestPendingTx` reads the latest such job compared on the named
   JSON fields only, for marker jobs that carry their own end time (M3-19). All have the checks of
   `InsertTx` (unit of work, phase, kind, type).
5. `Work` is idempotent, passes its context on and returns `ctx.Err()` when the context ends.
   `nil` marks the job as done; the wrapper turns a `nil` after the end of the context into
   `ctx.Err()`, because a swallowed cancellation would mark an unfinished job as done.
6. A panic is caught: the job is recorded with the error `jobs: job <kind> panicked` (never the
   panic value or the arguments) and retried, the process keeps running. The log line
   `job panicked` carries `bothy.job.kind` and `error.type=panic`.
7. Chapters with their own retry rule set `HandlerOptions.NextRetry` (pause after an attempt) and
   `MaxAttempts`. `HandlerOptions.Timeout` sets River's job timeout of a kind: zero keeps the
   default below, a negative value means no River timeout, for a handler that bounds its own work
   (`files.scan`, M3-04).
8. Periodic jobs (`Registry.AddPeriodic`) need a registered kind and a positive interval; only the
   leader among the workers inserts them.

## Spans

Every attempt runs inside the CONSUMER span `job <bothy.job.kind>` with `bothy.job.kind` and
`bothy.outcome`: `completed` (nil), `cancelled` (the context ended, for example at shutdown or
River's job timeout) or `failed` (any other error or a panic). `snoozed` of the catalogue stays
unused until this interface offers a snooze. The tracer is the global provider's unless `New`
gets one.

## Metrics (M1-68)

Every attempt records `bothy.job.duration` with `bothy.job.kind` (the kinds registered at start)
and `bothy.outcome` (`completed`, `failed`, `cancelled`; `snoozed` stays unused like on the span).
A runtime that works jobs (roles `worker` and `all`) observes `bothy.job.queue` from `Start` to
`Stop`: one grouped count over River's job table in the states `available`, `scheduled` and
`retryable` per collection, bounded by `database.statement_timeout` (`Config.StatementTimeout`),
0 for a registered kind without waiting jobs. `Config.MeterProvider` nil means the global
provider.

## Shutdown

`Stop` stops River with `SoftStopTimeout = shutdown.jobs_soft_timeout`: producers stop fetching,
running jobs may finish; after the soft timeout River cancels the contexts of the jobs still
running. A job cancelled this way returns an error, and River 0.49.0 puts it back as `available`
without counting the attempt, so the next worker runs it again (`JobSetStateInterrupted` for a
soft-stopped job in [internal/jobexecutor/job_executor.go](https://github.com/riverqueue/river/blob/v0.49.0/internal/jobexecutor/job_executor.go),
retrieved 2026-10-13). `internal/wire` registers two stop
steps in phase `jobs`: the soft stop with the budget `shutdown.jobs_soft_timeout`, then the wait
for the cancelled jobs within what `shutdown.total_timeout` leaves after the fixed budgets of the
role; without such a rest there is no second step.

## Migrations

`Migrate(ctx, ownerPool, schema)` runs in `bothy migrate up` before goose (upgrade.md M11):
under a session advisory lock (polled like goose's lock, so a concurrent run waits instead of ending after `migrate.lock_timeout`) it creates `jobs.schema` if needed, takes it away from `PUBLIC`,
grants `bothy_app` `USAGE` on it and, through default privileges of `bothy_owner` and grants on
what exists, `SELECT, INSERT, UPDATE, DELETE` on tables, `USAGE, SELECT` on sequences and
`EXECUTE` on functions; then `rivermigrate` migrates up with `Config.Schema = jobs.schema`. A
second run changes nothing. The lock holds one connection while River migrates on others, so `Migrate`
refuses a pool below two connections (`database.max_conns`); so does `New` for `worker` and `all`,
whose client holds one connection for LISTEN. River waits for a connection without a deadline,
so a smaller pool would hang instead of failing. The schema name reaches the statements only through
`set_config('bothy.jobs_schema', ...)` and `format('%I')`. `MigrationStatus` (for
`bothy migrate status`) only reads; `CheckEmbeddedMigrations` (for `bothy migrate validate`) needs
no database. `jobs.schema` must be an unquoted lower-case identifier, not `public`, not
`information_schema` and not starting with `pg_` (`ValidateSchema`, cross-check `jobs_schema`).
Reserved words are allowed, because River quotes the schema in every statement.

## River defaults

Pinned River version: v0.49.0. These defaults of that version stay in force (02 "Rules for jobs"
rule 5); a job with another need sets a named value of its chapter. Sources, retrieved
2026-10-13: the `...Default` constants in [client.go](https://github.com/riverqueue/river/blob/v0.49.0/client.go),
[internal/maintenance](https://github.com/riverqueue/river/blob/v0.49.0/internal/maintenance) (`JobSchedulerIntervalDefault`,
`JobRescuerRescueAfterDefault`) and the retention defaults of `riversharedmaintenance` in
[rivershared/riversharedmaintenance](https://github.com/riverqueue/river/blob/v0.49.0/rivershared/riversharedmaintenance):

| Setting | Default |
|---|---|
| job timeout (`JobTimeout`) | 1 minute |
| fetch poll interval (`FetchPollInterval`), used when LISTEN/NOTIFY is quiet | 1 second |
| fetch cooldown (`FetchCooldown`) | 100 milliseconds |
| maximum attempts (`MaxAttempts`) | 25 |
| retry backoff (`DefaultClientRetryPolicy`) | about attempt^4 seconds with jitter |
| scheduler interval (scheduled and retryable jobs become available) | 5 seconds |
| rescue of stuck jobs (`RescueStuckJobsAfter`) | 1 hour |
| retention of completed and cancelled jobs | 24 hours |
| retention of discarded jobs | 7 days |

One default is off: the nightly reindexer (`ReindexerSchedule = NeverSchedule`). `bothy_app`
owns no index of River, so a `REINDEX` could only fail (docs/DECISIONS.md M1-14/7).
