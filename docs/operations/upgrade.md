# Upgrade

Status: Entwurf M0. Leitfaden zu [16-betrieb.md](../spec/16-betrieb.md) 10 und
[18-release.md](../spec/18-release.md). Gilt für Betreiber (Ablauf) und für Entwickler von
Migrationen (Regeln in Abschnitt 4).

## 1. Vor dem Upgrade

1. Release Notes lesen: Abschnitt "Upgrade-Hinweise" sagt, ob Handarbeit nötig ist; Abschnitt
   "Migrationen" nennt Expand- oder Contract-Schritte und ihre erwartete Dauer auf großen Tabellen.
2. Nur eine Hauptversion auf einmal. Innerhalb einer Hauptversion darf eine Instanz Versionen
   überspringen; die Migrationen laufen in Reihenfolge.
3. Signatur prüfen ([release.md](release.md) 5).
4. Sicherung: `migrate` verlangt ohnehin eine erfolgreiche Sicherung jünger als
   `migrate.recent_backup_max_age`. Vor großen Releases zusätzlich von Hand eine Vollsicherung
   auslösen.
5. Mindest-App-Versionen prüfen: Hebt das Release `client.min_version.<plattform>` an, vorher die
   Apps über die Kanäle verteilen.

## 2. Ablauf im Compose-Stack

```
# .env: BOTHY_VERSION auf die neue Version
docker compose pull
docker compose up -d
```

Compose ersetzt `migrate` zuerst; erst nach Exit 0 starten `api` und `worker` neu. Während des
Neustarts sind Clients kurz getrennt; sie bekommen `4013 server_restart` und setzen mit `resume`
fort. Hilfsdienste (Collector, Prometheus usw.) folgen ihren eigenen Versionsvariablen.

Prüfen: `docker compose ps`, `/readyz`, Statusseite, SLO-Dashboard, Version in der Admin-Konsole.

## 3. Rückweg

1. Ein Release zurück: `BOTHY_VERSION` auf die vorige Version, `docker compose up -d`. Das Schema
   bleibt; die Expand/Contract-Regel garantiert, dass der Code von N-1 damit läuft. `migrate`
   der Version N-1 erkennt das neuere, aber kompatible Schema und beendet sich ohne Änderung.
2. Mehr als ein Release zurück: nur per Wiederherstellung auf einen Zeitpunkt vor dem Upgrade
   ([backup-restore.md](backup-restore.md) 5.2). Daten nach diesem Zeitpunkt gehen verloren.

## 4. Regeln für Migrationen (Expand/Contract)

| Nr | Regel | Grund |
|---|---|---|
| M1 | Version N darf nur entfernen oder umdeuten, was Version N-1 nicht mehr liest oder schreibt. | Rückweg um eine Version ohne Datenbank-Rückbau. |
| M2 | Umbenennen nie direkt: neue Spalte (Expand), beide schreiben (N), nur neu lesen (N+1), alte entfernen (N+2). | M1. |
| M3 | Neue Spalten sind `NULL` erlaubt oder haben einen Standard ohne Neuschreiben der Tabelle. | Kein langer Lock auf großen Tabellen. |
| M4 | Indexe mit `CREATE INDEX CONCURRENTLY` in einer Migration ohne Transaktion (goose: `-- +goose NO TRANSACTION`). | Schreiben läuft weiter. |
| M5 | Neue Constraints erst `NOT VALID`, in einer späteren Migration `VALIDATE CONSTRAINT`. | Prüfung ohne Schreibsperre. |
| M6 | Jeder Schritt setzt `lock_timeout` auf `migrate.lock_timeout` und bricht lieber ab, als zu warten. | Ein wartender Lock blockiert alle folgenden Anfragen. |
| M7 | Daten nachfüllen (Backfill) nie in der Migration, sondern als Hintergrundjob in Stapeln. | Kurze Transaktionen; Fortschritt sichtbar. |
| M8 | Aufzählungen sind `text` mit `CHECK`; neue Werte sind additiv (R-DM-02). Code von N-1 muss unbekannte Werte vertragen. | Rückweg. |
| M9 | Nutzlasten von Ereignissen ändern sich nur additiv (`schema_version`, [04-sync-protokoll.md](../spec/04-sync-protokoll.md) 2.2). | Alte Clients lesen weiter. |
| M10 | Partitionen des Ereignis-Logs legt ein Job im Voraus an, nie eine Migration im Betrieb. | Kein DDL-Lock im heißen Pfad. |
| M11 | Migrationen von River (Jobs) laufen im selben `migrate`-Lauf, in fester Reihenfolge vor den eigenen. | Ein Startschritt, ein Ergebnis. |
| M12 | Keine Down-Migrationen in Produktion. Fehler korrigiert eine neue Migration. | Down-Migrationen sind selten getestet und löschen oft Daten. |

Prüfung im Gate (Ticket): Jede Migration läuft gegen das Schema der vorigen Version mit
Testdaten; danach läuft die Testsuite der vorigen Version gegen das neue Schema (Kompatibilität
N-1).

## 5. Schema-Prüfung beim Start

1. Jede Binärversion kennt die kleinste Migration, die sie braucht. Ist die angewandte Migration
   kleiner, meldet `/readyz` `schema_too_old`.
2. `migrate` schreibt nach jedem Lauf `instance.min_server_version`: die älteste Server-Version,
   die mit dem neuen Schema laufen darf. Nach Regel M1 ist das die vorige Version; ein Release,
   das bewusst bricht (Hauptversion), setzt die eigene. Ist die eigene Version kleiner, meldet
   `/readyz` `schema_requires_newer_server` (03, `instance.min_server_version`).
3. Ein neueres Schema ohne diese Sperre ist erlaubt; so funktioniert der Rückweg aus Abschnitt 3.
4. `instance.schema_version` zeigt den Stand in der Admin-Konsole.

## 6. PostgreSQL-Hauptversion

Ein Wechsel der Hauptversion (z. B. 18 auf 19) ist ein eigener Schritt mit Wartungsfenster:

1. Erst wenn ein Release von `bothy-postgres` die neue Hauptversion unterstützt und der
   Portabilitätstest damit grün ist.
2. Standardweg: logischer Abzug und Restore in eine neue Datenbank ([cloud-migration.md](cloud-migration.md)
   mit gleichem Host). `pg_upgrade` ist schneller, braucht aber beide Hauptversionen im selben
   Container; das offizielle Image legt Daten ab Version 18 unter `/var/lib/postgresql/<major>/docker`
   ab, um `pg_upgrade --link` zu erleichtern (https://hub.docker.com/_/postgres, abgerufen
   2026-10-10) [belegt]. Ein eigenes Upgrade-Image ist ein späteres Ticket.
3. Danach sofort Vollsicherung; die alte pgBackRest-Stanza gilt nicht mehr für die neue Version.
