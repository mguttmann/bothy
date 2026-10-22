# `bothy-backupd`

The process of the Compose service `backup` in the image
[`bothy-postgres`](../../images/bothy-postgres/README.md)
([16-operations.md](../../../../docs/spec/16-operations.md) 2 and 8.2). It is a second program of the
server module, not a command of `bothy`: the server binary with its API role has no business in
the database image (docs/DECISIONS.md D-M7-01-3). Its version is the server version
(`internal/platform/buildinfo`), so one release builds and versions both images from one commit.

## Commands

| Command | Does |
|---|---|
| `bothy-backupd [--pgbackrest PATH] [--stanza NAME] [--retry DURATION] [--config FILE]` | loads the server configuration (keys `backup.*`, `database.*`, `file.*`, `migrate.bootstrap_*`; docs/operations/configuration.md), starts `pgbackrest server` (the TLS server of the repository host) as a child process, runs `pgbackrest --stanza=<stanza> stanza-create` until it succeeds (pgBackRest completes it without a change when the stanza exists and is valid), runs the scheduler, and stops with `SIGTERM` or `SIGINT`. When the TLS server exits on its own, `bothy-backupd` exits with `1` and the restart policy of the container starts it again. A configuration that does not load is exit code `2`. With an empty `backup.offsite.target` the start log says `warning: local backup only: backup.offsite.target is empty`. |
| `bothy-backupd backup <pg_full\|pg_diff\|logical\|objects> [--config FILE]` | runs one kind now (runbook, tests): exit `0` when the run succeeded, `1` when it failed or the kind runs already. |
| `bothy-backupd expire --offsite --credentials DIR [--config FILE]` | the owner's expiry of the off-site target with the separate key in `DIR` ([backup-restore.md](../../../../docs/operations/backup-restore.md) 9). |
| `bothy-backupd restore-test [--config FILE]` | runs the automatic restore test now (runbook, tests): restores the newest `pg_full` or `pg_diff` into `/var/lib/bothy-backup/restore`, starts PostgreSQL there on a Unix socket only, runs the seven checks of [16-operations.md](../../../../docs/spec/16-operations.md) 8.3, clears the directory and writes a `restore_test` row of `backup_runs`; exit `0` when no check failed, `1` otherwise. The scheduler runs the same test on `backup.restore_test.schedule`. |
| `bothy-backupd healthcheck [--addr HOST:PORT] [--server-name NAME] [--ca FILE] [--cert FILE] [--key FILE] [--timeout DURATION]` | a TLS handshake with the pgBackRest server, with the client certificate of `backup`, accepting only a server certificate of the stack's CA for the server name; sends no pgBackRest command. The defaults are the TLS server of the container and the files of `pgbackrest_tls/`. |
| `bothy-backupd version` | prints the version. |

Exit codes: `0` ok, `1` failure, `2` usage or configuration error. `bothy-backupd --help` prints
the defaults.

pgBackRest reads `/etc/pgbackrest/pgbackrest.conf`, which the entry point of the image writes
before `bothy-backupd` starts. The variables of the image (`BOTHY_PGBACKREST_PG_HOST` and the
others of the image README) are no configuration keys; `bothy-backupd` leaves them out of the
configuration, and any other unknown `BOTHY_` variable ends the start (R-OP-04).

## Backup service (M7-02)

The use cases live in the module `status` ([README](../../internal/status/README.md)); this
command loads the configuration with the schema of the server, starts the metrics with
`service.name = bothy-backupd` (`otel.SetupMetrics`, OTLP as configured by the `OTEL_*`
variables) and drives the service through `internal/wire` (`wire.NewBackupDaemon`).

- Scheduler: at the start of every minute (UTC) the kinds due since the previous minute start,
  each in its own goroutine; nothing before the start of the process is caught up. `pg_full` and
  `pg_diff` share one slot, and a full and a differential due together run only the full. After
  each tick it samples `pg_stat_archiver` and the volumes for the metrics.
- Database: the rows of `backup_runs` are written with `database.user` (the Compose service sets
  `bothy_backup` and `db_backup_password`); the manifest queries, `pg_dump` and
  `pg_stat_archiver` use `migrate.bootstrap_user`.
- pgBackRest: the long-running `pgbackrest server` and `stanza-create` get the repo2 options of
  `backup.offsite.target` and the key in `offsite/` as `PGBACKREST_*` variables, plus
  `PGBACKREST_EXPIRE_AUTO=n`, so the service expires repo1 only.
- Restore test (M7-03): `/var/lib/bothy-backup/restore` in the volume `backup-scratch` is the
  scratch directory; the copy is reached over its socket there as `migrate.bootstrap_user`; the
  object sample needs the driver `fs` (the restic snapshots of `file.fs.root`).
- Volumes measured for `bothy.storage.volume.*`: `/var/lib/postgresql` (`pgdata`, read-only),
  `file.fs.root` (`files`, driver `fs`), `/var/lib/pgbackrest` (`pgbackrest-repo`) and
  `/var/lib/bothy-backup` (`backup-scratch`, also the directory of the `pg_dump` file).

## Tests

`go test ./cmd/bothy-backupd/` with a fake `pgbackrest`: the child is stopped on `SIGTERM`,
`stanza-create` is retried until it succeeds, an exiting TLS server fails the command, and
`healthcheck` fails without an answer, against a server of another CA or with another name; the
command lines of `backup` and `expire`, the configuration that must load, the variables of the
image left out of the configuration, and the scheduler loop on a fake clock. The real image is
tested in `services/server/test/postgresimage` (`TestBackupKinds`).
