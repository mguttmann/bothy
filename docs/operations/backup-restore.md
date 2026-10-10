# Sicherung und Wiederherstellung

Status: Entwurf M0. Anleitung zu [16-betrieb.md](../spec/16-betrieb.md) 8. Gilt für den
Compose-Stack; mit einem verwalteten PostgreSQL-Dienst entfallen die Schritte zu pgBackRest
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

## 2. Einrichten

1. Off-site-Ziel anlegen. S3: Bucket mit Versionierung und Object Lock (Modus Governance oder
   Compliance, Frist mindestens `backup.logical.retention`), ein Schlüssel nur mit Schreiben und
   Lesen für `backup`, ein zweiter mit Löschen nur beim Owner. SFTP: Zugang ohne Löschen, Aufräumen
   durch einen zweiten Zugang.
2. Zugang in `secrets/offsite/` ablegen, `BOTHY_BACKUP_OFFSITE_TARGET` setzen.
3. `docker compose up -d backup`; `bothy-backupd` legt beim ersten Start die pgBackRest-Stanza und
   das restic-Repository an.
4. Sofort eine Vollsicherung und einen Wiederherstellungstest auslösen
   ([install-compose.md](install-compose.md) 4).
5. Secrets-Bündel exportieren und die Entschlüsselung einmal auf einem anderen Gerät probieren.

## 3. Manifest

Jeder Lauf schreibt ein Manifest (JSON, verschlüsselt neben der Sicherung). Felder:

| Feld | Inhalt |
|---|---|
| `backup_id`, `kind`, `started_at`, `finished_at` | Lauf |
| `server_version`, `schema_version` | Stand des Codes und der Migrationen |
| `instance_id`, `sync_epoch` | Identität und Zeitlinie (Ergänzung beantragt) |
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
   Agenten und Webhooks auf `head_seq`, Audit-Eintrag (Ergänzung beantragt).
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
   (Ergänzung beantragt bei Kapitel 20).
3. Gewöhnliche Löschungen nach dem Zielzeitpunkt (eigene Nachricht gelöscht) sind mit der neuen
   Zeitlinie verloren; die Inhalte erscheinen wieder. Die Störungsmeldung nennt das.
