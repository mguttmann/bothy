---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Architecture team
informed: all planning teams
---

# Typed database access (sqlc) and migration tool

## Context and Problem Statement

The data model (03-data-model.md) uses triggers, partitioning, generated columns,
exclusion constraints, partial indexes and separate database roles. Many build sessions write
queries and migrations in parallel in their own worktrees. How does the server access
PostgreSQL in a typed way, and with which tool do migrations run?

## Decision Drivers

* SQL stays visible and checkable (permission filters, lock order, counter pattern from 04).
* Type errors show up at build time, not at runtime.
* Migrations in plain SQL with all PostgreSQL features, without proprietary parts.
* Parallel branches must not block each other on migration numbers.
* Several api nodes must run with the old and the new schema during an upgrade.

## Considered Options

* sqlc with pgx v5 and goose
* sqlc with pgx v5 and golang-migrate
* sqlc with pgx v5 and Atlas
* ORM with its own migration (for example ent or GORM)

## Decision Outcome

Chosen option: "sqlc with pgx v5 and goose", because sqlc generates typed Go code from plain SQL
and goose offers plain SQL migrations with embedding into the binary and a documented way for parallel
development.

Rules:

1. pgx v5 with a pool; sqlc with `sql_package: pgx/v5`; one `sql` entry per module in
   `services/server/sqlc.yaml`, queries under `internal/<module>/adapters/postgres/queries/`.
2. sqlc reads the schema from `migrations/`; it understands goose files and ignores down parts.
   [verified, Q3]
3. Migrations are embedded and run only through `bothy migrate` (02, R-AR-16). Statements that
   do not tolerate a transaction (for example `CREATE INDEX CONCURRENTLY`) are in their own files with
   `-- +goose NO TRANSACTION`. [verified, Q4]
4. Versioning with sequential numbers everywhere, also on branches: a branch takes the
   next free number of the base branch (`NNNNN_<name>.sql`). If another ticket has taken the
   number in the meantime, the branch merged later renumbers its own new migrations
   before its last `make check`. The gate `migrations-lint` rejects timestamp versions in
   every run. Production never uses `-allow-missing`. Reason: the local merge follows only a
   green `make check` of the branch; the gate must therefore already hold the state of the
   base branch on the branch. The hybrid procedure of goose (timestamps on branches, `goose fix`
   on merge) [verified, Q4] does not fit this, because a branch with timestamps would never be green.
5. A migration on the base branch is immutable: the checksum file
   `<module>/migrations.sha256` (format of `shasum -a 256`, outside of `migrations/`) is written
   only by `make migrations-lock`, which never changes an existing line; the gate compares.
6. Expand and contract: a release version may only rely on additive changes;
   removal follows at the earliest one release version later. The server starts with every schema
   version from the smallest one it needs (02, 15.1).
7. Down migrations only for development; a rollback in production is a new migration.
8. `rivermigrate` migrates the River tables in the same command, in its own schema `jobs.schema`.

### Consequences

* Good, because SQL stays readable in the review and permission filters are visible.
* Good, because schema changes without code generation result in build errors.
* Good, because every branch already has, before the merge, exactly the state that the base branch
  has afterwards, and the gate checks this in every run.
* Bad, because parallel branches compete for the same number; the branch merged later
  renumbers. The gate catches duplicate numbers and changed checksums.
* Bad, because dynamic queries (filters with many optional parts) are cumbersome in sqlc;
  several fixed queries or nullable parameters are intended for them.

### Confirmation

* Gate: `sqlc generate` and `git diff --exit-code` (generated code up to date).
* Gate `migrations-lint` (`tools/ci/migrations_lint.py`, build ticket M1-01): only sequential,
  gapless five-digit numbers, no timestamp versions, checksums of merged migrations
  unchanged; plus `goose validate`.
* Integration test: all migrations on an empty PostgreSQL 18.
* Compatibility test: the server of the previous version runs against the schema of the new version
  (expand and contract).
* Table ownership check from 02, section 7.3: the queries of a module name only its own tables.

## Pros and Cons of the Options

### sqlc with pgx v5 and goose

* Good, because sqlc (MIT) v1.31.1 supports `pgx/v5` as a package. [verified, Q1, Q2]
* Good, because goose (MIT) v3.28.0 supports plain SQL, embedded migrations and Go migrations.
  [verified, Q4]
* Good, because goose describes a hybrid procedure with `fix` for teams [verified, Q4]; this
  project does not use it (rule 4), the sequential numbers are enough.
* Bad, because goose generates no schema diff; migrations are written by hand.

### sqlc with pgx v5 and golang-migrate

* Good, because golang-migrate (MIT) v4.20.1 uses plain SQL and is widespread. [verified, Q5]
* Bad, because a documented way for timestamps in branches and sequential numbers on
  `main` is missing from the sources read. [derived]

### sqlc with pgx v5 and Atlas

* Good, because Atlas generates schema diffs.
* Bad, because the Community Edition lacks, among other things, triggers, functions, RLS,
  partitioning and `migrate lint`, and the standard binaries run under a proprietary license.
  [verified, Q6]

### ORM with its own migration

* Good, because simple CRUD queries need less code.
* Bad, because the counter pattern from 04, section 3.2, the lock order and the permission filters
  need exact SQL; an ORM hides exactly these places. [derived]
* Bad, because PostgreSQL features such as exclusion constraints and partitioning usually end up
  as raw SQL in ORM migrations. [assumed]

## More Information

Sources, retrieved 2026-10-10:

* Q1 https://docs.sqlc.dev/en/latest/reference/config.html
* Q2 https://github.com/jackc/pgx (v5.11.0)
* Q3 https://docs.sqlc.dev/en/latest/howto/ddl.html
* Q4 https://github.com/pressly/goose (README, sections "Hybrid Versioning" and "NO TRANSACTION")
* Q5 https://github.com/golang-migrate/migrate
* Q6 https://atlasgo.io/community-edition

Planning basis B2, R-Stack. [02-architecture.md](../spec/02-architecture.md), sections 6, 7.3,
8 and 15.1.
