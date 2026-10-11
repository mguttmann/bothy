# Alarme

Status: Entwurf M0. Regeln: [infra/observability/prometheus/rules/](../../infra/observability/prometheus/rules/)
(SLO-Regeln erzeugt, `bothy-alerts.yaml` von Hand). Wege:
[infra/observability/alertmanager/alertmanager.yaml](../../infra/observability/alertmanager/alertmanager.yaml).

## 1. Stufen und Wege

| Stufe | Bedeutung | Weg |
|---|---|---|
| `page` | Menschen spüren es jetzt oder bald; sofort handeln | externer Kanal (`external-oncall`) und Meldungskanal der Instanz (`bothy-ops`) |
| `ticket` | handeln an einem der nächsten Arbeitstage | Meldungskanal der Instanz |
| `heartbeat` | Watchdog; feuert immer | externer Herzschlag-Dienst |

Warum zwei Wege: Ein Ausfall des Servers darf seine eigene Meldung nicht verschlucken. Der
Watchdog meldet den Ausfall von Prometheus oder Alertmanager: Bleibt sein Herzschlag beim externen
Dienst aus, meldet dieser. Die Wahl dieses Dienstes trifft der Betreiber.

Routing lokal geprüft mit `amtool config routes test` (Alertmanager v0.34.1) am 2026-10-10: `page`
geht an `external-oncall` und `bothy-ops`, `ticket` an `bothy-ops`, `Watchdog` an
`external-heartbeat` [belegt].

## 2. Meldungskanal in der Instanz

1. Der Owner legt einen Kanal "Betrieb" an (nur Owner und Vertretung mit `read`) und dort einen
   Eingangs-Webhook ([05-aktionen.md](../spec/05-aktionen.md) `webhook_in.create`).
2. Alertmanager sendet sein eigenes JSON-Format. Der Eingangs-Webhook braucht dafür ein Format
   `alertmanager`, das daraus eine Nachricht macht (Alarmname, Stufe, Zusammenfassung,
   Runbook-Link, Zustand firing oder resolved). Das ist eine Anforderung an Kapitel 06 und 11
   (Bericht, Ticket).
3. Die URL des Webhooks ist ein Geheimnis und liegt in
   `secrets/alertmanager/alertmanager_bothy_webhook_url`. Alertmanager erreicht den Server über den
   internen Listener (`http://api:8081`).
4. Nachrichten aus Eingangs-Webhooks lösen nie Agenten aus (`origin = app`).

## 3. Alarme

### 3.1 SLO-Alarme (erzeugt)

| Alarm | SLO | Stufen | Erste Schritte |
|---|---|---|---|
| `BothyApiAvailabilityBudgetBurn` | `bothy-api/requests-availability` | page, ticket | Dashboard Server: Routen mit 5xx; Logs `ERROR`; Datenbank erreichbar? |
| `BothyDeliveryLatencyBudgetBurn` | `bothy-sync/delivery-latency` | page, ticket | Dashboard Delivery: Fan-out-Verzug, Warten am Sequenz-Zähler, CPU |
| `BothyReconnectBudgetBurn` | `bothy-sync/reconnect` | page, ticket | Rand (Proxy, Tailscale) und Zertifikat; Anmeldung (OIDC erreichbar?) |
| `BothyPushAcceptanceBudgetBurn` | `bothy-push/provider-acceptance` | page, ticket | Push-Dashboard: Ergebnis je Anbieter; Schlüssel abgelaufen? Statusseite des Anbieters |
| `BothyRunStartBudgetBurn` | `bothy-agents/run-start` | page, ticket | Agenten-Dashboard: Heartbeat, Verbraucher-Rückstand, Ablehnungen, Modus-Abweichungen |
| `BothyTurnDurationBudgetBurn` | `bothy-agents/turn-duration` | ticket | Turn-Dauer je Agent; Modell oder Anbieter langsam? Kein Eingriff am Server nötig |

### 3.2 Weitere Alarme (`bothy-alerts.yaml`)

Schwellen sind Startwerte. Wo eine Schwelle einem Konfigurationswert entspricht, steht er in der
Tabelle; die Regeldatei muss ihm folgen. `make ops-check` hält das (Check `alert-thresholds`):
Steht `` `<name>` `` in der Spalte, ist die Schwelle des letzten Vergleichs `> <zahl>` in Sekunden
(eine schlichte Zahl, Kommentare zählen nicht) gleich dem Standard des Werts in der [Konfigurationsreferenz](../operations/configuration.md);
steht ``über `<name>` ``, ist sie echt größer. Die Alarme der Tabelle sind genau die Alarme von
`bothy-alerts.yaml`.

| Alarm | Stufe | Bedingung | Konfigurationswert |
|---|---|---|---|
| `Watchdog` | heartbeat | immer | |
| `BothyServerTelemetryMissing` | page | keine `target_info` des Servers seit 10 Minuten | |
| `BothyCollectorDown` | page | Collector antwortet nicht | |
| `BothyTelemetryExportFailing` | ticket | Exporter des Collectors scheitert | |
| `BothyTelemetryDropped` | ticket | Server verwirft Telemetrie | |
| `BothyEventFanoutLag` | page | Fan-out-Verzug über 5 s | `status.fanout_lag_degraded` |
| `BothyAgentConsumerLag` | ticket | über 1000 Ereignisse Rückstand des Verbrauchers | |
| `BothyAgentOffline` | ticket | kein Heartbeat seit 5 Minuten | über `agent.presence_timeout` |
| `BothyBillingModeMismatch` | page | Ist-Modus weicht vom Soll ab (I3) | |
| `BothyAuthzError` | page | Rechteprüfung meldet einen Programmfehler (`authz_not_checked`, `unknown_action`) oder einen Unterschied zwischen SQL-Filter und `authorize` (`filter_mismatch`) | |
| `BothySecretRefreshFailing` | ticket | erneutes Lesen eines Geheimnisses scheitert | |
| `BothyMalwareScanFailing` | ticket | nur Fehler, kein Ergebnis | |
| `BothyBackupStale` | page | keine Sicherung seit 26 h | `backup.max_age` |
| `BothyWalArchiveStale` | page | kein WAL-Segment seit 15 Minuten | über `backup.pg.archive_timeout` |
| `BothyRestoreTestStale` | ticket | kein Wiederherstellungstest seit 8 Tagen | `backup.restore_test.max_age` |
| `BothyAuditChainBroken` | page | Prüfung der Hash-Kette scheitert | |
| `BothyVolumeFull` | page | unter 10 Prozent frei | |
| `BothyDatabaseConnectionsHigh` | ticket | über 80 Prozent von `max_connections` | |
| `BothyDatabaseDeadlocks` | ticket | Deadlock gemeldet | Sperrreihenfolge [03-datenmodell.md](../spec/03-datenmodell.md) 6.3 |

## 4. Runbook-Anker

Jede Regel nennt `runbook_url` auf einen Anker dieser Datei (Alarmname in Kleinbuchstaben). Die
ersten Schritte stehen in 3.1; für 3.2:

### watchdog
Kein Handeln nötig. Fehlt der Herzschlag beim externen Dienst: Prometheus und Alertmanager prüfen.

### bothyservertelemetrymissing
`docker compose ps`; läuft `api`? Wenn ja: Collector und `OTEL_EXPORTER_OTLP_ENDPOINT` prüfen.

### bothycollectordown
`docker compose logs otel-collector`; häufig ein fehlendes Geheimnis eines Overlays.

### bothytelemetryexportfailing
Ziel des Overlays erreichbar? Schlüssel gültig? Bis zur Lösung gehen Daten dieses Backends
verloren; andere Overlays laufen weiter.

### bothytelemetrydropped
Collector überlastet oder nicht erreichbar; `memory_limiter` und Exporter prüfen.

### bothyeventfanoutlag
Datenbank-Last (Dashboard Database), lange Transaktionen, `pg_stat_activity`. Ein `api`-Prozess
hängt: neu starten.

### bothyagentconsumerlag
Adapter verarbeitet oder bestätigt nicht. Adapter-Logs, `ack`-Verhalten, Verbindung.

### bothyagentoffline
Adapter-Prozess und seine Laufzeit prüfen; Status der Komponente des Agenten.

### bothybillingmodemismatch
Sofort prüfen: Läuft ein Agent mit `personal` gegen einen Organisationsschlüssel oder umgekehrt?
Konfiguration der Laufzeit des Adapters; nichts am Server ändern, bevor die Ursache klar ist.

### bothyauthzerror
Als Fehler mit Sicherheitsbezug behandeln: Eine Route hat ohne ihre Rechteprüfung geantwortet
(die Antwort wurde durch `500` ersetzt) oder eine unbekannte Aktion wurde geprüft. Logs mit der
`correlation_id` und dem Routen-Muster lesen; die betroffene Version nicht weiter ausrollen.

### bothysecretrefreshfailing
Der Server nutzt weiter den alten Wert. Welcher Verweis scheitert, zeigt
`docker compose exec api bothy config check --secrets` (je Schlüssel `ok` oder der Fehlercode);
die Logzeile nennt nur den Fehlercode. Datei im Secret-Provider prüfen: vorhanden, Modus `0400`
oder `0600`, Besitzer der Dienstbenutzer (SEC-SEC-1). Bis zur Behebung wirkt eine Rotation nicht.

### bothymalwarescanfailing
`docker compose logs clamav`; Signaturen geladen? Speicher reicht? Uploads bleiben bis dahin im
Zustand `pending` und werden nicht ausgeliefert.

### bothybackupstale
`backup_runs` in der Admin-Konsole; Off-site-Ziel erreichbar? Platz im Repository?

### bothywalarchivestale
pgBackRest `check` im Container `backup`; TLS zwischen `postgres` und `backup`; Platz in `pg_wal`.
Füllt sich `pg_wal`, hält PostgreSQL an (PANIC), Daten bleiben erhalten
(https://www.postgresql.org/docs/18/continuous-archiving.html, abgerufen 2026-10-10) [belegt].

### bothyrestoreteststale
Letzten Testlauf in `backup_runs` ansehen; welche Prüfung scheitert?

### bothyauditchainbroken
Als Sicherheitsvorfall behandeln: Wer hatte Schreibzugriff auf die Datenbank? Letzte gute Stelle
der Kette bestimmen, Sicherungen vergleichen, Owner informieren.

### bothyvolumefull
Welches Volume? `files`: Aufbewahrung, Kontingente; `pgdata`: Ereignis-Log-Partitionen, Bloat;
`pgbackrest-repo`: lokale Aufbewahrung.

### bothydatabaseconnectionshigh
`database.max_conns` je Prozess mal Prozesse gegen `max_connections`; hängende Verbindungen.

### bothydatabasedeadlocks
Logs nach `deadlock detected`; die beteiligten Abfragen gegen die Sperrreihenfolge prüfen.

## 5. Prüfen

`make ops-check` prüft Regeln und Wege im Gate ([tools/opscheck](../../tools/opscheck/README.md)),
mit `promtool` und `amtool` in der Version der Images:

1. `promtool check rules` für alle Regeldateien (Check `prometheus-rules`) und
   `promtool check config` mit den geladenen Regeln (Check `prometheus-config`); jede Regeldatei
   muss über `rule_files` geladen werden.
2. `promtool test rules` mit
   [bothy-alerts.test.yaml](../../infra/observability/prometheus/tests/bothy-alerts.test.yaml)
   (check `prometheus-rule-tests`): every alert of `bothy-alerts.yaml` fires in one case at the
   first evaluation at which it may, with labels and texts: once its `for` has run out, or, for
   an alert without `for`, as soon as its condition holds. One minute earlier it is still quiet
   (Watchdog fires from the first evaluation on). Every alert with a threshold stays quiet in one
   case exactly at the threshold. An alert without a firing case fails the check.
3. `amtool check-config` und die Wege aus Abschnitt 1 mit `amtool config routes test` (Check
   `alertmanager`): `page` an `external-oncall` und `bothy-ops`, `ticket` an `bothy-ops`,
   `Watchdog` an `external-heartbeat`.
   Templates are parsed too: every `templates` entry must lie under `/etc/alertmanager/`, where
   Compose mounts `infra/observability/alertmanager/`, and match a file.
4. Die Schwellen aus 3.2 gegen die Konfigurationsreferenz (Check `alert-thresholds`). Nennt eine
   Zelle der Spalte "Konfigurationswert" einen Wert der Referenz, muss sie genau `` `<name>` `` oder
   ``über `<name>` `` lauten; jede andere Form ist ein Befund.
