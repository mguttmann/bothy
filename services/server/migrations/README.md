# Package `migrations`

Status: built (M1-12). The SQL migrations of the server, embedded into the binary (`embed.go`,
`var FS embed.FS`) and applied only by `bothy migrate up` (R-AR-16, ADR 0007). Normative are
[ADR 0007](../../../docs/adr/0007-typed-database-access-sqlc-and-migration-tool.md),
[upgrade.md](../../../docs/operations/upgrade.md) and
[16-operations.md](../../../docs/spec/16-operations.md) "Roles".

## Files and numbering

- One file per migration, directly in this directory: `NNNNN_<slug>.sql` with a five digit
  sequential version and a lower-case slug (`^\d{5}_[a-z0-9_]+\.sql$`), goose annotations
  (`-- +goose Up`, optionally `-- +goose Down`), versions contiguous from `00001`.
  `bothy migrate validate` and `make migrations-lint` check this; `-- +goose ENVSUB` is refused.
- A branch uses the next free number of its base branch. Before the merge it merges the base
  branch, renumbers its own new migrations if another ticket took the number, and reruns
  `make check` (ADR 0007 rule 4).
- A merged migration never changes. `make migrations-lock` records the checksum of a new file in
  `services/server/migrations.sha256`; `make migrations-lint` fails on a changed or removed one.

## Grant rule

There are no default privileges. Every migration that creates a table grants exactly what
`bothy_app` needs, normally `SELECT, INSERT, UPDATE, DELETE`, less where the spec says so (for
example `audit_events`), never `TRUNCATE`, `REFERENCES` or `TRIGGER`. Reason: least privilege by
default; a forgotten grant fails loudly in integration tests, a forgotten revoke would be silent.
`00001_base_privileges.sql` grants only `SELECT` on `goose_db_version`, because the server reads
the schema version at start. `TestPrivilegesAfterAllMigrations` holds the rule for every
migration in this directory: it compares the rights of `bothy_app` on each table with the
expectation list `appPrivileges` in `privileges_test.go` (16 "Roles" rule 3), so a migration that
creates a table adds its row there.

## Expand and contract

A migration must keep the previous binary working: version N removes or reinterprets only what
version N-1 no longer reads or writes, renames go through a new column, indexes are built
`CONCURRENTLY` in a file marked `-- +goose NO TRANSACTION`, constraints start `NOT VALID`, and
backfills run as jobs. The full list is upgrade.md section 4 "Rules for migrations
(Expand/Contract)", rules M1 to M12; `bothy migrate` sets `lock_timeout = migrate.lock_timeout`
for every step (rule M6). A generated column states `STORED` (`make migrations-lint`).

## Writing a test

Tests of a migration use `pgtest` (needs a Docker-compatible runtime) from an external test
package:

```go
package migrations_test

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func TestChannelsMigration(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)                 // a clone of the template migrated with migrations.FS
	app := db.Pool(t, pg.RoleApp)       // check what bothy_app may do
	if err := app.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}
```

`pgtest.WithMigrations(fsys)` builds the database from another set (for example the current set
plus one migration under test), `pgtest.WithoutMigrations()` gives a bootstrapped empty database.
Test files must not live in a subdirectory: `make migrations-lint` allows only files directly in
this directory.
