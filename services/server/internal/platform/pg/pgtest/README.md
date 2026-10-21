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
- A clone is `CREATE DATABASE ... TEMPLATE` plus the database grants of `pg.BootstrapDatabase`
  (owner and ACL of the database object, which a template copy does not carry), sent over one
  superuser pool of the container without connecting to the clone; the schema grants come with
  the template. Only the first `New` of a migration set waits for the template; clones of a ready
  template run in parallel. `TestCloneHasTheGrantsOfBootstrapDatabase` holds a clone and a
  database bootstrapped directly equal.
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

`New(t, pgtest.Durable())` creates the database on a second container of the test binary that
runs with the PostgreSQL defaults (`fsync`, `synchronous_commit` and `full_page_writes` on)
instead of the speed flags below; it starts with the first such `New`. Load and chaos tests use it,
because ADR 0063 point 2 measures only with the durability settings of production (M1-16,
`perf/event_commit`; M1-72). Normal tests keep the fast container.

M1-72 adds two things for the load and chaos tests of `make perf`:

- `Limits(cpus, memoryBytes)` puts the database on a container started with `docker --cpus` and
  `--memory` (the emulated profile of ADR 0063); each combination of `Durable` and `Limits` is its
  own container of the test binary.
- `DB.Restart(ctx)` restarts the PostgreSQL server of a durable database inside its container:
  it sends `SIGUSR1` to the watchdog, which stops the server with a fast shutdown and starts it
  again. Every connection breaks; the container, its host port, its data, its lease and
  `AutoRemove` stay. `Restart` affects every database of that container and returns once a
  superuser connection works after the server went down.

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
`full_page_writes=off` (except the `Durable` container), and Ryuk is disabled because it would
publish a port beyond loopback (the Ryuk container exposes `8080/tcp`, `defaultReaperPort` in
[testcontainers-go v0.44.0 reaper.go](https://github.com/testcontainers/testcontainers-go/blob/v0.44.0/reaper.go),
retrieved 2026-10-12; switched off with `TESTCONTAINERS_RYUK_DISABLED`,
[garbage collector](https://golang.testcontainers.org/features/garbage_collector/)).

## Cleanup

After the tests `Main` terminates the container and removes the secrets directory. A test binary
that ends without reaching that point (a panicking test, the `-test.timeout` alarm, `SIGINT`,
`SIGTERM`, `SIGKILL`) cannot clean up itself, and Ryuk is off, so the container cleans up after
itself:

- The test binary holds one superuser connection with `application_name` `pgtest-lease` for as
  long as it runs and reopens it when it breaks. Right after opening it, it sets the comment
  `pgtest-lease` on the database `postgres`, which marks the lease as taken.
- The container runs a watchdog (`watchdogScript` in `pgtest.go`) around the image's own entry
  point. It checks every second; once the lease is taken, `leaseMisses` checks in a row without
  the connection stop PostgreSQL. A container whose lease was never taken (the binary died while
  the container started) stops after `leaseStartGrace`, and so does one whose server gives no
  readable answer for `leaseStartGrace` after the lease was taken (a hung server). The script
  waits at most `leaseStopWait` for PostgreSQL to end, then exits, which ends the container.
- The container has `AutoRemove`, so Docker removes it together with its anonymous data volume
  when it stops.

The operating system closes the connection of a process however it ends, so a container
usually outlives its test binary by little more than `leaseMisses` seconds plus the shutdown (at
most `leaseStopWait`); with a hung server it takes up to `leaseStartGrace`. `TestNoContainerSurvivesTheRun` proves it: it
runs the test binary as a child that passes, fails, panics, runs into `-test.timeout` or is
killed with `SIGKILL`, and checks that the container and its volume are gone afterwards; for the
hanging children it first checks that the container of the live run stays longer than the
watchdog needs to decide. `TestWatchdogScript` runs the watchdog script against stub
executables for `psql` and the entry point and covers a held lease, a lost lease, a server that
becomes unreadable after the lease was taken, a lease never taken and `docker stop`.
`TestWatchdogScriptRestartsOnUSR1` covers the in-place restart of `DB.Restart`.

Containers of older `pgtest` versions and of other test helpers that label their containers
`org.bothy.test` but have no watchdog (such as `org.bothy.test=seaweedfs`) lack the label
`org.bothy.test.watchdog=lease`; `Main` removes them, with their volumes, once they are older
than `legacyReapAge`. `go test` stops a binary after 10 minutes by default (`-timeout`: "The default is 10 minutes (10m).", [go test flags](https://pkg.go.dev/cmd/go#hdr-Testing_flags), retrieved 2026-10-13); such a run with
`-timeout 0` or a timeout above `legacyReapAge` may lose its container. Not covered: a Docker
runtime that is paused or shut down while the test binary dies; the watchdog acts only while the
container runs, and a container stopped by a runtime shutdown is left to Docker's `AutoRemove`
handling. To look for or remove containers by hand:

```sh
docker ps -a --filter label=org.bothy.test=pgtest
docker rm -f -v <container id>   # -v removes the anonymous data volume as well
```
