# Package `migrations`

Status: built (M1-12). The SQL migrations of the server, embedded into the binary (`embed.go`,
`var FS embed.FS`) and applied only by `bothy migrate up` (R-AR-16, ADR 0007). Normative are
[ADR 0007](../../../docs/adr/0007-typisierter-datenbankzugriff-sqlc-und-migrationswerkzeug.md),
[upgrade.md](../../../docs/operations/upgrade.md) and
[16-betrieb.md](../../../docs/spec/16-betrieb.md) "Rollen".

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

## Minimum server version

`embed.go` declares `MinServerVersion`, the oldest server version allowed to run against the
schema of these migrations (18 "Komponenten" rule 6). `bothy migrate up` writes it to
`instance.min_server_version`; a server below it keeps running but is never ready
(`schema_requires_newer_server`). By rule M1 of upgrade.md it is the previous release, a release
that breaks on purpose sets its own version, and before the first release it equals `VERSION`.
Whoever changes the schema checks at release time whether it must rise;
`TestMinServerVersionNotAboveOwnVersion` keeps it valid SemVer and not above the build version.

## Grant rule

There are no default privileges. Every migration that creates a table grants exactly what
`bothy_app` needs, normally `SELECT, INSERT, UPDATE, DELETE`, less where the spec says so (for
example `audit_events`), never `TRUNCATE`, `REFERENCES` or `TRIGGER`. Reason: least privilege by
default; a forgotten grant fails loudly in integration tests, a forgotten revoke would be silent.
`00001_base_privileges.sql` grants only `SELECT` on `goose_db_version`, because the server reads
the schema version at start. `TestPrivilegesAfterAllMigrations` holds the rule for every
migration in this directory: it compares the rights of `bothy_app` on each table with the
expectation list `appPrivileges` in `privileges_test.go` (16 "Rollen" rule 3), so a migration that
creates a table adds its row there.

## Expand and contract

A migration must keep the previous binary working: version N removes or reinterprets only what
version N-1 no longer reads or writes, renames go through a new column, indexes are built
`CONCURRENTLY` in a file marked `-- +goose NO TRANSACTION`, constraints start `NOT VALID`, and
backfills run as jobs. The full list is upgrade.md section 4 "Regeln für Migrationen
(Expand/Contract)", rules M1 to M12; `bothy migrate` sets `lock_timeout = migrate.lock_timeout`
for every step (rule M6). A generated column states `STORED` (`make migrations-lint`).

## Search vectors and the platform function `search_tsquery` (M3-13)

`00035_search_vectors.sql` adds `search_language regconfig NOT NULL DEFAULT 'simple'` and the
`STORED` generated vectors `messages.search_vector` and `threads.title_search` with the
expressions of [03-datenmodell.md](../../../docs/spec/03-datenmodell.md) section 5, and the
platform function `search_tsquery(langs regconfig[], q text) RETURNS tsquery` of
[14-suche.md](../../../docs/spec/14-suche.md) 4.2 (owner `bothy_owner`, `EXECUTE` only for
`bothy_app`). `00036_search_vector_indexes.sql` builds the two GIN indexes `CONCURRENTLY` outside a
transaction and first drops an invalid leftover of an interrupted build.

Rewrite note (RK-38, docs/DECISIONS.md D-M3-13-8): adding a `STORED` generated column rewrites the
table under an exclusive lock. 00035 does it once, before the first server release; existing rows
keep `simple`. Any later change of a vector expression (weights, languages, columns) rewrites
`messages` again and is a release with a maintenance window, whose duration the load test of
M3-16 measures.

Hand-off to the search coordinator (M3-14):

- A NULL result means "no term": `400 validation_failed` with code `empty` on `q` (14 4.1).
- A query of only negations (`-z`) gives `!'z' | !'z' | !'z'`, which matches nearly every row;
  only `search.max_candidates` and `search.statement_timeout` bound it (a spec gap reported by
  M3-13).
- PostgreSQL `text` holds no NUL byte and refuses invalid UTF-8: check `q` before it reaches SQL.
- Call it with the languages of `searchlang.Map.Languages()` cast as `$n::text[]::regconfig[]`;
  `simple` is added by the function. Never log `q` or a notice text (SEC-LOG-4):
  `websearch_to_tsquery` repeats the input in some notices.

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
