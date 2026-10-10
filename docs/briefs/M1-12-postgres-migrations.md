# M1-12: PostgreSQL access, migrations and schema version

Status: ready | Depends on: M1-09 | Etappe: M1

## Goal

After this ticket the server reaches PostgreSQL 18 through one pgx v5 pool with the timeouts of
02 "Fristen", runs embedded goose migrations only through `bothy migrate` (`up`, `status`,
`validate`, `up --bootstrap`), creates its four database roles idempotently with least privilege,
and refuses to start against a schema older than the binary needs. Every later integration test
uses the test helper `pgtest` with a disposable PostgreSQL 18 container on a random loopback port
(ADR 0006, ADR 0007, B3, R-AR-16).

## Read first

- `docs/spec/02-architektur.md`: "Befehle des Binarys" (row `bothy migrate`), "Code-Layout",
  "Transaktionen und Unit of Work" ("Ablauf"; this ticket only provides the timeout helper),
  "Fristen", "Fehlerarten" (`db_pool_exhausted`), "Ausfall einer Abhängigkeit" (row PostgreSQL),
  "Start" (steps 3 and 4), "Konfigurationswerte dieses Kapitels" (`database.*`,
  `startup.dependency_wait`), "Modulregister als Quelle", "go-arch-lint".
- `docs/spec/16-betrieb.md`: "Datenbank" ("Rollen", "Einstellungen"), "Upgrade",
  "Secret-Provider", "Konfigurationswerte dieses Kapitels" (`database.host` to
  `database.max_conns`, `migrate.*`).
- `docs/spec/03-datenmodell.md`: "Grundsätze" (items 1, 11, 12), table `instance` (columns
  `schema_version`, `min_server_version`; created by M1-15), "Technische Tabellen".
- `docs/spec/19-sicherheit.md`: "Transport" (SEC-TLS-5), "Lieferkette, Sicherungen und
  Audit-Integrität" (SEC-AUD-1, SEC-SC-4).
- `docs/operations/upgrade.md`: "Rückweg", "Regeln für Migrationen (Expand/Contract)",
  "Schema-Prüfung beim Start".
- `docs/operations/configuration.md`: "Geheimnisse" (`db_*_password` files).
- `docs/adr/0006-postgresql-ab-tag-1-statt-sqlite.md`,
  `docs/adr/0007-typisierter-datenbankzugriff-sqlc-und-migrationswerkzeug.md`.
- `infra/compose/compose.yaml`: services `postgres` and `migrate`.
- `tools/sonarqube/compose.yaml`: the already pinned `postgres:18.6-trixie@sha256:...` reference.
- `docs/PLAN.md`: "Arbeitsweise je Ticket", P-09, P-10, M1 row M1-12, M7 row M7-02.
- Code you build on: `internal/platform/secrets` (M1-09), `internal/platform/config`, `cmd/bothy`
  (M1-07), `internal/kernel/errs`, `internal/kernel/clock` (M1-05), the `migrations-lint` target
  and `tools/ci/migrations_lint.py` (M1-01).
- Tables: none created by this ticket except the goose version table. Actions: none.

## Fixed requirements that apply

1. No MVP. Build from scratch directly in the final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: a README per module or service, an ADR for every architecture
   decision, a CHANGELOG entry per release, the docs site kept current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: write the failing test, show it red, then make it green, then clean up.
7. No en dash or em dash characters in any file you write.
8. White label: no company, person, host or domain names except `example.com`, people Alex, Sam,
   Robin; no paths under a user home directory; example credentials contain `EXAMPLE`.
9. Configuration only through named keys listed in `docs/operations/configuration.md`.
10. R-AR-16: migrations never run implicitly at `serve`; only `bothy migrate`.
11. 02 "Start" step 4 and the plan row M1-12: the binary knows the smallest schema version it
    needs; against an older database the start aborts with a hint to `bothy migrate up`; a newer
    schema is allowed (expand and contract, rollback by one version).
12. 16 "Rollen": `bothy_owner` owns the schema and runs DDL; `bothy_app` does DML for `api` and
    `worker`; `bothy_maint` trims the audit log; `bothy_monitor` has `pg_monitor`;
    `migrate --bootstrap` creates missing roles idempotently and sets their passwords from the
    secrets.
13. 02 "Fristen" and 16 "Einstellungen": `database.acquire_timeout` (pool overload becomes
    `503 unavailable`; the internal cause `db_pool_exhausted` appears only in log and metric, 02
    "Fehlerarten"), `database.statement_timeout` per transaction via `SET LOCAL`,
    `database.lock_timeout`, `database.idle_in_tx_timeout`; isolation `READ COMMITTED`.
14. upgrade.md rule M6: every migration step runs with `lock_timeout = migrate.lock_timeout`.
15. SEC-TLS-5: PostgreSQL over a Unix socket, with `sslmode=verify-full`, or with
    `sslmode=disable` only inside the internal Compose network without egress and host port.
16. P-09 and P-10: integration tests need a Docker-compatible runtime and fail (never skip)
    without one; containers listen only on `127.0.0.1` on a random port.

## Scope

All Go paths are relative to `services/server/`.

### `internal/platform/pg`

- `type Config struct` filled in `internal/wire` from `database.host`, `database.port`,
  `database.name`, `database.user`, `database.password_ref`, `database.sslmode`,
  `database.sslrootcert` (new), `database.max_conns`, `database.acquire_timeout`,
  `database.statement_timeout`, `database.lock_timeout`, `database.idle_in_tx_timeout`, plus the
  application name `bothy-<role>`.
- `func Open(ctx context.Context, cfg Config, sp *secrets.Provider, log *slog.Logger) (*Pool, error)`.
- `(*Pool).WaitReady(ctx context.Context, maxWait time.Duration) error`: ping with exponential
  backoff and full jitter until success or `maxWait` (`startup.dependency_wait`); the error names
  host and port, never the password.
- `(*Pool).Ping(ctx) error` (bounded by `database.acquire_timeout`; used as readiness check by
  M1-11), `Acquire(ctx)`, `Begin(ctx, pgx.TxOptions)` (both bounded by
  `database.acquire_timeout`; on timeout the `kernel/errs` error of kind `unavailable` with code
  `db_pool_exhausted`), `Stat()`, `Close()`. The raw `*pgxpool.Pool` is not exported.
- `type Timeouts struct { Statement, Lock, IdleInTx time.Duration }` and
  `func SetLocalTimeouts(ctx context.Context, tx pgx.Tx, t Timeouts) error` (M1-13 builds the unit
  of work on it).
- `func CheckSchema(ctx context.Context, p *Pool, required int64) error` returning
  `*SchemaTooOldError{Have, Need int64}` whose message is
  `database schema version <have> is older than required <need>; run "bothy migrate up"`.
- `func RequiredSchemaVersion(fsys fs.FS) (int64, error)`: highest version among embedded
  migrations.
- `type Role string` with constants `RoleOwner = "bothy_owner"`, `RoleApp = "bothy_app"`,
  `RoleMaint = "bothy_maint"`, `RoleMonitor = "bothy_monitor"`.
- `func Bootstrap(ctx context.Context, super *pgx.Conn, database string, passwords map[Role]secrets.Value) error`
  runs `BootstrapRoles` (cluster-wide: roles, attributes, passwords, `pg_monitor`) and then
  `BootstrapDatabase(ctx, super, database)` (database owner, database and schema grants); both
  are exported because `pgtest` reruns the database part for every cloned database.
- `type Migrator` built by `NewMigrator(db *sql.DB, fsys fs.FS, opts MigratorOptions)` with
  `Up(ctx) (Result, error)`, `Status(ctx) ([]MigrationStatus, int64, error)` and the package
  function `Validate(fsys fs.FS) error` (offline). `MigratorOptions{ Before, After []Step }` where
  `type Step interface { Name() string; Run(ctx context.Context, db *sql.DB) error }`. Before-steps
  run before goose (River's `rivermigrate` from M1-14, upgrade.md rule M11), after-steps after it
  (`instance.schema_version` and `min_server_version` from M1-15).

### `internal/platform/pg/pgtest`

- `func Main(m *testing.M) int`: call from `TestMain`; starts one container per test binary on
  first use, terminates it after `m.Run`. `New` fails with a clear message when `Main` is not in use.
- `func New(t testing.TB, opts ...Option) *DB`: a fresh database cloned from a template that is
  bootstrapped and migrated once per container; options `WithMigrations(fs.FS)` (template from
  another set, for schema tests) and `WithoutMigrations()`.
- `(*DB)` gives `Name()`, `Config(role pg.Role) pg.Config` and `SecretsDir()` (a `mktemp`
  directory with the role password files at mode `0600`, so tests open pools through the real
  secrets provider), `SuperConn(t) *pgx.Conn`, `ContainerLogs(t) string`.
- `const Image` in `image.go`: exactly `postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336`
  (same pin as `tools/sonarqube/compose.yaml`) with a `// renovate: datasource=docker depName=postgres`
  comment; add a matching custom regex manager to `renovate.json` (`make lint` validates it).

### `migrations/` (Go package `migrations`)

- `embed.go`: `//go:embed *.sql` as `var FS embed.FS`.
- `00001_base_privileges.sql`: `GRANT SELECT ON goose_db_version TO bothy_app;` (Down: revoke).
  Rationale in D4.
- `README.md`: numbering procedure (D6), grant rule (D4), expand and contract rules (link to
  upgrade.md), how to write a test with `pgtest`.

### `cmd/bothy`

- `bothy migrate up [--bootstrap]`, `bothy migrate status`, `bothy migrate validate`; behaviour in
  D5. Use the command framework and logger that `cmd/bothy` already has.

### Configuration

Register in the schema (if M1-07 did not) with type, default and doc: all `database.*` keys of
02 and 16, `startup.dependency_wait`, `migrate.lock_timeout`, and the keys of D3 (already in the
16 table); regenerate the configuration reference. Do not register
`migrate.require_recent_backup` and `migrate.recent_backup_max_age`: they need `backup_runs` and
apply from M7-02 (16 "Upgrade" item 1).

### Modules register

`architecture/modules.yaml`: vendors `github.com/jackc/pgx/v5/**`, `github.com/pressly/goose/v3/**`,
`github.com/exaring/otelpgx`, `github.com/testcontainers/testcontainers-go/**` for the platform
component; a component for the `migrations` package; regenerate `.go-arch-lint.yml`.

## Out of scope

- Unit of work, phase hooks, retries on `40P01` and `55P03` (M1-13); the `LISTEN` connection
  (M1-16).
- River migrations (M1-14) and the instance row with `schema_version` and `min_server_version`
  (M1-15); you only provide the step hooks.
- The backup requirement of `migrate` (M7-02).
- Wiring the pool and schema check into `bothy serve` and `/readyz` (M1-11).
- Pool metrics `bothy.db.pool.connections` (M1-68).

## Boundaries

- Work in a dedicated git worktree on branch `feat/m1-12-postgres-migrations`.
- Conventional Commits with scope (`feat(pg): ...`, `docs(spec): ...`) and DCO sign-off; squash
  before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the docs named in "Definition of done".
- Follow the host rules supplied by the launcher. Never connect to a database you did not start;
  only the `pgtest` containers. Stop only processes and containers you started, by id.
- Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print real secrets. No network calls except package downloads for pinned modules,
  tools and the pinned PostgreSQL image.
- A missing core-contract piece means `Verdict: BLOCKED`, not a deviation.

## Design notes

D1. Pool.
- pgx v5 `pgxpool`. Connection runtime parameters: `statement_timeout`, `lock_timeout`,
  `idle_in_transaction_session_timeout` from the config, `application_name = bothy-<role>`.
  `SetLocalTimeouts` uses `SELECT set_config('statement_timeout', $1, true), ...` (parameterised,
  equivalent to `SET LOCAL`), so a unit of work can override per transaction.
- Password: a `BeforeConnect` hook resolves `database.password_ref` through the secrets provider
  for every new connection, so a rotated password works after `secrets.cache_ttl` without restart.
- `database.sslmode`: allowed values `disable` and `verify-full` (SEC-TLS-5); anything else is a
  configuration error naming the key. A host that starts with `/` is a Unix socket directory and
  ignores `sslmode`. `database.sslrootcert` is a file path for the CA bundle; empty means the
  system roots. Reason: managed services sign with their own CA.
- Tracing: `otelpgx` tracer on the global tracer provider without query parameters (never enable
  the parameter option); spans carry `db.query.text` with placeholders only. signals.md
  "Datenbank" asks to check the attribute names of the installed version: if the pinned `otelpgx`
  emits names other than `db.system.name`, `db.namespace`, `db.operation.name`,
  `db.collection.name`, `db.query.text`, use its option for current semantic conventions if it has
  one; otherwise list the differences in the report (the source-side span allowlist of M1-10 drops
  unknown keys, so nothing leaks, but the spans lose those fields).

D2. Schema version check.
- Required version = highest embedded migration version (each binary needs all of its own
  migrations; older binaries run against newer schemas by the expand and contract rule).
- `CheckSchema` reads the goose version table (`goose_db_version`, default name). Lower than
  required: `*SchemaTooOldError`. Equal or higher: nil.
- The start aborts on this error (02 "Start" step 4, 16 "Upgrade" item 4, upgrade.md
  "Schema-Prüfung beim Start" item 1); `schema_requires_newer_server` is the readiness case of
  M1-15 (reason in 02 "Start").

D3. Configuration keys (16, table "Konfigurationswerte dieses Kapitels"; already there).

| Key | Default | Meaning |
|---|---|---|
| `database.sslrootcert` | empty | CA bundle file for `verify-full`; empty uses the system roots |
| `migrate.bootstrap_user` | `postgres` | superuser login used only by `migrate up --bootstrap` |
| `migrate.bootstrap_password_ref` | `file:<secrets.dir>/db_superuser_password` | its password |
| `migrate.owner_password_ref` | `file:<secrets.dir>/db_owner_password` | password set for `bothy_owner` |
| `migrate.app_password_ref` | `file:<secrets.dir>/db_app_password` | password set for `bothy_app` |
| `migrate.maint_password_ref` | `file:<secrets.dir>/db_maint_password` | password set for `bothy_maint` |
| `migrate.monitor_password_ref` | `file:<secrets.dir>/db_monitor_password` | password set for `bothy_monitor` |

Reason: the spec names the role passwords and the superuser but no keys; the defaults match the
secret catalog and the `migrate` subpath, so Compose needs no new variables.

D4. Bootstrap and privileges (16 "Rollen" states these rules).
- Runs as `migrate.bootstrap_user` against `database.name` (the database must exist; Compose
  creates it with `POSTGRES_DB`; a missing database is an error naming it).
- Per role: `CREATE ROLE` if missing, else `ALTER ROLE`; always `LOGIN NOSUPERUSER NOCREATEDB
  NOCREATEROLE NOREPLICATION NOBYPASSRLS` and the password. The password is sent only as a
  SCRAM-SHA-256 verifier computed in Go (RFC 5802, RFC 7677; format
  `SCRAM-SHA-256$<iterations>:<salt>$<StoredKey>:<ServerKey>`), so the plaintext never reaches the
  server, its logs or `pg_stat_activity`. Statements use `pgx.Identifier` for names and a quoted
  literal only for the verifier.
- `GRANT pg_monitor TO bothy_monitor`.
- `ALTER DATABASE <name> OWNER TO bothy_owner`; `REVOKE ALL ON DATABASE <name> FROM PUBLIC`;
  `GRANT CONNECT ON DATABASE <name> TO bothy_app, bothy_maint, bothy_monitor`.
- In the database: `ALTER SCHEMA public OWNER TO bothy_owner`; `REVOKE ALL ON SCHEMA public FROM
  PUBLIC`; `GRANT USAGE ON SCHEMA public TO bothy_app, bothy_maint`.
- Domain tables live in schema `public`, together with `goose_db_version` (03 "Technische
  Tabellen").
- No default privileges. Every migration that creates a table grants exactly what `bothy_app`
  needs (normally `SELECT, INSERT, UPDATE, DELETE`; less where the spec says so, for example
  `audit_events`). Reason: least privilege by default; a forgotten grant fails loudly in
  integration tests, a forgotten revoke would be silent. `00001_base_privileges.sql` grants only
  `SELECT` on `goose_db_version` because the app reads the version at start.
- Managed services: the OpenTofu module creates roles with the same names and runs the same SQL;
  `--bootstrap` is then not used (16 "Rollen").

D5. Commands.
- `migrate up`: if `--bootstrap`, resolve the five passwords of D3 and run `Bootstrap` (separate
  connection, closed afterwards). Then connect as `database.user` with
  `database.password_ref` (Compose sets `bothy_owner`), wait up to `startup.dependency_wait`,
  verify that `current_user` owns schema `public` (else exit 1: `migrate must run as the schema
  owner; set database.user`), run before-steps, goose up, after-steps. Session parameters for the
  migration connections: `lock_timeout = migrate.lock_timeout`, `statement_timeout = 0`,
  `idle_in_transaction_session_timeout = 0`. Concurrency: goose session locker
  (`lock.NewPostgresSessionLocker`), so two `migrate up` runs apply each migration once. If the
  database version is higher than the highest embedded version, print
  `schema is newer than this binary; nothing to do` and exit 0 without calling goose (upgrade.md
  "Rückweg"). Output: one line per applied migration (`applied <version> <name>`) and a summary.
- `migrate status`: one line per known migration `<version> <name> <applied|pending> <applied_at>`
  and `database version <n>, binary requires <m>, pending <k>`; exit 0.
- `migrate validate`: offline; parses every embedded file with goose, checks the name pattern
  `^\d{5}_[a-z0-9_]+\.sql$`, versions contiguous from 1 without duplicates; exit 0 or 1.
- Exit codes: `0` success, `1` failure, `2` usage error (reuse codes `cmd/bothy` already defines).
- The goose provider gets a `*sql.DB` from `pgx/v5/stdlib.OpenDBFromPool`; verify the provider
  and locker API names against the pinned goose version.

D6. Numbering on branches (ADR 0007 rule 4): a branch uses the next free sequential number of the
base branch; before the local merge the session merges the base branch, renumbers its own new
migrations if another ticket took the number, and reruns `make check`. This ticket's migration is
`00001`.

D7. `pgtest`.
- testcontainers-go with the pinned image, Ryuk disabled (its port would be published beyond
  loopback), port binding `127.0.0.1` with a random host port set through the host config, label
  `org.bothy.test=pgtest`, `AutoRemove`; termination in `Main`.
- Server flags for speed in tests only: `fsync=off`, `synchronous_commit=off`,
  `full_page_writes=off`.
- Bootstrap uses `pg.Bootstrap` with generated `EXAMPLE` passwords, then migrates the template as
  `bothy_owner` through `Migrator`, so the test path equals production.
- Each `New` creates `CREATE DATABASE <random> TEMPLATE <template> OWNER bothy_owner` and runs
  `BootstrapDatabase` again (database grants are not copied from a template); `t.Cleanup` drops it
  with `DROP DATABASE IF EXISTS <name> WITH (FORCE)` (a test may already have dropped it).
- No Docker runtime: `Main` prints `pgtest: no Docker-compatible runtime reachable (<error>); integration tests need one (plan P-09)`
  and returns exit code 1; never skip.

## Tests (write them first)

Integration tests use `pgtest`; each test package calls `pgtest.Main` from `TestMain`.

- `TestMigrateEmptyDatabase`: fresh database without migrations; `Up` applies all; second `Up`
  applies nothing and returns the same version.
- `TestMigrateConcurrentRunsApplyOnce`: two `Up` in parallel; each version applied once.
- `TestMigrateNewerSchemaIsNoop`: database migrated with a test FS that has one more migration;
  `Up` with the real FS prints the notice and changes nothing.
- `TestBootstrapIdempotent`: run twice; all four roles exist with the attributes of D4; each role
  logs in with its password; changing a password and rerunning rotates it (old fails, new works).
- `TestBootstrapNeverSendsPlaintext`: container with `log_statement=all`; bootstrap with canary
  passwords; container logs contain no canary.
- `TestPrivilegesAfterAllMigrations`: for every table in `public` except `goose_db_version`:
  owner `bothy_owner`, `bothy_app` has `SELECT`, `bothy_app` has no `TRUNCATE`, `REFERENCES` or
  `TRIGGER`, `PUBLIC` has nothing; `goose_db_version` is `SELECT` only for `bothy_app`;
  `bothy_app` cannot `CREATE TABLE` in `public`. This test lives in the `migrations` package so
  every later migration is checked.
- `TestCheckSchemaTooOld`: template from a test FS with two migrations, database at version 1,
  `CheckSchema(required=2)` returns `*SchemaTooOldError` with the hint text; equal and higher
  return nil.
- `TestPoolTimeouts`: `SHOW statement_timeout` and friends equal the config; `SetLocalTimeouts`
  inside a transaction changes them and they are back after commit; `pg_sleep` longer than the
  statement timeout fails with SQLSTATE `57014`.
- `TestPoolAcquireTimeout`: `max_conns = 1`, one connection held, `Acquire` returns the
  `unavailable` error with code `db_pool_exhausted` after about `database.acquire_timeout`.
- `TestPoolPasswordRotation`: rotate the role password and the secret file; after the cache TTL
  (fake clock) a new connection succeeds.
- `TestWaitReadyGivesUp`: closed `127.0.0.1` port; returns an error after `maxWait`; message
  contains no password.
- `TestSSLModeValidation` (unit): `disable`, `verify-full` accepted; `require`, `prefer` rejected.
- `TestDBSpansHaveNoParameters`: span recorder; query with a canary parameter; canary absent.
- `TestPgtestBindsLoopbackOnly`: inspect the container; every port binding has host IP
  `127.0.0.1`.
- `TestPgtestImagePinned` (unit): `Image` matches `^postgres:18\.[0-9]+-[a-z]+@sha256:[0-9a-f]{64}$`.
- `TestValidate` (unit): good FS passes; gap, duplicate, bad name and timestamp version fail.
- Command tests through the `cmd/bothy` entry point: `migrate status` output format,
  `migrate up` as `bothy_app` fails with the schema owner message.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/platform/pg/... ./migrations/... ./cmd/bothy/...
make migrations-lint
make arch
make check
```

## Acceptance criteria

- [ ] An empty PostgreSQL 18 database is migrated; a second run changes nothing.
- [ ] `--bootstrap` creates `bothy_owner`, `bothy_app`, `bothy_maint`, `bothy_monitor`
      idempotently; passwords never reach the server in plaintext.
- [ ] A start check against an older schema fails with the hint to `bothy migrate up`; a newer
      schema passes.
- [ ] Pool timeouts and the acquire timeout behave as in 02 "Fristen".
- [ ] Without a Docker runtime the integration tests fail with the P-09 message instead of
      skipping; containers bind only `127.0.0.1` on a random port.
- [ ] The privilege invariant test passes and runs for every future migration.
- [ ] `make migrations-lint`, `make arch` and `make check` are green; module coverage at or above
      `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests; doc comment on every exported symbol.
- READMEs: `internal/platform/pg/README.md` (pool, timeouts, roles, bootstrap, migrator steps,
  schema check), `internal/platform/pg/pgtest/README.md` (usage, Docker requirement, cleanup and
  the label for leaked containers), `migrations/README.md`.
- Spec: 16 "Rollen", "Upgrade" and "Konfigurationswerte dieses Kapitels", upgrade.md and ADR 0007
  already state D2 to D4 and D6; change them only if the build proves a statement wrong. Add
  `--bootstrap` to 02 "Befehle des Binarys".
- `docs/DECISIONS.md`: anything decided here that the spec does not state (for example the SCRAM
  iteration count).
- Configuration reference regenerated without diff.
- `services/server/CHANGELOG.md` entry under "Unreleased".
- No new ADR: ADR 0006 and 0007 decide the stack; the rulings go to DECISIONS.md.
- `make check` green; independent review and red team (`docs/quality/red-team-gate.md`) with
  verdict `JA` recorded; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs), what is not
proven (for example `verify-full` against a real managed service), follow-ups that could not be
done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or
`exit=1 <reason>`.
