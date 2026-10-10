---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team F (Betrieb und Observability)
informed: Team A (Architektur), Team C (Datenschutz)
---

# Sicherung mit pgBackRest, Objekt-Sicherung und automatischem Restore-Test

## Context and Problem Statement

Eine Instanz hält ihre Daten in PostgreSQL und im Objektspeicher (ADR 0006, ADR 0010). Der
Datenverlust soll unter `dr.rpo_target` bleiben, die Wiederherstellung unter `dr.rto_target`
([16-betrieb.md](../spec/16-betrieb.md), Abschnitt 1). Erstziel ist ein einzelner Host; spätere
Ziele sind Clouds mit verwalteten Datenbanken. Eine Sicherung gilt erst als belegt, wenn sie
zurückgespielt wurde. Mit welchen Werkzeugen wird gesichert, wo liegen die Kopien, und wie wird
ihre Wiederherstellbarkeit regelmäßig bewiesen?

## Decision Drivers

- Wiederherstellung auf einen Zeitpunkt (PITR) mit begrenztem Datenverlust.
- Ein Weg, der unabhängig von Hauptversion und Anbieter ist (Umzug, letzte Rückfallebene).
- Kopien außer Haus, die ein übernommener Server nicht löschen kann (Ransomware).
- Regelmäßiger, automatischer Beweis der Wiederherstellung mit gemessener Dauer.
- Bewährte Werkzeuge mit Prüfsummen, Verschlüsselung und Aufbewahrung statt Eigenbau.
- Der Datenbank-Container bekommt keinen Ausgang und keine Zugangsdaten des Off-site-Ziels.

## Considered Options

- pgBackRest für physische Sicherung und PITR (Repository lokal und off-site, TLS-Repository-Host),
  `pg_dump -Fc` als logische Sicherung, restic für Objekte, gesteuert von `bothy-backupd` mit
  Manifest und automatischem Wiederherstellungstest
- Nur logische Sicherung (`pg_dump`) plus restic
- WAL-G statt pgBackRest
- Barman auf einem eigenen Sicherungs-Host
- Snapshots von Volume oder VM
- Nur die Sicherung eines verwalteten Datenbankdienstes

## Decision Outcome

Gewählte Option: "pgBackRest, `pg_dump`, restic und `bothy-backupd` mit Restore-Test", weil sie
PITR, einen anbieterunabhängigen Abzug und unveränderliche Off-site-Kopien vereint und den Beweis
der Wiederherstellung automatisch führt. Normativ in [16-betrieb.md](../spec/16-betrieb.md),
Abschnitt 8 (R-OP-06 bis R-OP-10); Ablauf in [backup-restore.md](../operations/backup-restore.md).

1. **PostgreSQL physisch:** pgBackRest ab der Version mit dem Fix für schwache Schlüssel (2.59.3);
   WAL fortlaufend, spätestens nach `backup.pg.archive_timeout`; Voll- und Differenzsicherung nach
   `backup.pg.full_schedule` und `backup.pg.diff_schedule`; repo1 lokal, repo2 off-site. Der
   Container `postgres` schiebt WAL per pgBackRest-TLS an den Repository-Host `backup` (eigenes
   Image `bothy-postgres`, R-OP-07, R-OP-08).
2. **PostgreSQL logisch:** `pg_dump -Fc` nach `backup.logical.schedule` in restic off-site.
3. **Objekte:** restic beim Dateisystem-Treiber; beim S3-Treiber Versionierung und Replikation des
   Anbieters, sonst ein Spiegel ohne Löschen.
4. **Off-site nur anhängend:** Object Lock oder ein Zugang ohne Löschrecht; Löschen nach Fristende
   nur mit einem getrennten Konto (R-OP-09).
5. **Manifest je Lauf:** Zeilenzahlen, `head_seq`, `sync_epoch`, Kopf der Audit-Kette, Schema- und
   Server-Version, Zahl und Summe der Dateien, erledigte Löschaufträge; keine Inhalte. Jeder Lauf
   schreibt eine Zeile in `backup_runs` und Kennzahlen.
6. **Automatischer Wiederherstellungstest** nach `backup.restore_test.schedule` in einen eigenen
   Bereich: `pgbackrest verify`, Manifest-Vergleich, lückenlose Sequenz je Partition, Audit-Kette,
   Stichproben von `thread_seq` und Datei-Prüfsummen, Dauer gegen `dr.rto_target`.
7. `migrate` verlangt eine erfolgreiche Sicherung jünger als `migrate.recent_backup_max_age`
   (R-OP-13). Mit einem verwalteten Dienst übernimmt der Anbieter WAL und PITR; `bothy-backupd`
   macht dann nur logische Sicherung, Objekte, Manifest und Test (16, Abschnitt 3).

### Consequences

- Gut, weil PITR den Verlust auf das Archiv-Intervall begrenzt und repo1 eine schnelle lokale
  Wiederherstellung erlaubt.
- Gut, weil der logische Abzug Umzug und Versionswechsel ohne pgBackRest erlaubt.
- Gut, weil der Restore-Test die echte Dauer misst und eine manipulierte oder lückenhafte Sicherung
  meldet, bevor sie gebraucht wird.
- Schlecht, weil ein eigenes Image `bothy-postgres` gebaut, gepinnt und aktualisiert werden muss.
- Schlecht, weil TLS zwischen `postgres` und `backup` aufwendig einzurichten ist; Rückfall ist repo1
  auf einem gemeinsamen Volume mit Spiegel ins Off-site-Ziel.
- Schlecht, weil eine Wiederherstellung Gelöschtes zurückbringt. Gegenmittel: Lösch-Journal
  außerhalb der Datenbank; `bothy restore finalize` führt erledigte Löschaufträge erneut aus
  ([20-datenschutz.md](../spec/20-datenschutz.md), R-DP-04).
- Schlecht, weil eine Wiederherstellung die Sequenz zurücksetzt; die Sync-Epoche fängt das ab
  (ADR 0059).
- Neutral, weil drei Kopien (lokal, off-site physisch, off-site logisch) Speicher kosten; die
  Formel steht in 16, Abschnitt 13.

### Confirmation

- Bau-Ticket M7-01 in [PLAN.md](../PLAN.md): Integrationstest mit zwei Containern für
  `archive-push`, Vollsicherung, PITR-Restore und `pgbackrest verify` auf amd64 und arm64; der
  Datenbank-Container hat keinen Ausgang.
- Bau-Ticket M7-02: alle Sicherungsarten laufen, Manifest vollständig, Kennzahlen im Collector,
  Off-site nur anhängend.
- Bau-Ticket M7-03: Eine manipulierte Kopie (Lücke im Ereignis-Log, falscher Audit-Hash, geänderte
  Datei) fällt je mit der richtigen Prüfung durch; eine saubere Kopie besteht; Dauer gegen
  `dr.rto_target`.
- Alarme `BothyBackupStale`, `BothyWalArchiveStale`, `BothyRestoreTestStale`
  ([alerting.md](../observability/alerting.md)) mit `promtool check rules`.
- Portabilitätstest (16, Abschnitt 11.1, Schritt 4): Sicherung und Wiederherstellungstest auf
  einem frischen Runner grün.

## Pros and Cons of the Options

### pgBackRest, `pg_dump`, restic, Restore-Test

- Gut, weil pgBackRest Voll-, Differenz- und Inkrementalsicherung, Verschlüsselung,
  Prüfsummen mit `verify`, mehrere Repositorys und Aufbewahrung bietet [Q1, belegt] und unter
  MIT steht [Q2, belegt]; restic prüft Stichproben mit `check --read-data-subset` [Q3, belegt].
- Schlecht, weil drei Werkzeuge und ein Steuerdienst zusammenspielen müssen.

### Nur logische Sicherung

- Gut, weil einfach und unabhängig von Version und Anbieter.
- Schlecht, weil der Verlust bis zur letzten Sicherung reicht (Stunden statt Minuten) und große
  Datenbanken lange zum Zurückspielen brauchen.

### WAL-G

- Gut, weil Apache-2.0 [Q4, belegt] und mit vielen Speicherzielen nutzbar.
- Schlecht, weil der Modus mit Repository-Host, über den der Datenbank-Container ohne eigenen
  Ausgang sichert, für pgBackRest belegt ist, für WAL-G hier nicht geprüft [vermutet]; ein Wechsel
  ändert Image und Runbooks.

### Barman

- Gut, weil auf PostgreSQL spezialisiert.
- Schlecht, weil ein eigener Sicherungs-Host üblich ist und das Werkzeug unter GPL-3.0 steht
  [Q5, belegt]; im ausgelieferten Image brächte das Weitergabepflichten in die Release-Kette
  [abgeleitet].

### Snapshots

- Gut, weil schnell und ohne Werkzeug in der Datenbank.
- Schlecht, weil nur absturzkonsistent, ohne PITR, und abhängig von der Plattform; für
  Container-Volumes auf einem einzelnen Host gibt es keinen einheitlichen Snapshot-Weg
  [abgeleitet].

### Nur verwalteter Dienst

- Gut, weil der Anbieter WAL und PITR betreibt.
- Schlecht, weil es auf dem Erstziel (eigener Host) keinen Anbieter gibt und Objekte, Manifest und
  Restore-Test trotzdem fehlen; der Weg bleibt als Variante für Clouds (Punkt 7).

## More Information

- Quellen (abgerufen 2026-10-10): Q1 https://pgbackrest.org/ und
  https://pgbackrest.org/release.html [belegt, wie in 16 zitiert]; Q2
  https://github.com/pgbackrest/pgbackrest/blob/main/LICENSE [belegt]; Q3
  https://restic.readthedocs.io/en/stable/045_working_with_repos.html [belegt]; Q4
  https://github.com/wal-g/wal-g/blob/master/LICENSE.md [belegt]; Q5 https://github.com/EnterpriseDB/barman
  [belegt].
- Offen (16, Abschnitt 8.1): ob Repositorys, die vor pgBackRest 2.59.3 verschlüsselt wurden, neu
  verschlüsselt werden müssen; für neue Instanzen stellt sich die Frage nicht.
- Verwandt: ADR 0006 (PostgreSQL), ADR 0010 (Objektspeicher), ADR 0011 (Secret-Provider),
  ADR 0012 (Compose), ADR 0059 (Sync-Epoche).
