# Upgrade

Status: built (M1 to M7). Guide to [16-operations.md](../spec/16-operations.md) 10 and
[18-release.md](../spec/18-release.md). Applies to instance operators (procedure) and to developers
of migrations (rules in section 4).

The procedure covers the Compose stack of the server. The host service installs, updates and
restarts as a system service by itself ([services/host/README.md](../../services/host/README.md)).
The Claude Code runner and the adapters are processes the operator runs: the repository has no
service template (launchd, systemd) for them, no rule for their logs and rotation, and no order
of their update relative to the server (plan row V3-47). On `SIGTERM` the runner hands its running
turns over and exits with `75`, and only its next start adopts them, so something must start it
again ([claude-code-runner](../../plugins/claude-code-runner/README.md), "Handover"); its check
before a start is `check-config`.

## 1. Before the upgrade

1. Read the release notes: the section "Upgrade notes" says whether manual work is needed; the
   section "Migrations" names expand or contract steps and their expected duration on large tables.
2. Only one major version at a time. Within a major version, an instance may skip versions; the
   migrations run in order.
3. Check the signature ([release.md](release.md) 5).
4. Backup: `migrate` requires a successful backup younger than
   `migrate.recent_backup_max_age` anyway. Before large releases, also trigger a full backup
   by hand. Without an off-site target and with the default `backup.pg.local_retention_full`,
   that full backup removes the one before it: from then on a point-in-time restore reaches back
   only to this backup ([backup-restore.md](backup-restore.md) 7 item 4).
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

Check: `docker compose ps` (the image tag of `api` and `worker` is the new version; the label
`org.opencontainers.image.version` of `docker image inspect` names it too), `/readyz`, status
page, SLO dashboard, and `bothy migrate status` for the applied migrations. The admin console
shows only the version of the specification, neither the server nor the schema version.

### 2.1 Installations from a bundle

The procedure above replaces the images only. An installation made by `install.sh` also holds the
files of its bundle, among them `compose.yaml`, `.env.example`, `infra/otel` and
`infra/observability` with the Grafana panel plugin, and the marker `.bothy-install`, which names
the version of that bundle. `install.sh` of another version refuses the folder instead of
upgrading it, because a newer bundle over a `.env` that keeps the old `BOTHY_VERSION` would mix two
releases, and there is no upgrade mode of `install.sh` before version 1.0
(docs/decisions.d/AUD-PKG-R2-owner-ops.md). The OpenTofu modules do not upgrade either: the cloud
modules ignore a changed `bothy_version` on an existing server, and over SSH the setup ends in that
refusal; run the steps below on the server. `<dir>` is the folder of the installation, `<old>` and
`<new>` are the two versions:

1. Keep a copy of the installation, with modes, in a folder only root reads:
   `cp -Rp <dir> <dir>.before-<new>` and then `chmod 700 <dir>.before-<new>`. It holds `.env` and
   `infra/compose/secrets/`. The folder must not exist yet: a copy left from an earlier attempt
   would receive `<dir>` inside it, and the later steps would read its older files. Remove or
   rename an old copy first.
2. Download the bundle `bothy-compose-<new>.tar.gz` of the new product release and its
   signature `bothy-compose-<new>.tar.gz.sigstore.json`, and check both as in
   [release.md](release.md) 5.
3. Unpack it into a new, empty folder and check that every member lies under `bothy-<new>/` and
   that it holds neither `infra/compose/.env` nor `infra/compose/secrets/`.
4. Copy the bundle over the installation: `cp -R <unpacked>/bothy-<new>/. <dir>/`. This replaces
   every file of the bundle and leaves `.env` and `secrets/` as they are. A file that the new
   bundle no longer has stays behind; the release notes name such files under "Upgrade notes".
5. Compare the two templates,
   `diff <dir>.before-<new>/infra/compose/.env.example <dir>/infra/compose/.env.example`, add
   every new variable to `infra/compose/.env` with its value, and copy the value of
   `BOTHY_VERSION` from the new `.env.example` into `.env`: it is the server version the product
   release pins, which can differ from `<new>`, the version of the product release.
6. Advance the marker: `printf 'version=%s\n' <new> > <dir>/.bothy-install`.
7. In `<dir>/infra/compose` run the procedure of section 2 and its checks. Then recreate the
   services that read files of the bundle only at their start, because Compose keeps a container
   whose image did not change:
   `docker compose up -d --force-recreate --no-deps otel-collector clamav`, and with the profile
   `observability` also `prometheus alertmanager loki tempo grafana`.

The way back to `<old>` (section 3 item 1) puts the files of the copy back before
`docker compose up -d`: every file of `<dir>.before-<new>` except `infra/compose/secrets/`, which
keeps its newer state; then recreate the same services as in step 7. Remove the copy once the
new version runs and the way back is no longer needed; it holds secrets.

## 3. Rollback

1. One release back: set `BOTHY_VERSION` to the previous version, `docker compose up -d`. The schema
   stays; the expand/contract rule guarantees that the code of N-1 runs with it. `migrate`
   of version N-1 recognizes the newer but compatible schema and exits without a change.
2. More than one release back: only by a restore to a point in time before the upgrade
   ([backup-restore.md](backup-restore.md) 5.2). Data after this point in time is lost. The
   point must lie after the oldest full backup the repository still keeps: without an off-site
   target and with the default `backup.pg.local_retention_full`, the first full backup after the
   upgrade, scheduled or by hand, ends this way back
   ([backup-restore.md](backup-restore.md) 7 item 4).

## 4. Rules for migrations (Expand/Contract)

| No. | Rule | Reason |
|---|---|---|
| M1 | Version N may only remove or reinterpret what version N-1 no longer reads or writes. | Rollback by one version without reverting the database. |
| M2 | Never rename directly: new column (expand), write both (N), read only the new one (N+1), remove the old one (N+2). | M1. |
| M3 | New columns allow `NULL` or have a default without rewriting the table. | No long lock on large tables. |
| M4 | Indexes with `CREATE INDEX CONCURRENTLY` in a migration without a transaction (goose: `-- +goose NO TRANSACTION`), one index per migration file; the `DROP INDEX CONCURRENTLY IF EXISTS` before the build only removes an invalid rest of the same build. Replacing a unique index builds the new one in one migration and drops the old one in a later one. | Writes continue. goose records each finished file, so a repeated run after an abort never drops an index that was already built, and the uniqueness is never without an index. |
| M5 | New constraints first as `NOT VALID`, `VALIDATE CONSTRAINT` in a later migration. | Check without a write lock. |
| M6 | Every step sets `lock_timeout` to `migrate.lock_timeout` and would rather abort than wait. | A waiting lock blocks all following requests. |
| M7 | Never backfill data in the migration, but as a background job in batches. | Short transactions; progress visible. |
| M8 | Enumerations are `text` with `CHECK`; new values are additive (R-DM-02). Code of N-1 must tolerate unknown values. A stored JSON document with a closed schema (for example `agents.operator_config`) is checked strictly on write only; a read ignores keys it does not know. | Rollback. |
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

Not built: a gate that runs every migration against the schema of the previous version with test
data and then the test suite of the previous version against the new schema (N-1 compatibility;
plan row V3-37). Before V1 no such gate runs: it would take hours per change, and there is no
release to go back to (docs/decisions.d/AUD-PKG-R2-owner-ops.md). Until then rules M1 to M12 hold
by review, `make migrations-lint` checks numbers, checksums and form rules only, and an older
build is not tested against a newer schema (section 3 item 1).

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
4. `bothy migrate status` shows the applied migrations; `instance.schema_version` is stored in
   the database, and no view of the admin console shows it.

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
