# Package `platform/pg`

Status: built (M1-12). Normative are [02-architektur.md](../../../../../docs/spec/02-architektur.md)
"Fristen", "Fehlerarten", "Start" and "Befehle des Binarys",
[16-betrieb.md](../../../../../docs/spec/16-betrieb.md) "Datenbank" ("Rollen", "Einstellungen")
and "Upgrade", [upgrade.md](../../../../../docs/operations/upgrade.md) and ADR 0006 and 0007; where
this file differs, the chapter wins.

## Purpose

The only way the server reaches PostgreSQL 18: one pgx v5 pool per process with the deadlines of
02 "Fristen", the schema check at start, the four database roles and their bootstrap, and the
migrator behind `bothy migrate`. Migrations never run at `bothy serve` (R-AR-16).

## Code

| File | Content |
|---|---|
| `config.go` | `Config`, `Validate`, `WithAppName`, `ForMigrate`, the connection string and session parameters |
| `pool.go` | `Open`, `Pool` (`WaitReady`, `Ping`, `Acquire`, `Begin`, `Stat`, `Close`, `OpenSQL`), `Connect` |
| `timeouts.go` | `Timeouts`, `SetLocalTimeouts` |
| `tracer.go` | otelpgx tracer without query parameters, statement errors redacted |
| `schema.go` | `CheckSchema`, `SchemaTooOldError`, `RequiredSchemaVersion` |
| `roles.go`, `scram.go` | `Role`, `Bootstrap`, `BootstrapRoles`, `BootstrapDatabase`, SCRAM-SHA-256 verifiers |
| `migrator.go` | `Migrator`, `Step`, `MigratorOptions`, `Result`, `MigrationStatus`, `ErrNotSchemaOwner` |
| `validate.go` | `Validate`: names, numbering and goose syntax without a database |
| `pgtest/` | test helper: a disposable PostgreSQL container per test binary ([README](pgtest/README.md)) |

## Pool

- `Open(ctx, cfg, secretsProvider, logger)` builds the pool without connecting; the start calls
  `WaitReady(ctx, startup.dependency_wait)` next, which pings with exponential backoff (100ms to
  2s) and full jitter; when it gives up, the error names host and port and the last cause the
  database reported (for example 28P01 or a missing database), never the password.
- Every connection is configured completely by `Config`: pgx would otherwise read `PG*`
  environment variables, `~/.pgpass`, `~/.pg_service.conf` and `~/.postgresql/` (only
  `cmd/bothy` reads the environment, 02 "Konfigurationsladen" item 5).
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
parameters. A zero field keeps the session value; a negative one is an error. The unit of work of
M1-13 builds on it.

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
verifiers computed in Go (RFC 5802, RFC 7677; 4096 iterations, 16 bytes of salt), so neither the
server log, also with `log_statement = all`, nor `pg_stat_activity` ever shows one. Passwords must
be printable ASCII (docs/DECISIONS.md M1-12). There are no default privileges: each migration
grants what `bothy_app` needs ([migrations/README.md](../../../migrations/README.md)).

## Migrator

`NewMigrator(db, fsys, MigratorOptions{Before, After})` validates `fsys` and builds a goose
provider with the PostgreSQL session locker, so two `bothy migrate up` apply each migration once.
`Up` checks that the session user owns schema `public` (`ErrNotSchemaOwner`), returns
`Result.Newer` without running anything when the database is newer than the binary, and otherwise
runs the before-steps (River, M1-14), goose and the after-steps (instance row, M1-15). `Status`
only reads. `bothy migrate` connects with `Config.ForMigrate(migrate.lock_timeout)`: lock timeout
per step (upgrade.md rule M6), no statement or idle deadline.

## Schema check

`RequiredSchemaVersion(migrations.FS)` is the highest embedded version. `CheckSchema(ctx, pool,
required)` reads `goose_db_version` (as `bothy_app`, which migration 00001 allows) and returns
`*SchemaTooOldError` with the message
`database schema version <have> is older than required <need>; run "bothy migrate up"` when the
database is older; an equal or newer schema passes (expand and contract). Wiring it into
`bothy serve` is M1-11.

## Telemetry

Spans come from otelpgx on the global tracer provider, only below a recording parent span. Query
parameters are never recorded, password literals (`PASSWORD '...'` of the role statements)
become `'[redacted]'`, and the error of a failed statement reaches the span with its SQLSTATE
only, because PostgreSQL messages can quote parameter values. The pinned otelpgx emits
`db.system.name`, `db.namespace`, `db.operation.name`, `db.query.text` (placeholders only),
`db.collection.name` on COPY spans, and also `server.address`, `server.port`, `user.name`,
`pgx.rows_affected`, `pgx.sql_state` and `pgx.prepare_stmt.name`. The retry line of `WaitReady`
carries `server.address`, `server.port` and `error.type` (a SQLSTATE or error class), never an
error text.

## Tests

Integration tests use [`pgtest`](pgtest/README.md) and need a Docker-compatible runtime; without
one the test binary fails (plan P-09). `go test -race ./internal/platform/pg/...`.

## Not covered here

Unit of work and retries (M1-13), the `LISTEN` connection (M1-16), River migrations (M1-14), the
instance row (M1-15), the backup requirement of `migrate` (M7-02), pool metrics (M1-68).
