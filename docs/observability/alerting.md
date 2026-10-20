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
`external-heartbeat` [belegt]. As built (M7-20): `make ops-check` holds the three routes in the
gate (section 5 item 3), and `make alert-e2e` proves the delivery end to end (section 5 item 5).
The three receivers read their URLs only from secret files (`url_file`), never from the
configuration.

## 2. Meldungskanal in der Instanz

As built (M7-20). The guide with the commands is
[docs/operations/alerting-setup.md](../operations/alerting-setup.md).

1. The owner creates, in a session with step-up (console or desktop app; an API token cannot step
   up, [08-identitaet.md](../spec/08-identitaet.md) 12), a space and in it the text channel
   "Operations" that only owners and their deputies read (no grant for anyone else), installs an
   app with the scope `messages:write` (`app.install`), grants it `write` on the channel and
   creates an incoming webhook of that app in the channel
   ([05-aktionen.md](../spec/05-aktionen.md) `channel.create`, `webhook_in.create`). No tool
   creates these: they need the step-up of a person (07 SR5).
2. Alertmanager sends its own unchanged JSON payload. The server recognises the format per
   request from the body (format `alertmanager`, [webhooks.md](../api/webhooks.md) section 8),
   so the URL that `createIncomingWebhook` returns serves both `native` and `alertmanager`; no
   format is chosen when the webhook is created. Each notification becomes one message with
   alert name, severity, summary, runbook link and the state firing or resolved. Alertmanager in
   the stack uses the same path `/api/v1/hooks/{webhook_id}/{token}` on
   `server.internal_listen_addr` (point 3).
3. The URL of the webhook is a secret. `infra/compose/alert-setup.sh channel` reads it from
   standard input (hidden on a terminal), keeps the path `/api/v1/hooks/<id>/<token>`, replaces
   the origin by the internal listener `http://api:8081` (option `--internal-base`), writes
   `infra/compose/secrets/alertmanager_bothy_webhook_url` with mode `0600` and prints only
   `<created|updated|kept> <name> sha256:<12 hex>`, the fingerprint form of `secrets-init`
   ([16-betrieb.md](../spec/16-betrieb.md) 5.3); a second run with the same URL changes nothing.
   `alert-setup.sh external` and `alert-setup.sh heartbeat` write the URLs of the external
   on-call channel and of the heartbeat service unchanged. `secrets-init` copies the three files
   into the subdirectory `alertmanager` of the volume `secrets` (files `0444`, as for every
   third-party service), which Alertmanager mounts read-only at `/run/bothy/secrets`. Alertmanager
   reaches the server only through the internal listener; there is no public path.
4. Alertmanager v0.34.1 starts with a missing `url_file` and fails only when it sends (checked
   2026-10-20 with the image `prom/alertmanager:v0.34.1`), which would drop every alert in
   silence. So it starts through the guard `infra/observability/alertmanager/start.sh`: while one
   `url_file` of the configuration is missing, unreadable or empty, it exits with a message that
   names the files (never a value) and the commands that fix it.
5. Nachrichten aus Eingangs-Webhooks lösen nie Agenten aus (`origin = app`).

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
| `BothyEventConsumerLag` | page | the oldest unprocessed event of a worker consumer is older than `consumer.pending_alert_after` | `consumer.pending_alert_after` |
| `BothyAgentConsumerLag` | ticket | über 1000 Ereignisse Rückstand des Verbrauchers | |
| `BothyAgentOffline` | ticket | kein Heartbeat seit 5 Minuten | über `agent.presence_timeout` |
| `BothyBillingModeMismatch` | page | reported mode differs from the configured mode, according to the heartbeat for longer than the threshold or at a run (I3) | `billing.mismatch_alert_after` |
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

### bothyeventconsumerlag
A worker consumer (`push`, `webhooks`; label `bothy_consumer_name`) stands still: pushes or
outgoing webhooks no longer go out. Is a process of the role `worker` or `all` running? Read the
lines `event consumer round failed` (with `error.type`) and `event consumer lost events to
retention`; is the database reachable? A handler that keeps failing stalls only its own consumer,
which catches up by itself after the fix. Never move the cursor in `event_consumer_cursors` by
hand: the events it jumps over are lost for that consumer for good. A consumer that stands longer
than `sync.event_retention` loses events ([02-architektur.md](../spec/02-architektur.md) 11.5).

### bothyagentconsumerlag
Adapter verarbeitet oder bestätigt nicht. Adapter-Logs, `ack`-Verhalten, Verbindung.

### bothyagentoffline
Adapter-Prozess und seine Laufzeit prüfen; Status der Komponente des Agenten.

### bothybillingmodemismatch
Sofort prüfen: Läuft ein Agent mit `personal` gegen einen Organisationsschlüssel oder umgekehrt?
Konfiguration der Laufzeit des Adapters; nichts am Server ändern, bevor die Ursache klar ist.
The heartbeat cause comes from the gauge of the heartbeat comparison (`bothy_agent_billing_mode_mismatch_age_seconds`): the agent card (`getAgent`) names the kind in `presence.warnings` (`billing_mode_mismatch`, `billing_mode_unsupported`, `billing_evidence_unknown`), and the audit entries `agent.heartbeat` show when the reported mode changed.

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
5. `make alert-e2e` ([tools/observability/alert_e2e.py](../../tools/observability/alert_e2e.py),
   not in `make check`, [gates.md](../quality/gates.md)) proves the path end to end against the
   Compose smoke stack of M1-73 with the profile `observability` and the test overlay
   `infra/compose/test/compose.alert-e2e.yaml`: Alertmanager refuses to start without its URL
   files; the owner sets up the channel "Operations", the app and the incoming webhook with
   step-up; `alert-setup.sh` writes the three URLs (the external ones point to a local stub);
   Prometheus loads the test rules `infra/compose/test/alert-e2e/rules.yaml` (one `page`, one
   `ticket` alert, only in the overlay) next to the real ones. The channel shows a firing message
   per test alert with name, level and summary and origin `app`, then a resolved message after
   the rules stop; the external stub gets the `page` alert and never the `ticket` alert; the
   heartbeat stub gets the Watchdog. Not proven: a real external on-call or heartbeat service.
