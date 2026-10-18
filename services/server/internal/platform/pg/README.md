# Package `platform/pg`

Status: built (M1-12; unit of work M1-13). Normative are [02-architecture.md](../../../../../docs/spec/02-architecture.md)
"Deadlines", "Error kinds", "Start" and "Commands of the binary",
[16-operations.md](../../../../../docs/spec/16-operations.md) "Database" ("Roles", "Settings")
and "Upgrade", [upgrade.md](../../../../../docs/operations/upgrade.md) and ADR 0006 and 0007; where
this file differs, the chapter wins.

## Purpose

The only way the server reaches PostgreSQL 18: one pgx v5 pool per process with the deadlines of
02 "Deadlines", the schema check at start, the four database roles and their bootstrap, and the
migrator behind `bothy migrate`. Migrations never run at `bothy serve` (R-AR-16).

## Code

| File | Content |
|---|---|
| `config.go` | `Config`, `Validate`, `WithAppName`, `ForMigrate`, the connection string and session parameters |
| `pool.go` | `Open`, `Pool` (`WaitReady`, `Ping`, `Acquire`, `Begin`, `Stat`, `Close`, `OpenSQL`, `PGX` for River's driver in `platform/jobs`), `Connect` |
| `unit.go` | `UnitConfig`, `UnitOfWork` (`NewUnitOfWork`, `Do`), `Tx`, `Querier` |
| `querier.go` | `DBTX`, `Pool.Exec`, `Pool.Query`, `Pool.QueryRow` |
| `timeouts.go` | `Timeouts`, `SetLocalTimeouts` |
| `tracer.go` | otelpgx tracer without query parameters, statement errors redacted |
| `metrics.go` | `db.client.operation.duration` of the query tracer with closed attributes, `Pool.RegisterMetrics` (`bothy.db.pool.connections`) (M1-68) |
| `schema.go` | `CheckSchema`, `SchemaTooOldError`, `RequiredSchemaVersion`, `SchemaVersion` |
| `roles.go`, `scram.go` | `Role`, `Bootstrap`, `BootstrapRoles`, `BootstrapDatabase`, SCRAM-SHA-256 verifiers |
| `migrator.go` | `Migrator`, `Step`, `MigratorOptions`, `Result`, `MigrationStatus`, `ErrNotSchemaOwner` |
| `validate.go` | `Validate`: names, numbering and goose syntax without a database |
| `pgtest/` | test helper: a disposable PostgreSQL container per test binary ([README](pgtest/README.md)) |

## Pool

- `Open(ctx, cfg, secretsProvider, logger)` builds the pool without connecting; the start calls
  `WaitReady(ctx, startup.dependency_wait)` next, which pings with exponential backoff (from `backoffBase`
  to `backoffCap` in `pool.go`) and full jitter; when it gives up, the error names host and port and the last cause the
  database reported (for example 28P01 or a missing database), never the password.
- Every connection is configured completely by `Config`: pgx would otherwise read `PG*`
  environment variables, `~/.pgpass`, `~/.pg_service.conf` and `~/.postgresql/` (only
  `cmd/bothy` reads the environment, 02 "Configuration loading" item 5).
- Each new connection resolves `database.password_ref` through the secret provider, so a rotated
  password works after `secrets.cache_ttl` without a restart.
- Session parameters: `statement_timeout`, `lock_timeout`,
  `idle_in_transaction_session_timeout` from the configuration, `application_name`
  (`bothy-<role>`), `default_transaction_isolation = 'read committed'`, `TimeZone = 'UTC'`.
- `Acquire`, `Begin` and `Ping` wait at most `database.acquire_timeout`. An exhausted pool is the
  `kernel/errs` error of code `unavailable` (503) with the internal cause `db_pool_exhausted`, a
  database that does not answer the cause `dependency_unavailable`, an expired request context
  `deadline_exceeded`; a canceled context comes back unchanged. The cause appears only in log and
  metric.
- `database.sslmode` is `disable` or `verify-full` (SEC-TLS-5); a host starting with `/` is a Unix
  socket directory without TLS. `database.sslrootcert` names the CA bundle; empty uses the system
  roots.

## Timeouts

`SetLocalTimeouts(ctx, tx, Timeouts{Statement, Lock, IdleInTx})` overrides the deadlines for one
transaction with `set_config(..., true)`, the function form of `SET LOCAL`, so values travel as
parameters. A zero field keeps the session value; a negative one is an error. The unit of work
applies it to every transaction.

## Unit of work

`NewUnitOfWork(pool, cfg, reg)` implements `kernel/txn.UnitOfWork`, and `platform/pg` is the only
package that begins, commits or rolls back a transaction (02 "Flow", R-AR-06; the Semgrep rule
`bothy-go-tx-only-in-platform-pg` flags `Begin`, `BeginTx`, `pgx.BeginFunc` and
`pgx.BeginTxFunc` elsewhere). `cfg` comes from `wire.UnitConfig`: `database.tx_retry_max`,
`database.statement_timeout`, `database.lock_timeout`, `database.idle_in_tx_timeout`; `reg` from
`wire.NewTxnRegistry`, sealed. `Do` before `Seal` is an internal error.

One attempt of an outermost `Do`:

1. `BEGIN` with `READ COMMITTED` set explicitly, never the database default (ADR 0009).
2. `SET LOCAL` of the three deadlines (`SetLocalTimeouts`).
3. Phase `rights_begin`: the authz hook, only with `txn.RightsChange`.
4. Phase `work`: the function of the use case. Events published here reach their subscribers
   synchronously, in registration order, each in a span `txn.subscribe <module>`.
5. Phase `rights_check`: the authz hook, only with `txn.RightsChange`.
6. Phase `sync`, then phase `audit`: their hooks.
7. `COMMIT`.

A phase without a hook is skipped. Any error rolls back and comes back unchanged (a veto stays
the caller's error); a panic rolls back and is raised again, so the HTTP chain answers `500`. The
rollback runs even when the request context is gone, bounded by `database.statement_timeout`.

Retry: only the outermost unit retries, and only when the error carries SQLSTATE `40P01`
(deadlock) or `55P03` (lock not available) and the context is still live; at most
`database.tx_retry_max` times, so there are `1 + database.tx_retry_max` attempts. Every attempt
re-runs the function with a fresh per-unit state (`txn.Load`, `txn.Store`) and publishes its
events again. When the last attempt fails, `Do` returns the `kernel/errs` error of code
`unavailable` (503) that wraps the database error; the SQLSTATE stays in the log. Other errors are
never retried. The wait at an idempotency key (02 "Idempotency in the Unit of Work") must not surface
as a retryable `55P03`; M1-20 maps it to `request_in_progress`.

Nesting: `Do` inside a unit joins the ambient transaction and runs its function in the current
phase; it never commits, rolls back or retries, and its error goes to the outer `Do`. A nested
`txn.RightsChange` is an internal error unless the outermost unit declared `RightsChange` with
all its principals, because the rights begin hook must lock `permission_version` before any
domain row (03 "Lock order" item 1). One exception (M1-31): an outermost unit opened with
`txn.AllowLateRightsChange()` (only the Idempotency-Key stage, which has written nothing but its
key row) becomes a rights change through its first nested `Do` that declares one; that `Do` runs
the rights begin hook, its function and the rights check hook in this order, and sync and audit
run at the end of the outer unit. A second rights change in the same unit is an internal error.

Repositories write through `Tx(ctx)` (internal error outside a unit) or run statements on
`Querier(ctx, pool)`: the ambient transaction inside a unit, the pool outside. The pool methods
`Exec`, `Query` and `QueryRow` wait at most `database.acquire_timeout` for a connection.

```go
uow := pg.NewUnitOfWork(pool, unitCfg, reg)
err := uow.Do(ctx, func(ctx context.Context) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE grants SET revoked_at = now() WHERE id = $1", id); err != nil {
		return err
	}
	return txn.Publish(ctx, authz.GrantChanged{ID: id})
}, txn.RightsChange(subject))
```

## Roles and bootstrap

| Role | Rights |
|---|---|
| `bothy_owner` | owns the database and schema `public`, runs the DDL of `bothy migrate` |
| `bothy_app` | `CONNECT`, `USAGE` on `public`, and what each migration grants on its tables |
| `bothy_maint` | `CONNECT`, `USAGE` on `public`; trims the audit log (its grants come with the audit tables) |
| `bothy_monitor` | `CONNECT`, member of `pg_monitor` |

`Bootstrap(ctx, superConn, database, passwords)` runs `BootstrapRoles` (cluster-wide: `CREATE ROLE`
or `ALTER ROLE` with `LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`, the
password, `GRANT pg_monitor TO bothy_monitor`) and `BootstrapDatabase` (owner, `REVOKE ALL ... FROM
PUBLIC`, `CONNECT` and `USAGE` grants). It is idempotent, narrows a role widened by hand, and
rotates passwords when the secrets changed. Passwords reach the server only as SCRAM-SHA-256
verifiers computed in Go ([RFC 5802](https://www.rfc-editor.org/rfc/rfc5802),
[RFC 7677](https://www.rfc-editor.org/rfc/rfc7677), retrieved 2026-10-12; `scramIterations`
iterations and `scramSaltBytes` bytes of salt in `scram.go`), so neither the
server log, also with `log_statement = all`, nor `pg_stat_activity` ever shows one. Passwords must
be printable ASCII (docs/DECISIONS.md M1-12). There are no default privileges: each migration
grants what `bothy_app` needs ([migrations/README.md](../../../migrations/README.md)).

## Migrator

`NewMigrator(db, fsys, MigratorOptions{Before, After})` validates `fsys` and builds a goose
provider with the PostgreSQL session locker, so two `bothy migrate up` apply each migration once.
`Up` checks that the session user owns schema `public` (`ErrNotSchemaOwner`), returns
`Result.Newer` without running anything when the database is newer than the binary, and otherwise
runs the before-steps (River, M1-14), goose and the after-steps (instance row, M1-15, built by
`wire.MigrationSteps`). `Status`
only reads. `bothy migrate` connects with `Config.ForMigrate(migrate.lock_timeout)`: lock timeout
per step (upgrade.md rule M6), no statement or idle deadline.

## Schema check

`RequiredSchemaVersion(migrations.FS)` is the highest embedded version. `CheckSchema(ctx, pool,
required)` reads `goose_db_version` (as `bothy_app`, which migration 00001 allows) and returns
`*SchemaTooOldError` with the message
`database schema version <have> is older than required <need>; run "bothy migrate up"` when the
database is older; an equal or newer schema passes (expand and contract). `bothy serve` runs it
as the start component `schema`. `SchemaVersion(ctx, q)` reads the version over any `DBTX`, the
pool or the transaction of a unit of work; the instance step of `bothy migrate up` reads it inside
its unit (M1-15).

## Text search check (M3-13)

`CheckTextSearch(ctx, pool, configs)` runs in the start step `schema` of every role directly after
`CheckSchema` (02 15.1 step 4, docs/DECISIONS.md D-M3-13-6). It answers
`*TextSearchConfigMissingError{Position}` for the first name of `configs` that is no text search
configuration visible on the search path (`pg_catalog.pg_ts_config`, `pg_ts_config_is_visible`),
and `ErrTextSearchCaseFolding` when the `simple` vector of a capital umlaut differs from that of
its small form, which happens under the character classification `C` ([16-operations.md](../../../../../docs/spec/16-operations.md)
7.2). Two short queries on one connection; no error carries a configured name. `wire` turns the
position into `search.languages entry n` or `search.locale_map entry n`.
`TestCheckTextSearch` creates a database with `LC_CTYPE 'C'` inside the pgtest container and
drops it again.

## Telemetry

Spans come from otelpgx on the global tracer provider, only below a recording parent span. Query
parameters are never recorded, password literals (`PASSWORD '...'` of the role statements)
become `'[redacted]'`, and the error of a failed statement reaches the span with its SQLSTATE
only, because PostgreSQL messages can quote parameter values. The pinned otelpgx emits (source
and date in docs/DECISIONS.md M1-12/9)
`db.system.name`, `db.namespace`, `db.operation.name`, `db.query.text` (placeholders only),
`db.collection.name` on COPY spans, and also `server.address`, `server.port`, `user.name`,
`pgx.rows_affected`, `pgx.sql_state` and `pgx.prepare_stmt.name`. The retry line of `WaitReady`
carries `server.address`, `server.port` and `error.type` (a SQLSTATE or error class), never an
error text.

Metrics (M1-68): otelpgx v0.13.0 records `db.client.operation.duration` with the raw first word of
a statement as `db.operation.name`, `pgx.operation.type` and `db.system.name`, and without
`db.collection.name`; the view of the metric registry drops every otelpgx instrument. The query
tracer of this package records the metric itself for every query: `db.operation.name` is the
first keyword (for `WITH` the keyword of the main statement) from the closed list of
`metrics.DBClientOperationDuration`, `db.collection.name` the table the statement reads or writes
from the tables the embedded migrations create (the module register), anything else `other`, and
no attribute without a table. Batches and `COPY FROM` are not measured. `Pool.RegisterMetrics`
observes `bothy.db.pool.connections` with `bothy.pool.state` `idle`, `acquired` and `max` from the
pool statistics; `internal/wire` registers the process pool after `Open`.

## Dedicated connections

`Pool.DialDedicated(ctx, applicationName)` opens one connection with the configuration, session
settings, tracer and password source of the pool, outside the pool and named `applicationName`
(the log tail's `LISTEN` connection, `bothy-event-tail`, M1-16); connecting waits at most
`database.acquire_timeout`, and the caller closes it.

## Tests

Integration tests use [`pgtest`](pgtest/README.md) and need a Docker-compatible runtime; without
one the test binary fails (plan P-09). `go test -race ./internal/platform/pg/...`. The tests of the unit of work register their hooks
and subscribers through the stand-in module `internal/fakemodule`, because the registry rejects
handlers of a platform package.

## Not covered here

The phase hooks of authz, sync and audit (M1-29, M1-16, M1-24), idempotency keys in the unit
of work (M1-20), River migrations (M1-14), the backup requirement of `migrate` (M7-02).
