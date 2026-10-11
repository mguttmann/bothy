---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team F (operations and observability)
informed: Team A (architecture), Team C (privacy)
---

# Backup with pgBackRest, object backup and automatic restore test

## Context and Problem Statement

An instance keeps its data in PostgreSQL and in the object store (ADR 0006, ADR 0010). Data loss
should stay below `dr.rpo_target`, the restore below `dr.rto_target`
([16-operations.md](../spec/16-operations.md), section 1). The first target is a single host; later
targets are clouds with managed databases. A backup only counts as proven once it has been
restored. With which tools are backups made, where do the copies lie, and how is their
restorability proven regularly?

## Decision Drivers

- Restore to a point in time (PITR) with limited data loss.
- A path that is independent of major version and provider (relocation, last fallback).
- Off-site copies that a compromised server cannot delete (ransomware).
- Regular, automatic proof of the restore with a measured duration.
- Proven tools with checksums, encryption and retention instead of a custom build.
- The database container gets no egress and no credentials for the off-site target.

## Considered Options

- pgBackRest for physical backup and PITR (repository local and off-site, TLS repository host),
  `pg_dump -Fc` as logical backup, restic for objects, controlled by `bothy-backupd` with
  manifest and automatic restore test
- Logical backup only (`pg_dump`) plus restic
- WAL-G instead of pgBackRest
- Barman on a separate backup host
- Snapshots of volume or VM
- Only the backup of a managed database service

## Decision Outcome

Chosen option: "pgBackRest, `pg_dump`, restic and `bothy-backupd` with restore test", because it
combines PITR, a provider-independent dump and immutable off-site copies and runs the proof of
restore automatically. Normative in [16-operations.md](../spec/16-operations.md),
section 8 (R-OP-06 to R-OP-10); procedure in [backup-restore.md](../operations/backup-restore.md).

1. **PostgreSQL physical:** pgBackRest from the version with the fix for weak keys (2.59.3) onward;
   WAL continuously, at the latest after `backup.pg.archive_timeout`; full and differential backup
   per `backup.pg.full_schedule` and `backup.pg.diff_schedule`; repo1 local, repo2 off-site. The
   container `postgres` pushes WAL via pgBackRest TLS to the repository host `backup` (own
   image `bothy-postgres`, R-OP-07, R-OP-08).
2. **PostgreSQL logical:** `pg_dump -Fc` per `backup.logical.schedule` into restic off-site.
3. **Objects:** restic with the file system driver; with the S3 driver, versioning and replication
   of the provider, otherwise a mirror without deletion.
4. **Off-site append-only:** Object Lock or an access without delete permission; deletion after the
   end of the retention period only with a separate account (R-OP-09).
5. **Manifest per run:** row counts, `head_seq`, `sync_epoch`, head of the audit chain, schema and
   server version, number and sum of the files, completed deletion requests; no content. Every run
   writes a row in `backup_runs` and metrics.
6. **Automatic restore test** per `backup.restore_test.schedule` into a separate area:
   `pgbackrest verify`, manifest comparison, gapless sequence per partition, audit chain,
   samples of `thread_seq` and file checksums, duration against `dr.rto_target`.
7. `migrate` requires a successful backup younger than `migrate.recent_backup_max_age`
   (R-OP-13). With a managed service, the provider takes over WAL and PITR; `bothy-backupd`
   then only does logical backup, objects, manifest and test (16, section 3).

### Consequences

- Good, because PITR limits the loss to the archive interval and repo1 allows a fast local
  restore.
- Good, because the logical dump allows relocation and version changes without pgBackRest.
- Good, because the restore test measures the real duration and reports a tampered or incomplete
  backup before it is needed.
- Bad, because an own image `bothy-postgres` must be built, pinned and updated.
- Bad, because TLS between `postgres` and `backup` is laborious to set up; the fallback is repo1
  on a shared volume with a mirror to the off-site target.
- Bad, because a restore brings back deleted data. Mitigation: a deletion journal
  outside the database; `bothy restore finalize` runs completed deletion requests again
  ([20-privacy.md](../spec/20-privacy.md), R-DP-04).
- Bad, because a restore resets the sequence; the sync epoch catches that
  (ADR 0059).
- Neutral, because three copies (local, off-site physical, off-site logical) cost storage; the
  formula is in 16, section 13.

### Confirmation

- Build ticket M7-01 in [PLAN.md](../PLAN.md): integration test with two containers for
  `archive-push`, full backup, PITR restore and `pgbackrest verify` on amd64 and arm64; the
  database container has no egress.
- Build ticket M7-02: all backup kinds run, manifest complete, metrics in the Collector,
  off-site append-only.
- Build ticket M7-03: a tampered copy (gap in the event log, wrong audit hash, changed
  file) fails, each with the right check; a clean copy passes; duration against
  `dr.rto_target`.
- Alerts `BothyBackupStale`, `BothyWalArchiveStale`, `BothyRestoreTestStale`
  ([alerting.md](../observability/alerting.md)) with `promtool check rules`.
- Portability test (16, section 11.1, step 4): backup and restore test green on
  a fresh runner.

## Pros and Cons of the Options

### pgBackRest, `pg_dump`, restic, restore test

- Good, because pgBackRest offers full, differential and incremental backup, encryption,
  checksums with `verify`, multiple repositories and retention [Q1, verified] and is under
  MIT [Q2, verified]; restic checks samples with `check --read-data-subset` [Q3, verified].
- Bad, because three tools and a control service must work together.

### Logical backup only

- Good, because it is simple and independent of version and provider.
- Bad, because the loss reaches back to the last backup (hours instead of minutes) and large
  databases take long to restore.

### WAL-G

- Good, because Apache-2.0 [Q4, verified] and usable with many storage targets.
- Bad, because the mode with a repository host, through which the database container backs up
  without its own egress, is verified for pgBackRest but not checked here for WAL-G [assumed]; a
  switch changes the image and runbooks.

### Barman

- Good, because it is specialized in PostgreSQL.
- Bad, because a separate backup host is usual and the tool is under GPL-3.0
  [Q5, verified]; in the shipped image this would bring distribution obligations into the release
  chain [derived].

### Snapshots

- Good, because fast and without a tool in the database.
- Bad, because only crash-consistent, without PITR, and dependent on the platform; for
  container volumes on a single host there is no uniform snapshot path
  [derived].

### Managed service only

- Good, because the provider runs WAL and PITR.
- Bad, because on the first target (own host) there is no provider and objects, manifest and
  restore test are still missing; the path remains as a variant for clouds (item 7).

## More Information

- Sources (retrieved 2026-10-10): Q1 https://pgbackrest.org/ and
  https://pgbackrest.org/release.html [verified, as cited in 16]; Q2
  https://github.com/pgbackrest/pgbackrest/blob/main/LICENSE [verified]; Q3
  https://restic.readthedocs.io/en/stable/045_working_with_repos.html [verified]; Q4
  https://github.com/wal-g/wal-g/blob/master/LICENSE.md [verified]; Q5 https://github.com/EnterpriseDB/barman
  [verified].
- Open (16, section 8.1): whether repositories that were encrypted before pgBackRest 2.59.3 must be
  re-encrypted; for new instances the question does not arise.
- Related: ADR 0006 (PostgreSQL), ADR 0010 (object store), ADR 0011 (secret provider),
  ADR 0012 (Compose), ADR 0059 (sync epoch).
