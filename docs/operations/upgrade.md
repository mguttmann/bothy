# Upgrade

Status: draft M0. Guide to [16-operations.md](../spec/16-operations.md) 10 and
[18-release.md](../spec/18-release.md). Applies to instance operators (procedure) and to developers
of migrations (rules in section 4).

## 1. Before the upgrade

1. Read the release notes: the section "Upgrade notes" says whether manual work is needed; the
   section "Migrations" names expand or contract steps and their expected duration on large tables.
2. Only one major version at a time. Within a major version, an instance may skip versions; the
   migrations run in order.
3. Check the signature ([release.md](release.md) 5).
4. Backup: `migrate` requires a successful backup younger than
   `migrate.recent_backup_max_age` anyway. Before large releases, also trigger a full backup
   by hand.
5. Check the minimum app versions: if the release raises `client.min_version.<platform>`, distribute
   the apps via the channels first.

## 2. Procedure in the Compose stack

```
# .env: set BOTHY_VERSION to the new version
docker compose pull
docker compose up -d
```

Compose replaces `migrate` first; only after exit 0 do `api` and `worker` restart. During the
restart, clients are briefly disconnected; they receive `4013 server_restart` and continue with
`resume`. Auxiliary services (collector, Prometheus and so on) follow their own version variables.

Check: `docker compose ps`, `/readyz`, status page, SLO dashboard, version in the admin console.

## 3. Rollback

1. One release back: set `BOTHY_VERSION` to the previous version, `docker compose up -d`. The schema
   stays; the expand/contract rule guarantees that the code of N-1 runs with it. `migrate`
   of version N-1 recognizes the newer but compatible schema and exits without a change.
2. More than one release back: only by a restore to a point in time before the upgrade
   ([backup-restore.md](backup-restore.md) 5.2). Data after this point in time is lost.

## 4. Rules for migrations (Expand/Contract)

| No. | Rule | Reason |
|---|---|---|
| M1 | Version N may only remove or reinterpret what version N-1 no longer reads or writes. | Rollback by one version without reverting the database. |
| M2 | Never rename directly: new column (expand), write both (N), read only the new one (N+1), remove the old one (N+2). | M1. |
| M3 | New columns allow `NULL` or have a default without rewriting the table. | No long lock on large tables. |
| M4 | Indexes with `CREATE INDEX CONCURRENTLY` in a migration without a transaction (goose: `-- +goose NO TRANSACTION`). | Writes continue. |
| M5 | New constraints first as `NOT VALID`, `VALIDATE CONSTRAINT` in a later migration. | Check without a write lock. |
| M6 | Every step sets `lock_timeout` to `migrate.lock_timeout` and would rather abort than wait. | A waiting lock blocks all following requests. |
| M7 | Never backfill data in the migration, but as a background job in batches. | Short transactions; progress visible. |
| M8 | Enumerations are `text` with `CHECK`; new values are additive (R-DM-02). Code of N-1 must tolerate unknown values. | Rollback. |
| M9 | Event payloads change only additively (`schema_version`, [04-sync-protocol.md](../spec/04-sync-protocol.md) 2.2). | Old clients keep reading. |
| M10 | A job creates the partitions of the event log in advance, never a migration during operation. `bothy migrate up` calls the same idempotent routine after the migrations so that a fresh database has partitions ([16-operations.md](../spec/16-operations.md) 7.1). | No DDL lock in the hot path. |
| M11 | Migrations of River (jobs) run in the same `migrate` run, in a fixed order before our own. | One start step, one result. |
| M12 | No down migrations in production. A new migration corrects errors. | Down migrations are rarely tested and often delete data. |

A release that changes the expression of the search vector `messages.search_vector` rewrites
`messages` ([14-search.md](../spec/14-search.md) section 8): that is the known exception to rule M3
and needs a maintenance window. The release notes name its
expected duration under "Migrations" (section 1 item 1), computed from the values per million
messages of the search load test (`make bench-search`; `rewrite_seconds_per_million` and
`index_build_seconds_per_million` under `extra.search.rewrite` in `.cache/perf/results.json`)
times the number of messages of the instance in millions.

Check in the gate (ticket): every migration runs against the schema of the previous version with
test data; then the test suite of the previous version runs against the new schema (N-1
compatibility).

## 5. Schema check at startup

1. Every binary version knows the smallest migration it needs (the highest embedded one).
   If the applied migration is smaller, the startup aborts with a reference to `bothy migrate up`
   (exit code `1`, [02-architecture.md](../spec/02-architecture.md) 15.1).
2. After every run, `migrate` writes `instance.min_server_version` from the constant
   `MinServerVersion` next to the embedded migrations ([18-release.md](../spec/18-release.md),
   section 2): the oldest server version that may run with the new schema. According to rule M1,
   that is the previous version; a release that breaks deliberately (major version) sets its own;
   before the first release it is the current one. If its own version is smaller, the server keeps
   running but never becomes ready: `/readyz` reports `schema_requires_newer_server` (03,
   `instance.min_server_version`). Whoever changes the schema checks at release time whether the
   constant must rise.
3. A newer schema without this lock is allowed; this is how the rollback from section 3 works.
4. `instance.schema_version` shows the state in the admin console.

## 6. PostgreSQL major version

A change of the major version (for example 18 to 19) is a separate step with a maintenance window:

1. Only when a release of `bothy-postgres` supports the new major version and the
   portability test is green with it.
2. Default path: logical dump and restore into a new database ([cloud-migration.md](cloud-migration.md)
   with the same host), with the old database frozen by maintenance mode
   (`server.maintenance_mode = read_only`, restart of `api` and `worker`; cloud-migration.md section 3
   steps 1 to 3, including the repetition of emergency revocations made after the final dump). The new
   database starts with the mode `off`. `pg_upgrade` is faster but needs both major versions in the same
   container; from version 18 on, the official image stores data under `/var/lib/postgresql/<major>/docker`
   to make `pg_upgrade --link` easier (https://hub.docker.com/_/postgres, retrieved
   2026-10-10) [verified]. An own upgrade image is a later ticket.
3. Immediately afterwards a full backup; the old pgBackRest stanza no longer applies to the new version.
