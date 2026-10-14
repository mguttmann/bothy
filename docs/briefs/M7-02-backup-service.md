# M7-02: Backup service: schedules, manifest and metrics (ADR 0058)

Status: ready | Depends on: M3-02, M3-03, M7-01 | Milestone: M7

## Goal

After this ticket `bothy-backupd` in the service `backup` backs up an instance on its own: full and differential pgBackRest backups per `backup.pg.full_schedule` and `backup.pg.diff_schedule` into repo1 (local) and repo2 (off-site), a logical dump `pg_dump -Fc` per `backup.logical.schedule` into a restic repository off-site, the object backup per `backup.objects.schedule` (restic for the file system driver, a mirror without deletion into a second bucket for the S3 driver), and `expire` on repo1 (16 8.1, 8.2, ADR 0058 items 1 to 5). Every run writes a manifest (`backup_manifest.v1`: row counts per table, `head_seq`, last audit `seq` and `hash`, schema and server version, `sync_epoch`, count and sum of the file sizes; never content) next to the backup and a row into `backup_runs` through the port `BackupRuns` of the module `status`, with a separate database role that may write only this table (03 4.12). It reports the metrics `bothy.backup.*` and `bothy.storage.volume.*` of `signals.md`. Off-site targets are append-only: the credentials of `backup` can write and read, never delete (R-OP-09). From this ticket on, `bothy migrate up` aborts on pending migrations of an existing schema when no successful backup is younger than `migrate.recent_backup_max_age`, as long as `migrate.require_recent_backup` holds (16 10 item 1, R-OP-13). Why: a backup that is not scheduled, not described and not reported is not a backup; the manifest is what the restore test (M7-03) and the epoch (M7-04) compare against.

## Read first

- `docs/spec/16-operations.md`: 2, 7.1 (roles; `bothy-backupd` uses the superuser for pgBackRest and `pg_dump`), 8.1, 8.2, 10 item 1, 13 (formula, `bothy.storage.volume.free`), 14 (all `backup.*`, `migrate.require_recent_backup`, `migrate.recent_backup_max_age`, `dr.rpo_target`), 15 (R-OP-06 to R-OP-10, R-OP-13).
- `docs/spec/03-data-model.md` (core contract, read only): `backup_runs` (4.12: columns, check, index, retention `ops`, the separate role), `instance` (`sync_epoch`, `schema_version`), `audit_events` (`seq`, `hash`), `files` (`size_bytes`, `deleted_at`).
- `docs/adr/0058-backup-with-pgbackrest-object-backup-and-restore-test.md`; `docs/operations/backup-restore.md` 1 to 3 and 6; `docs/operations/configuration.md` (rows `backup.*`, the secret rows `pgbackrest_cipher_pass`, `restic_password`, `offsite/`, `db_superuser_password`, the variable `BOTHY_BACKUP_OFFSITE_TARGET`).
- `docs/observability/signals.md` (rows `bothy.backup.*`, `bothy.storage.volume.*` and the paragraph on read-only volumes), `docs/observability/alerting.md` (`BothyBackupStale`, `BothyWalArchiveStale`, `BothyVolumeFull`), `docs/spec/17-observability.md` 2 and 6.
- `docs/spec/02-architecture.md`: 4.5 (module `status`, port `BackupRuns`), 12.3 (jobs), 13 (configuration schema).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-02-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-01-postgres-image.md` (image, `cmd/bothy-backupd`, `postgres-image-test`), `M1-12-postgres-migrations.md` (`migrate up --bootstrap`, roles, grants per table), `M1-68-core-metrics.md` (signal registry, `make signal-check`), `M3-01-object-storage-fs.md` and `M3-02-object-storage-s3.md` (drivers, SeaweedFS test container), `M3-03-upload-sessions.md` (`files`).
- Code on the base branch: `services/server/cmd/bothy-backupd`, `services/server/images/bothy-postgres`, `services/server/internal/status`, `services/server/cmd/bothy` (`migrate`), `services/server/migrations`, `infra/compose/compose.yaml`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Schedules, limits and periods only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour, configuration, a role or a secret entry updates `docs/spec/16-operations.md` and `docs/operations/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket; `backup_runs` is built exactly as 03 has it. Everything this ticket adds is English.
11. No cost (D-W-125): tests use local targets and throwaway containers only; no cloud account, no paid service.

Product requirements implemented here:

12. Every kind of 16 8.1 runs on its schedule: `pg_full`, `pg_diff`, `logical`, `objects`; each run is one `backup_runs` row from `running` to `succeeded` or `failed`, with `error` as a code without content or secrets.
13. The manifest contains exactly the fields of 03 (`backup_manifest.v1`) and the references of `backup-restore.md` 3 (`pgbackrest.label`, `restic.snapshot_id`), never content; it is stored encrypted next to the backup and as `backup_runs.manifest`.
14. Off-site writes are append-only: the credentials in the secret directory of `backup` cannot delete or overwrite; `expire` runs only on repo1; off-site expiry needs the separate account of the owner (R-OP-09, Design note 4).
15. With an empty `backup.offsite.target`, backups stay local, `bothy config check` and the start log of `bothy-backupd` warn "local backup only".
16. The database role that writes `backup_runs` can write only this table; `pgBackRest` and `pg_dump` connect as `migrate.bootstrap_user` (16 7.1).
17. `migrate up` checks the most recent successful run of a kind `pg_full`, `pg_diff` or `logical` against `migrate.recent_backup_max_age` before it applies a pending migration to an existing schema; an empty database is exempt; `migrate.require_recent_backup = false` turns the check off.
18. Metrics as in `signals.md`, through the OpenTelemetry SDK of `bothy-backupd` (`service.name = bothy-backupd`); no person, path or bucket name in an attribute.

## Scope

Server module (paths under `services/server/` unless they start with `docs/` or `infra/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- Module `status`: migration `backup_runs` (03 4.12) with the grants of 16 7.1 rule 3; the port `BackupRuns` (start, finish, last successful run per kind); `domain` for the manifest schema and its validation; `app` for the backup use cases (schedule evaluation, one run per kind, manifest building through read-only queries, metrics); `adapters` for the executor of `pgbackrest`, `pg_dump`, `restic` and the S3 mirror (Design note 2). The table list and README of `status` follow.
- `cmd/bothy-backupd`: the scheduler loop (cron expressions of `backup.*`, at most one run per kind at a time), the subcommands `backup <kind>` (run one kind now, for the runbook and tests) and `expire --offsite` (owner only, Design note 4); configuration through the schema of `internal/platform/config` (the keys `backup.*` become active).
- Database role `bothy_backup` (Design note 3): `migrate up --bootstrap` creates it like the four roles of 16 7.1; secret `db_backup_password` in the catalogue (generated, to `migrate` and `backup`); key `migrate.backup_password_ref`.
- `cmd/bothy` `migrate`: the backup requirement of item 17; keys `migrate.require_recent_backup` and `migrate.recent_backup_max_age` become active.
- `images/bothy-postgres`: restic pinned with a SHA-256 per architecture; nothing else changes in the image.
- `infra/compose/compose.yaml` and `.env.example`: what `backup` needs (secret paths, the off-site target variable); the off-site test overlay goes under `infra/compose/test/` if one is needed.
- Docs: `docs/spec/16-operations.md` 7.1 (row `bothy_backup`), 8.2, 14 (new keys), `docs/operations/backup-restore.md` (2, 3, the owner's off-site expiry), `docs/operations/configuration.md` (keys active, secret row), `docs/observability/signals.md` only if a name differs, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-02-<n>`.

## Out of scope

- The restore test and `backup.restore_test.*` (M7-03); `bothy restore finalize` (M7-04); the deletion journal and the list of completed deletion requests in the manifest (M7-07, which builds on the privacy requests of M7-06).
- Retention of old `backup_runs` rows (job `retention.ops`, M7-09) and the alert rules (M7-20 routes alerts; the rules of `alerting.md` already exist).
- A console view of `backup_runs`: `docs/api/` has no operation for it; adding one would be a core change.
- The path for a managed PostgreSQL service (16 3 rule 3, `backup-restore.md` 6); this ticket builds the Compose path with pgBackRest.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-02-backup-service`; one writer per worktree.
- Conventional Commits with the module or component as scope (for example `feat(status): ...`, `feat(backupd): ...`, `feat(compose): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories or cloud resources; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL, SeaweedFS and every other service run only as throwaway containers on random ports (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; tests generate throwaway passwords and keys.
- Time only through `kernel/clock`; the scheduler test moves a fake clock, it never sleeps for an interval.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Scheduler** (16 8.1). Five-field cron expressions parsed by a small parser in `status/domain` or a pinned library under the license gate (record which); missed times while the process was down are not caught up, the next due time runs; at most one run per kind; a full and a differential due at the same minute run only the full. `backup <kind>` runs one kind now and exits with `0` or `1`.

2. **Executor port** (rule 2). The use cases call a port with one method per external step (`PgBackRestBackup`, `PgBackRestExpireLocal`, `PgDump`, `ResticBackup`, `S3Mirror`, `VolumeUsage`); the adapter runs the binaries with arguments as a list, never through a shell, and maps exit codes to error codes without output text. Unit tests use a fake executor; the real binaries run in the image test.

3. **Role `bothy_backup`** (03 4.12; Ruling for docs/DECISIONS.md). `LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`, `CONNECT`, `USAGE` on `public`, `SELECT, INSERT, UPDATE` on `backup_runs` only (a test against the expectation list of 16 7.1 rule 3). The manifest queries (row counts, `head_seq`, audit head, files) run as `migrate.bootstrap_user` in a `READ ONLY` transaction with `REPEATABLE READ`, so the counts belong to one snapshot. Exact row counts per table come from `count(*)` per table of the domain schema (not `pg_class.reltuples`), because the restore test compares them.

4. **Append-only off-site** (R-OP-09; Ruling for docs/DECISIONS.md). Supported targets: S3-compatible with versioning and Object Lock, and restic's REST server in append-only mode or SFTP without delete permission, as `backup-restore.md` 2 names. `backup` holds only the credentials in `offsite/`; deletion after the retention period needs `offsite-admin/`, which never enters the volume `secrets` and which the owner passes to `bothy-backupd expire --offsite` by hand (runbook). Test: with the credentials of `backup`, deleting or overwriting an existing object or snapshot fails against the local test target.

5. **Object backup** (16 8.1 rows "Objects"). Driver `fs`: restic over the read-only volume `files`. Driver `s3`: a mirror into the second bucket that copies new objects and never deletes (deleted objects stay until the off-site expiry); if the provider replicates with versioning, `backup.objects.*` only checks that the replica is present (record the decision).

6. **Metrics** (signals.md). Exactly the instruments and attributes of the table, declared as `Spec`s in the registry of M1-68 so `make signal-check` holds them; `bothy.backup.wal.last_archived.timestamp` from `pgbackrest info` (archive stop time).

7. **Rulings for `docs/DECISIONS.md`** (`D-M7-02-<n>`): cron parser, role and snapshot (note 3), off-site targets and the owner's expiry (note 4), object mirror (note 5), skipped catch-up of missed times (note 1).

## Tests (write them first)

Unit:

- `status/domain`: `TestManifestSchema` (exactly the fields of 03 and `backup-restore.md` 3; a field with message text fails validation), `TestCronDue` (due times, a full and a differential at the same minute).
- `status/app`: `TestRunWritesRowAndManifest`, `TestRunFailureRecordsCode` (an executor error gives `failed` with a code and no output text), `TestOneRunPerKind`, `TestOffsiteEmptyWarns`.
- `cmd/bothy`: `TestMigrateRequiresRecentBackup` (pending migrations and no run younger than the maximum age: abort with a hint; a fresh run: proceed; empty database: proceed; switch off: proceed).

Integration (`pgtest` and throwaway containers):

- `TestBackupRunsRole`: `bothy_backup` can insert and update `backup_runs` and nothing else (insert into another table fails).
- `TestManifestCounts`: after seeding messages, files and audit rows the manifest counts and heads equal direct queries.
- Image test in `postgres-image-test` (extend the suite of M7-01): every kind runs against the database container and local targets (a local directory for repo1, SeaweedFS or a restic REST server in append-only mode for off-site), `backup_runs` has one `succeeded` row per kind, the manifest is present next to the backup, a delete with the credentials of `backup` fails, metrics reach an OTLP test receiver with the names of `signals.md`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/status/... ./cmd/... ./internal/platform/... ./internal/wire/...`
- `make postgres-image postgres-image-test` on the build host
- `make migrations-lint`, `make migrations-lock`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`, `make signal-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] All kinds run on their schedule and by hand; each writes a `backup_runs` row and a complete manifest without content.
- [ ] Off-site is append-only for the credentials of `backup`; `expire` runs only on repo1; the owner's off-site expiry is documented.
- [ ] Metrics of `signals.md` reach the collector path; `make signal-check` is green.
- [ ] `bothy_backup` writes only `backup_runs`; `migrate up` enforces a recent backup as in 16 10 item 1.
- [ ] 16, the runbook, the configuration reference, READMEs, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `status` and `cmd/bothy-backupd` updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make postgres-image-test`.
3. Review and red team verdicts and the findings fixed.
4. The off-site target kinds tested locally and how append-only was proven.
5. What is not proven (a real cloud bucket with Object Lock).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
