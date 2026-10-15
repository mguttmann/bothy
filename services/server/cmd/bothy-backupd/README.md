# `bothy-backupd`

The process of the Compose service `backup` in the image
[`bothy-postgres`](../../images/bothy-postgres/README.md)
([16-betrieb.md](../../../../docs/spec/16-betrieb.md) 2 and 8.2). It is a second program of the
server module, not a command of `bothy`: the server binary with its API role has no business in
the database image (docs/DECISIONS.md D-M7-01-3). Its version is the server version
(`internal/platform/buildinfo`), so one release builds and versions both images from one commit.

## Commands

| Command | Does |
|---|---|
| `bothy-backupd [--pgbackrest PATH] [--stanza NAME] [--retry DURATION]` | starts `pgbackrest server` (the TLS server of the repository host) as a child process, runs `pgbackrest --stanza=<stanza> stanza-create` until it succeeds (pgBackRest completes it without a change when the stanza exists and is valid), and stops the child with `SIGTERM` when it receives `SIGTERM` or `SIGINT`. When the TLS server exits on its own, `bothy-backupd` exits with `1` and the restart policy of the container starts it again. |
| `bothy-backupd healthcheck [--addr HOST:PORT] [--server-name NAME] [--ca FILE] [--cert FILE] [--key FILE] [--timeout DURATION]` | a TLS handshake with the pgBackRest server, with the client certificate of `backup`, accepting only a server certificate of the stack's CA for the server name; sends no pgBackRest command. The defaults are the TLS server of the container and the files of `pgbackrest_tls/`. |
| `bothy-backupd version` | prints the version. |

Exit codes: `0` ok, `1` failure, `2` usage error. `bothy-backupd --help` prints the defaults.

pgBackRest reads `/etc/pgbackrest/pgbackrest.conf`, which the entry point of the image writes
before `bothy-backupd` starts.

## Later tickets

Schedules of full and differential backups, `pg_dump`, the object backup, `expire`, manifests
and `backup_runs` (M7-02) and the restore test (M7-03) are use cases of the module `status`
(owner of `backup_runs`, port `BackupRuns`); this command will drive them. It holds no business
logic itself.

## Tests

`go test ./cmd/bothy-backupd/` with a fake `pgbackrest`: the child is stopped on `SIGTERM`,
`stanza-create` is retried until it succeeds, an exiting TLS server fails the command, and
`healthcheck` fails without an answer, against a server of another CA or with another name. The
real image is tested in `services/server/test/postgresimage`.
