# Backup and restore

Status: draft M0. Guide to [16-operations.md](../spec/16-operations.md) 8
([ADR 0058](../adr/0058-backup-with-pgbackrest-object-backup-and-restore-test.md)). Applies to
the Compose stack; with a managed PostgreSQL service the steps for pgBackRest are dropped
(section 6).

![Backup](diagrams/backup-flow.svg)

## 1. Overview

| What | How | Where to | Check |
|---|---|---|---|
| WAL continuously | pgBackRest `archive-push` from `postgres` to the repository host `backup` | repo1 local, repo2 off-site | `pgbackrest check`, alert `BothyWalArchiveStale` |
| Full and differential backup | `bothy-backupd` starts `pgbackrest backup` on schedule | repo1, repo2 | checksums per file, `pgbackrest verify` |
| Logical dump | `pg_dump -Fc` | restic off-site | restore test |
| Objects | restic (driver `fs`) or mirror (driver `s3`) | off-site | `restic check --read-data-subset`, sample in the test |
| Manifest | per run, JSON | next to the backup | comparison in the test |
| Secrets | `bothy secrets export` with age | offline with the owner | trial decryption during setup |

All schedules and retention periods are configuration values `backup.*` ([configuration.md](configuration.md)).

As built in M7-01 ([image README](../../services/server/images/bothy-postgres/README.md)): the
stanza is `bothy`; `postgres` and `backup` talk over pgBackRest TLS on port 8432 with the
certificates of `pgbackrest_tls/`; repo1 is the volume `pgbackrest-repo` of `backup`, encrypted
with `aes-256-cbc` and `pgbackrest_cipher_pass`. A full backup by hand:
`docker compose exec backup pgbackrest --stanza=bothy backup --type=full`; the check of the
repository: `docker compose exec backup pgbackrest --stanza=bothy verify`.

As built in M7-02 ([bothy-backupd README](../../services/server/cmd/bothy-backupd/README.md),
[16-operations.md](../spec/16-operations.md) 8.2 "As built"): `bothy-backupd` runs every kind on its
`backup.*` schedule, writes a row of `backup_runs` per run and a manifest per successful run, and
reports the metrics `bothy.backup.*` and `bothy.storage.volume.*`. One kind by hand:
`docker compose exec backup bothy-backupd backup pg_full` (also `pg_diff`, `logical`,
`objects`); exit code `0` means the run succeeded.

## 2. Set up

1. Create the off-site target. S3: bucket with versioning and Object Lock (mode Governance or
   Compliance, period at least `backup.logical.retention`), one key with only write and
   read for `backup`, a second one with delete only with the owner. SFTP: access without delete, cleanup
   through a second access.
2. Put the access into `secrets/offsite/`, set `BOTHY_BACKUP_OFFSITE_TARGET`.

   As built (M7-02): only S3-compatible targets are supported (docs/DECISIONS.md D-M7-02-4).
   `BOTHY_BACKUP_OFFSITE_TARGET` (the key `backup.offsite.target`) has the form
   `s3:https://<host>[:<port>]/<bucket>[/<prefix>]`, for example
   `s3:https://s3.example.com/bothy-backup/instance-1`; `backup.offsite.region` and
   `backup.offsite.use_path_style` describe the endpoint. The bucket needs versioning and Object
   Lock with a default retention in mode Compliance of at least `backup.objects.retention`, and
   a lifecycle rule on the whole prefix that expires noncurrent versions
   (`NoncurrentVersionExpiration`, `NoncurrentDays` at least the Object Lock period) and removes
   expired delete markers. Without that rule nothing ever leaves the bucket: on a versioned bucket
   every deletion by restic or pgBackRest, the owner's expiry in section 9 included, only adds a
   delete marker and keeps the old version (docs/DECISIONS.md D-M7-02-12).
   `secrets/offsite/` holds three files of the key of `backup`: `access_key_id`,
   `secret_access_key` and, for a private CA only, `ca.crt`. That key may list, read and write
   objects (for AWS: `s3:ListBucket`, `s3:ListBucketVersions`, `s3:GetObject`, `s3:PutObject`,
   `s3:DeleteObject` for restic's own lock files, which on a versioned bucket only adds a delete
   marker); it must not hold `s3:DeleteObjectVersion`, `s3:PutObjectRetention`,
   `s3:BypassGovernanceRetention` or bucket settings. The owner's second key, with the right to
   delete old versions, never goes into `secrets/` (section 9). The Compose file sets
   `BOTHY_PGBACKREST_REPO2=on` for `postgres` when the target is set, so `archive-push` writes
   every WAL segment into repo2 as well. While the off-site target cannot be reached,
   `archive-push` fails and PostgreSQL keeps the segments in `pg_wal` and retries
   (`BothyWalArchiveStale`); the volume `pgdata` grows until the target answers again.

3. `docker compose up -d backup`; on its first start `bothy-backupd` creates the pgBackRest stanza and
   the restic repository. It retries `stanza-create` until the database and its pgBackRest
   server answer; its log line `stanza bothy is ready` and the health check of `backup` (a TLS
   handshake with the repository host) show that the repository host is up.
4. Trigger a full backup and a restore test immediately
   ([install-compose.md](install-compose.md) 4).
5. Export the secrets bundle and try the decryption once on another device.

## 3. Manifest

Every run writes a manifest (JSON, encrypted next to the backup). Fields:

| Field | Content |
|---|---|
| `backup_id`, `kind`, `started_at`, `finished_at` | run |
| `server_version`, `schema_version` | state of the code and of the migrations |
| `instance_id`, `sync_epoch` | identity and timeline (03, `instance`) |
| `event_log.head_seq`, `event_log.min_retained_seq` | bounds of the event log |
| `audit.last_seq`, `audit.last_hash` | head of the audit chain |
| `row_counts` | rows per table |
| `files.count`, `files.total_bytes` | objects according to the table `files` |
| `pgbackrest.label`, `restic.snapshot_id` | reference to the backups |

The manifest contains no contents and no secrets.

As built (M7-02): the JSON has the schema name `schema: backup_manifest.v1` and the fields above,
nested as their names say (`event_log.head_seq`, `audit.last_hash` as lowercase hex,
`files.total_bytes`); `row_counts` holds an exact `count(*)` of every table of the schema
`public` (partitions through their parent), read as `migrate.bootstrap_user` in one `READ ONLY`
transaction with `REPEATABLE READ`. `pgbackrest.label` is the label in repo1 and is set for
`pg_full` and `pg_diff`; `restic.snapshot_id` is set for `logical` and for `objects` with the
driver `fs`. The manifest is stored in `backup_runs.manifest` and as a restic snapshot with the
tags `manifest`, `<kind>` and `<run id>` in the restic repository (off-site, or local without a
target), which restic encrypts with `restic_password`. Every string has a closed form, so
`bothy-backupd` refuses a manifest field that could carry text.

## 4. Automatic restore test

Procedure in `bothy-backupd` according to `backup.restore_test.schedule`:

1. Empty the scratch directory.
2. `pgbackrest restore` of the latest backup to `backup-scratch`, with
   `recovery_target = immediate`.
3. Start PostgreSQL in the scratch directory, Unix socket only, no TCP.
4. Checks from [16-operations.md](../spec/16-operations.md) 8.3, each with its own result.
5. Sample of the files: fetch objects from the object backup into the scratch directory and
   compare SHA-256.
6. Stop PostgreSQL, delete the scratch directory.
7. Result in `backup_runs` (kind `restore_test`, duration, result per check) and as metrics.

A failure is a `ticket` alert; two failures in a row or an error of the
audit chain are `page`. The test never writes to the production database.

## 5. Restore

### 5.1 Choose a point in time

| Situation | Target |
|---|---|
| Faulty change at a known time | `--type=time --target="<time shortly before>"` |
| Host lost | latest state in the off-site repository (`--type=default`) |
| Logical error, time unclear | try in scratch first (section 4 by hand) and search for the point in time |

### 5.2 PITR procedure in the Compose stack

1. Status page: create an incident; maintenance mode `server.maintenance_mode = read_only` does not help,
   because the database is replaced. Therefore: `docker compose stop api worker`.
2. `docker compose stop postgres`.
3. Do not delete the old data directory but rename it (copy the volume) until the
   restore is verified.
4. `docker compose run --rm backup bothy-backupd restore --target-time "<time>"` (calls
   `pgbackrest restore` with `--delta` on the volume `pgdata`).
5. `docker compose start postgres`; PostgreSQL replays WAL up to the target and begins a new
   timeline.
6. `docker compose run --rm migrate bothy restore finalize`: new `sync_epoch`, cursors of the
   agents, webhooks and worker consumers to `head_seq`, audit entry (03 section 6.7).
7. Trigger a restore test against the new state by hand.
8. `docker compose start api worker`. Clients get `resync_required` with reason
   `epoch_changed` and reload; their outbox is kept.
9. Describe the incident with the point in time and the lost period.

### 5.3 Logical dump (other host, other major version)

1. Start an empty instance with the same `BOTHY_VERSION` and the secrets bundle, do not let `migrate`
   run.
2. `pg_restore --clean --if-exists --no-owner --role=bothy_owner` from the restic snapshot.
3. Restore the objects from restic into the volume `files` (or into the bucket).
4. `bothy restore finalize`, then start and restore test.

## 6. Managed PostgreSQL service

1. The provider takes over WAL archiving and PITR; the OpenTofu module sets the retention period.
2. `bothy-backupd` runs without pgBackRest: logical dump, object backup, manifest,
   restore test (restore of the logical dump into a scratch database in the container).
3. A PITR at the provider creates a new database instance; after that, 5.2 applies from step 6.

## 7. Limits

1. Data loss up to `dr.rpo_target` is possible: what was committed after the last archived WAL
   segment is missing after a host loss. Clients that already got these messages confirmed
   no longer have them in their outbox.
2. A restore of individual messages or threads does not exist. Deleted contents are
   deleted on purpose (chapter 20); a restore must not bring back deletions from privacy requests.
   Therefore `bothy restore finalize` runs deletion requests again that were completed after the
   target point in time. The list of these requests is not in the restored
   database; `bothy-backupd` therefore also writes it into the manifest on every run
   ([20-privacy.md](../spec/20-privacy.md), R-DP-04: deletion journal outside the
   database).
3. Ordinary deletions after the target point in time (own message deleted) are lost with the new
   timeline; the contents appear again. The incident report states this.

## 8. Immutable storage of the audit checkpoints

The signed checkpoints of the audit log are the only evidence against a complete rewrite
of the hash chain by the database administrators ([19-security.md](../spec/19-security.md),
SEC-AUD-2; [ADR 0021](../adr/0021-hash-chained-audit-log-separate-from-telemetry.md)). They
only help if neither the database administrators nor the administrators of the server host can change or
delete them. The server stores them at `audit.checkpoint_sink`; the target makes them immutable,
not the server.

The recommended target is an S3 bucket with Object Lock in mode Compliance. AWS describes the mode like this:
"In compliance mode, a protected object version can't be overwritten or deleted by any user,
including the root user in your AWS account." Object Lock requires versioning
(https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html, retrieved 2026-10-10
[verified]). Mode Governance is not enough here: whoever has `s3:BypassGovernanceRetention` can
delete (same source [verified]).

Set up:

1. Create the bucket in a separate account, not in the account of the off-site backup (section 2)
   and not in that of the server. Turn on versioning and Object Lock, default retention
   in mode Compliance with a period of at least `retention.audit`.
2. The access of the server may only create and read objects; deletion, changes to the retention and
   bucket settings stay with the owner.
3. Other S3-compatible storage only if its documentation proves a lock of the same strength (no
   deletion before the end of the period, not even by the administrator of the storage). Without proof the target
   counts as not immutable.

Implementation status: built with M7-22. `audit.checkpoint_sink = s3://<bucket>[/<prefix>]`
stores every checkpoint as the object `[<prefix>/]<instance_id>/<seq with 20 digits>.jws`, written
once with `If-None-Match: *` and never overwritten; the server reads the oldest version of every
key, so an overwrite that a storage lets through only adds a version that is never read. Set up
the server side with the keys of [19-security.md](../spec/19-security.md) section 16:

1. `audit.checkpoint_sink = s3://<bucket>[/<prefix>]`, for example `s3://bothy-audit` or
   `s3://audit-checkpoints.example.com/prod`.
2. `audit.checkpoint_s3_endpoint` (the endpoint of the separate account, never that of
   `file.s3.endpoint` or of the backup), `audit.checkpoint_s3_region` and
   `audit.checkpoint_s3_use_path_style`. An `http` endpoint needs an `egress.allowed_hosts` entry
   with `allow_private`.
3. The access key of item 2 above as the supplied files `audit_checkpoint_s3_access_key` and
   `audit_checkpoint_s3_secret_key` ([configuration.md](configuration.md) section 5), read through
   `audit.checkpoint_s3_access_key_ref` and `audit.checkpoint_s3_secret_key_ref`. The access needs
   `s3:PutObject`, `s3:GetObject`, `s3:GetObjectVersion`, `s3:ListBucketVersions` and
   `s3:GetBucketObjectLockConfiguration`, nothing else.
4. `bothy config check` reads the Object Lock configuration of the bucket and fails, naming
   `audit.checkpoint_sink`, unless Object Lock is enabled with a default retention in mode
   Compliance of at least `retention.audit`; Governance fails. The server start runs the same
   check and refuses to start with a failing target.

What is proven: the conformance suite, the lock check and the detection of a rewritten chain run
against the SeaweedFS test container at the version the Compose stack pins (docs/DECISIONS.md
M7-22/2). Not proven: a real cloud bucket in mode Compliance; the owner checks the first
checkpoint object of a new bucket once (`HeadObject` shows mode `COMPLIANCE` and a retain-until
date). A `file://` target stays possible and is not immutable: then the owner copies the
checkpoints with their own access into the bucket above.

## 9. Off-site expiry by the owner

`bothy-backupd` never deletes anything in the off-site bucket: its key in `secrets/offsite/`
cannot remove a stored version (R-OP-09), and it runs `pgbackrest expire` on repo1 only. Old
backups in the bucket stay until the owner removes them with the separate key that may delete
versions after the Object Lock period:

1. Keep the owner's key outside the stack, for example on the owner's device; never place it
   under `secrets/`.
2. Copy it for the run into a temporary directory with the same three file names as
   `secrets/offsite/` (`access_key_id`, `secret_access_key`, optional `ca.crt`), readable by the
   user `999`, and mount it read-only:
   `docker compose run --rm -v /path/to/owner-key:/run/offsite-admin:ro backup bothy-backupd expire --offsite --credentials /run/offsite-admin`.
3. The command runs `pgbackrest expire` on repo2 keeping `backup.pg.retention_full` full
   backups, and `restic forget --prune` for the snapshots of `logical` older than
   `backup.logical.retention`, of `objects` older than `backup.objects.retention` and of the
   manifests older than the larger of both. On the versioned bucket these deletions only add
   delete markers: the forgotten files become noncurrent versions, and the lifecycle rule of
   section 2 removes them `NoncurrentDays` later, once the Object Lock no longer holds them. So
   off-site data lives up to its retention plus `NoncurrentDays`; the command does not delete
   versions itself. Exit code `0` means every step succeeded, not that space was freed; the
   bucket's version listing shows when the rule has run.
4. Remove the temporary copy of the key afterwards.

The object mirror of the driver `s3` (below `<prefix>/objects/`) keeps objects deleted from the
file bucket as current versions; the owner deletes them by hand with the same key, and the rule of
section 2 removes the old versions `NoncurrentDays` later.
