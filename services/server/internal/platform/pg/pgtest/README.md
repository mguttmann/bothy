# Package `platform/pg/pgtest`

Status: built (M1-12). Test helper only; no production package imports it (plan P-09, P-10).

## Purpose

Gives every integration test a fresh PostgreSQL 18 database with the schema production gets: one
disposable container per test binary, the five roles bootstrapped with `pg.Bootstrap`, a template
migrated once per migration set with `pg.Migrator` as `bothy_owner`, and a clone of that template
per test.

## Usage

```go
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func TestSomething(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)                      // server migrations
	pool := db.Pool(t, pg.RoleApp)           // through the real secret provider
	// pgtest.New(t, pgtest.WithoutMigrations()) or pgtest.WithMigrations(fsys)
}
```

- `New(t, opts...)` returns a database owned by `bothy_owner` with the grants of
  `pg.BootstrapDatabase`; `t.Cleanup` drops it (`DROP DATABASE IF EXISTS ... WITH (FORCE)`).
  `New` fails the test when `Main` is not in use.
- `Config(role)` is the connection configuration of a role (127.0.0.1, the mapped port, `sslmode
  disable`, generous test deadlines); change fields and open it with `OpenPool(t, cfg)`.
- `SecretsDir()` holds `db_owner_password`, `db_app_password`, `db_maint_password`,
  `db_monitor_password`, `db_backup_password` and `db_superuser_password` (mode 0600, generated `EXAMPLE` values).
- `SuperConn(t)` and `SuperConfig()` are the superuser of the container, `ContainerLogs(t)` its
  server log.

`Main(m, pgtest.WithSteps(wire.MigrationSteps))` migrates every template of the test binary with
the steps of `bothy migrate up` as well, so `New` returns a database with the instance row
(M1-15); the test packages that start the server (`internal/wire`, `cmd/bothy`) use it. pgtest
cannot import `internal/wire` itself. All databases of one binary then share the `instance_id`
of their template.

`New(t, pgtest.Durable())` creates the database on a second container of the test binary that
runs with the PostgreSQL defaults (`fsync`, `synchronous_commit` and `full_page_writes` on)
instead of the speed flags below; it starts with the first such `New`. Load and chaos tests use it,
because ADR 0063 point 2 measures only with the durability settings of production (M1-16,
`perf/event_commit`; M1-72). Normal tests keep the fast container.

M1-72 adds two things for the load and chaos tests of `make perf`:

- `Limits(cpus, memoryBytes)` puts the database on a container started with `docker --cpus` and
  `--memory` (the emulated profile of ADR 0063); each combination of `Durable` and `Limits` is its
  own container of the test binary.
- A durable container publishes a free loopback port picked first, as
  `127.0.0.1:<port>:5432` (retried on a bind conflict), and is not removed automatically, so
  `DB.Restart(ctx)` can stop and start it on the same port with its data; `Restart` affects every
  database of that container and returns once a superuser connection works.

Roles are cluster-wide: a test that changes a role password must not call `t.Parallel` and must
restore the password (see `roles_test.go` in package `pg`).

## Docker requirement

`Main` checks that a Docker-compatible runtime answers (`DOCKER_HOST` is honoured, for example
Colima). Without one it prints
`pgtest: no Docker-compatible runtime reachable (<error>); integration tests need one (plan P-09)`
and the test binary exits with 1; tests never skip. The image is the pinned `Image` in `image.go`
(the same pin as `tools/sonarqube/compose.yaml`, updated by Renovate). The container binds
`127.0.0.1` with a random host port, runs with `fsync=off`, `synchronous_commit=off` and
`full_page_writes=off` (except the `Durable` container), and Ryuk is disabled because it would publish a port beyond loopback.

## Cleanup

`Main` terminates the containers after the tests and removes the secrets directories; a container
with the speed flags also has `AutoRemove`. A test binary that was killed leaves its container running. Find and remove
such containers by their label:

```sh
docker ps -a --filter label=org.bothy.test=pgtest
docker rm -f <container id>
```
