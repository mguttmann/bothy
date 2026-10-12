# Package `platform/pg/pgtest`

Status: built (M1-12). Test helper only; no production package imports it (plan P-09, P-10).

## Purpose

Gives every integration test a fresh PostgreSQL 18 database with the schema production gets: one
disposable container per test binary, the four roles bootstrapped with `pg.Bootstrap`, a template
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
  `db_monitor_password` and `db_superuser_password` (mode 0600, generated `EXAMPLE` values).
- `SuperConn(t)` and `SuperConfig()` are the superuser of the container, `ContainerLogs(t)` its
  server log.

`Main(m, pgtest.WithSteps(wire.MigrationSteps))` migrates every template of the test binary with
the steps of `bothy migrate up` as well, so `New` returns a database with the instance row
(M1-15); the test packages that start the server (`internal/wire`, `cmd/bothy`) use it. pgtest
cannot import `internal/wire` itself. All databases of one binary then share the `instance_id`
of their template.

Roles are cluster-wide: a test that changes a role password must not call `t.Parallel` and must
restore the password (see `roles_test.go` in package `pg`).

## Docker requirement

`Main` checks that a Docker-compatible runtime answers (`DOCKER_HOST` is honoured, for example
Colima: [testcontainers-go configuration](https://golang.testcontainers.org/features/configuration/),
retrieved 2026-10-12). Without one it prints
`pgtest: no Docker-compatible runtime reachable (<error>); integration tests need one (plan P-09)`
and the test binary exits with 1; tests never skip. The image is the pinned `Image` in `image.go`
(the same pin as `tools/sonarqube/compose.yaml`, updated by Renovate). The container binds
`127.0.0.1` with a random host port, runs with `fsync=off`, `synchronous_commit=off` and
`full_page_writes=off`, and Ryuk is disabled because it would publish a port beyond loopback (the Ryuk container exposes
`8080/tcp`, `defaultReaperPort` in
[testcontainers-go v0.44.0 reaper.go](https://github.com/testcontainers/testcontainers-go/blob/v0.44.0/reaper.go),
retrieved 2026-10-12; switched off with `TESTCONTAINERS_RYUK_DISABLED`,
[garbage collector](https://golang.testcontainers.org/features/garbage_collector/)).

## Cleanup

`Main` terminates the container after the tests and removes the secrets directory; the container
also has `AutoRemove`. A test binary that was killed leaves its container running. Find and remove
such containers by their label:

```sh
docker ps -a --filter label=org.bothy.test=pgtest
docker rm -f <container id>
```
