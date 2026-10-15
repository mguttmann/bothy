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
repository: `docker compose exec backup pgbackrest --stanza=bothy verify`. Schedules, repo2 and
`expire` come with M7-02.

## 2. Einrichten

1. Off-site-Ziel anlegen. S3: Bucket mit Versionierung und Object Lock (Modus Governance oder
   Compliance, Frist mindestens `backup.logical.retention`), ein Schlüssel nur mit Schreiben und
   Lesen für `backup`, ein zweiter mit Löschen nur beim Owner. SFTP: Zugang ohne Löschen, Aufräumen
   durch einen zweiten Zugang.
2. Zugang in `secrets/offsite/` ablegen, `BOTHY_BACKUP_OFFSITE_TARGET` setzen.
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
6. `docker compose run --rm migrate bothy restore finalize`: neue `sync_epoch`, Cursor der
   Agenten, Webhooks und Worker-Verbraucher auf `head_seq`, Audit-Eintrag (03 Abschnitt 6.7).
7. Wiederherstellungstest gegen den neuen Stand von Hand auslösen.
8. `docker compose start api worker`. Clients bekommen `resync_required` mit Grund
   `epoch_changed` und laden neu; ihr Ausgangsfach bleibt erhalten.
9. Störung mit Zeitpunkt und verlorenem Zeitraum beschreiben.

### 5.3 Logischer Abzug (anderer Host, andere Hauptversion)

1. Leere Instanz mit gleicher `BOTHY_VERSION` und dem Secrets-Bündel starten, `migrate` nicht
   laufen lassen.
2. `pg_restore --clean --if-exists --no-owner --role=bothy_owner` aus dem restic-Snapshot.
3. Objekte aus restic in das Volume `files` (bzw. in den Bucket) zurückspielen.
4. `bothy restore finalize`, dann Start und Wiederherstellungstest.

## 6. Verwalteter PostgreSQL-Dienst

1. WAL-Archivierung und PITR übernimmt der Anbieter; die Frist stellt das OpenTofu-Modul ein.
2. `bothy-backupd` läuft ohne pgBackRest: logischer Abzug, Objektsicherung, Manifest,
   Wiederherstellungstest (Restore des logischen Abzugs in eine Scratch-Datenbank im Container).
3. Eine PITR beim Anbieter erzeugt eine neue Datenbank-Instanz; danach gilt 5.2 ab Schritt 6.

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
