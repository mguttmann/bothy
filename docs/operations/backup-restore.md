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

## 8. Unveränderliche Ablage der Audit-Prüfpunkte

Die signierten Prüfpunkte des Audit-Logs sind der einzige Beleg gegen ein vollständiges Neuschreiben
der Hash-Kette durch die Datenbank-Verwaltung ([19-sicherheit.md](../spec/19-sicherheit.md),
SEC-AUD-2; [ADR 0021](../adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md)). Sie
helfen nur, wenn weder die Datenbank-Verwaltung noch die Verwaltung des Server-Hosts sie ändern oder
löschen kann. Der Server legt sie an `audit.checkpoint_sink` ab; unveränderlich macht sie das Ziel,
nicht der Server.

Empfohlenes Ziel ist ein S3-Bucket mit Object Lock im Modus Compliance. AWS beschreibt den Modus so:
"In compliance mode, a protected object version can't be overwritten or deleted by any user,
including the root user in your AWS account." Object Lock setzt Versionierung voraus
(https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html, abgerufen 2026-10-10
[belegt]). Der Modus Governance genügt hier nicht: Wer `s3:BypassGovernanceRetention` hat, kann
löschen (dieselbe Quelle [belegt]).

Einrichten:

1. Bucket in einem eigenen Konto anlegen, nicht in dem Konto der Off-site-Sicherung (Abschnitt 2)
   und nicht in dem des Servers. Versionierung und Object Lock einschalten, Standard-Aufbewahrung
   im Modus Compliance mit einer Frist von mindestens `retention.audit`.
2. Der Zugang des Servers darf nur Objekte anlegen und lesen; Löschen, Ändern der Aufbewahrung und
   Bucket-Einstellungen bleiben beim Owner.
3. Andere S3-kompatible Speicher nur, wenn ihre Doku eine Sperre gleicher Stärke belegt (kein
   Löschen vor Fristende, auch nicht durch den Verwalter des Speichers). Ohne Beleg gilt das Ziel
   als nicht unveränderlich.

Stand der Umsetzung: Bis PLAN M7-22 kennt `audit.checkpoint_sink` nur `file://` (19,
"Konfigurationswerte"). Bis dahin zeigt das Ziel auf einen Pfad außerhalb des Datenbank-Volumes,
möglichst auf einem anderen Rechner eingebunden, den die Datenbank-Verwaltung nicht erreicht. Ein
Dateisystem-Ziel ist nicht unveränderlich: Der Owner kopiert die Prüfpunkte darum regelmäßig mit
eigenem Zugang in den Bucket oben, bis M7-22 das Objektspeicher-Ziel mitbringt. Ab M7-22 lehnt
`bothy config check` ein Objektspeicher-Ziel ohne wirksame Aufbewahrungssperre ab.
