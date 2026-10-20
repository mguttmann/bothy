# Sicherung und Wiederherstellung

Status: Entwurf M0. Anleitung zu [16-betrieb.md](../spec/16-betrieb.md) 8
([ADR 0058](../adr/0058-sicherung-mit-pgbackrest-objektsicherung-und-restore-test.md)). Gilt für
den Compose-Stack; mit einem verwalteten PostgreSQL-Dienst entfallen die Schritte zu pgBackRest
(Abschnitt 6).

![Sicherung](diagrams/backup-flow.svg)

## 1. Überblick

| Was | Wie | Wohin | Prüfung |
|---|---|---|---|
| WAL fortlaufend | pgBackRest `archive-push` aus `postgres` an den Repository-Host `backup` | repo1 lokal, repo2 off-site | `pgbackrest check`, Alarm `BothyWalArchiveStale` |
| Voll- und Differenzsicherung | `bothy-backupd` startet `pgbackrest backup` nach Plan | repo1, repo2 | Prüfsummen je Datei, `pgbackrest verify` |
| Logischer Abzug | `pg_dump -Fc` | restic off-site | Wiederherstellungstest |
| Objekte | restic (Treiber `fs`) oder Spiegel (Treiber `s3`) | off-site | `restic check --read-data-subset`, Stichprobe im Test |
| Manifest | je Lauf, JSON | neben der Sicherung | Vergleich im Test |
| Geheimnisse | `bothy secrets export` mit age | offline beim Owner | Probe-Entschlüsselung beim Einrichten |

Alle Pläne und Fristen sind Konfigurationswerte `backup.*` ([configuration.md](configuration.md)).

As built in M7-01 ([image README](../../services/server/images/bothy-postgres/README.md)): the
stanza is `bothy`; `postgres` and `backup` talk over pgBackRest TLS on port 8432 with the
certificates of `pgbackrest_tls/`; repo1 is the volume `pgbackrest-repo` of `backup`, encrypted
with `aes-256-cbc` and `pgbackrest_cipher_pass`. A full backup by hand:
`docker compose exec backup pgbackrest --stanza=bothy backup --type=full`; the check of the
repository: `docker compose exec backup pgbackrest --stanza=bothy verify`.

As built in M7-02 ([bothy-backupd README](../../services/server/cmd/bothy-backupd/README.md),
[16-betrieb.md](../spec/16-betrieb.md) 8.2 "As built"): `bothy-backupd` runs every kind on its
`backup.*` schedule, writes a row of `backup_runs` per run and a manifest per successful run, and
reports the metrics `bothy.backup.*` and `bothy.storage.volume.*`. One kind by hand:
`docker compose exec backup bothy-backupd backup pg_full` (also `pg_diff`, `logical`,
`objects`); exit code `0` means the run succeeded.

## 2. Einrichten

1. Off-site-Ziel anlegen. S3: Bucket mit Versionierung und Object Lock (Modus Governance oder
   Compliance, Frist mindestens `backup.logical.retention`), ein Schlüssel nur mit Schreiben und
   Lesen für `backup`, ein zweiter mit Löschen nur beim Owner. SFTP: Zugang ohne Löschen, Aufräumen
   durch einen zweiten Zugang.
2. Zugang in `secrets/offsite/` ablegen, `BOTHY_BACKUP_OFFSITE_TARGET` setzen.

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
3. `docker compose up -d backup`; `bothy-backupd` legt beim ersten Start die pgBackRest-Stanza und
   das restic-Repository an. It retries `stanza-create` until the database and its pgBackRest
   server answer; its log line `stanza bothy is ready` and the health check of `backup` (a TLS
   handshake with the repository host) show that the repository host is up.
4. Sofort eine Vollsicherung und einen Wiederherstellungstest auslösen
   ([install-compose.md](install-compose.md) 4).
5. Secrets-Bündel exportieren und die Entschlüsselung einmal auf einem anderen Gerät probieren.

## 3. Manifest

Jeder Lauf schreibt ein Manifest (JSON, verschlüsselt neben der Sicherung). Felder:

| Feld | Inhalt |
|---|---|
| `backup_id`, `kind`, `started_at`, `finished_at` | Lauf |
| `server_version`, `schema_version` | Stand des Codes und der Migrationen |
| `instance_id`, `sync_epoch` | Identität und Zeitlinie (03, `instance`) |
| `event_log.head_seq`, `event_log.min_retained_seq` | Grenzen des Ereignis-Logs |
| `audit.last_seq`, `audit.last_hash` | Kopf der Audit-Kette |
| `row_counts` | Zeilen je Tabelle |
| `files.count`, `files.total_bytes` | Objekte laut Tabelle `files` |
| `pgbackrest.label`, `restic.snapshot_id` | Bezug auf die Sicherungen |

Das Manifest enthält keine Inhalte und keine Geheimnisse.

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

## 4. Automatischer Wiederherstellungstest

Ablauf in `bothy-backupd` nach `backup.restore_test.schedule`:

1. Scratch-Verzeichnis leeren.
2. `pgbackrest restore` der letzten Sicherung nach `backup-scratch`, mit
   `recovery_target = immediate`.
3. PostgreSQL im Scratch-Verzeichnis starten, nur Unix-Socket, kein TCP.
4. Prüfungen aus [16-betrieb.md](../spec/16-betrieb.md) 8.3, jede mit eigenem Ergebnis.
5. Stichprobe der Dateien: Objekte aus der Objektsicherung in das Scratch-Verzeichnis holen und
   SHA-256 vergleichen.
6. PostgreSQL stoppen, Scratch-Verzeichnis löschen.
7. Ergebnis in `backup_runs` (Art `restore_test`, Dauer, Ergebnis je Prüfung) und als Kennzahlen.

Ein Fehlschlag ist ein Alarm `ticket`; zwei Fehlschläge in Folge oder ein Fehler der
Audit-Kette sind `page`. Der Test schreibt nie in die produktive Datenbank.

As built (M7-03, [16-betrieb.md](../spec/16-betrieb.md) 8.3, docs/DECISIONS.md D-M7-03-1 to
D-M7-03-9):

1. The scratch directory is `/var/lib/bothy-backup/restore` in the volume `backup-scratch`; every
   test empties it first and last, also after a failed or panicking check.
2. `pgbackrest --repo=1 --reset-pg1-host --pg1-path=<scratch>/pgdata --set=<label> --type=immediate
   --target-action=promote --archive-mode=off restore`, the label from the manifest of the newest
   succeeded `pg_full` or `pg_diff`; `restore_command` carries the same options, so the copy reads
   the WAL of the backup from the local repository. Then `pgbackrest --repo=1 verify`.
3. `postgres -D <scratch>/pgdata` with `listen_addresses` empty, `port=5432` (the name of the
   socket file only), `unix_socket_directories=<scratch>/socket` with mode `0700`, its own
   `pg_hba.conf` (`local all all trust`), `archive_mode=off`, `ssl=off`, no parallel query.
   `bothy-backupd` connects over the socket as `migrate.bootstrap_user`.
4. The checks run in the order of 16 8.3; each has its own result and duration in
   `backup_runs.checks` (`restore_test_checks.v1`).
5. The sampled objects come from `restic restore latest --tag objects --path <file.fs.root>` with
   one `--include` per object into `<scratch>/objects`; with the driver `s3` the file sample is
   skipped (`objects_driver_s3`).
6. The server is stopped with `SIGINT` (fast shutdown), after a minute with `SIGKILL`, and the
   directory is emptied.
7. `backup_runs` gets the row `restore_test` (`succeeded`, or `failed` with `checks_failed` or
   `scratch_cleanup_failed`) and the metrics of 16 8.3 "As built" item 5.

By hand: `docker compose exec backup bothy-backupd restore-test` (exit `0` when no check failed).
The result is in the newest row `SELECT state, error, checks FROM backup_runs WHERE kind =
'restore_test' ORDER BY started_at DESC LIMIT 1`. Not proven by this build: the duration of a
restore of a production-sized database against `dr.rto_target`.

## 5. Wiederherstellen

### 5.1 Zeitpunkt wählen

| Lage | Ziel |
|---|---|
| Fehlerhafte Änderung zu bekannter Zeit | `--type=time --target="<Zeit kurz davor>"` |
| Host verloren | letzter Stand im Off-site-Repository (`--type=default`) |
| Logischer Fehler, Zeit unklar | zuerst in Scratch probieren (Abschnitt 4 von Hand) und Zeitpunkt suchen |

### 5.2 Ablauf PITR im Compose-Stack

1. Statusseite: Störung anlegen; Wartungsmodus `server.maintenance_mode = read_only` hilft nicht,
   weil die Datenbank ersetzt wird. Darum: `docker compose stop api worker`.
2. `docker compose stop postgres`.
3. Altes Datenverzeichnis nicht löschen, sondern umbenennen (Volume kopieren), bis die
   Wiederherstellung geprüft ist.
4. `docker compose run --rm backup bothy-backupd restore --target-time "<Zeit>"` (ruft
   `pgbackrest restore` mit `--delta` auf das Volume `pgdata` auf).
5. `docker compose start postgres`; PostgreSQL spielt WAL bis zum Ziel und beginnt eine neue
   Zeitlinie.
6. `docker compose run --rm migrate restore finalize` (the entrypoint of the image is `bothy`): neue `sync_epoch`, Cursor der
   Agenten, Webhooks und Worker-Verbraucher auf `head_seq`, Audit-Eintrag (03 Abschnitt 6.7). It
   also stores the identity of the restored database (`instance.pg_system_identifier`,
   `instance.pg_timeline_id`, R-DM-50), all in one transaction; on an error nothing changes and
   the command can run again. It refuses with exit `1` while `api` or `worker` is connected
   (step 1 stopped them). Skipping this step is caught: a server whose database identity differs
   from the stored one stays not ready with `restore_not_finalized`, serves and delivers nothing and logs this step
   ([16-betrieb.md](../spec/16-betrieb.md) 8.4); the switch `database.restore_guard = off` is only
   for a managed failover without loss of committed transactions (section 6).
7. Wiederherstellungstest gegen den neuen Stand von Hand auslösen.
8. `docker compose start api worker`. Clients bekommen `resync_required` mit Grund
   `epoch_changed` und laden neu; ihr Ausgangsfach bleibt erhalten.
9. Störung mit Zeitpunkt und verlorenem Zeitraum beschreiben.

### 5.3 Logischer Abzug (anderer Host, andere Hauptversion)

1. Leere Instanz mit gleicher `BOTHY_VERSION` und dem Secrets-Bündel starten, `migrate` nicht
   laufen lassen.
2. `pg_restore --clean --if-exists --no-owner --role=bothy_owner` aus dem restic-Snapshot.
3. Objekte aus restic in das Volume `files` (bzw. in den Bucket) zurückspielen.
4. `bothy restore finalize`, dann Start und Wiederherstellungstest. The logical restore created a
   new cluster, so without this step the server stays not ready (`restore_not_finalized`). For a
   relocation without data loss use `bothy restore finalize --relocation <manifest>` instead
   ([cloud-migration.md](cloud-migration.md) 3).

## 6. Verwalteter PostgreSQL-Dienst

1. WAL-Archivierung und PITR übernimmt der Anbieter; die Frist stellt das OpenTofu-Modul ein.
2. `bothy-backupd` läuft ohne pgBackRest: logischer Abzug, Objektsicherung, Manifest,
   Wiederherstellungstest (Restore des logischen Abzugs in eine Scratch-Datenbank im Container).
3. Eine PITR beim Anbieter erzeugt eine neue Datenbank-Instanz; danach gilt 5.2 ab Schritt 6.
4. Automatic failover of the provider promotes a standby and raises the timeline, which the start
   lock treats like a restore (`restore_not_finalized`). Only where the provider documents that a
   failover loses no committed transaction (synchronous replication) set
   `database.restore_guard = off`; otherwise run `bothy restore finalize` after a failover, which
   starts a new epoch, because a lost commit would let new writes reuse sequence numbers clients
   already hold ([16-betrieb.md](../spec/16-betrieb.md) 8.4). Whether a provider shows
   `pg_control_system()` and `pg_control_checkpoint()` to the application role is not checked;
   PostgreSQL 18 grants both to `PUBLIC`.

## 7. Grenzen

1. Datenverlust bis `dr.rpo_target` ist möglich: Was nach dem letzten archivierten WAL-Segment
   committet wurde, fehlt nach einem Host-Verlust. Clients, die diese Nachrichten schon bestätigt
   bekamen, haben sie nicht mehr im Ausgangsfach.
2. Eine Wiederherstellung einzelner Nachrichten oder Threads gibt es nicht. Gelöschte Inhalte sind
   gewollt gelöscht (Kapitel 20); eine Wiederherstellung darf Löschungen aus DSGVO-Aufträgen
   nicht zurückholen. Darum führt `bothy restore finalize` Löschaufträge erneut aus, die nach dem
   Zielzeitpunkt erledigt wurden. Die Liste dieser Aufträge liegt nicht in der wiederhergestellten
   Datenbank; `bothy-backupd` schreibt sie darum bei jedem Lauf zusätzlich ins Manifest
   ([20-datenschutz.md](../spec/20-datenschutz.md), R-DP-04: Lösch-Journal außerhalb der
   Datenbank).
3. Gewöhnliche Löschungen nach dem Zielzeitpunkt (eigene Nachricht gelöscht) sind mit der neuen
   Zeitlinie verloren; die Inhalte erscheinen wieder. Die Störungsmeldung nennt das.

## 8. Immutable storage of the audit checkpoints

The signed checkpoints of the audit log are the only evidence against a complete rewrite
of the hash chain by the database administrators ([19-sicherheit.md](../spec/19-sicherheit.md),
SEC-AUD-2; [ADR 0021](../adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md)). They
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
the server side with the keys of [19-sicherheit.md](../spec/19-sicherheit.md) section 16:

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
