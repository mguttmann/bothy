# 17 Observability

Status: Entwurf M0. Normativ für Telemetrie (Traces, Metriken, Logs), Datenschutz der
Telemetrie, Collector, Dashboards, SLOs, Alarme, In-App-Statistik und die Abgrenzung zum
Audit-Log. Dieses Kapitel fasst die Regeln zusammen; die Einzelheiten stehen unter
[docs/observability/](../observability/signals.md).
Begriffe: [00-glossar.md](00-glossar.md). Betrieb: [16-betrieb.md](16-betrieb.md).

## 1. Ziele

1. Ein Betreiber sieht in Minuten, ob Nachrichten ankommen, ob Agenten laufen und ob Push
   funktioniert, und folgt einer Anfrage von der App bis zur Agenten-Laufzeit.
2. Telemetrie verrät keine Inhalte und taugt nicht zur Überwachung einzelner Menschen.
3. Telemetrie blockiert nie den Nachrichtenpfad.
4. Das Backend ist austauschbar: Ein Wechsel ist Konfiguration, kein Code.

## 2. Grundregeln

| Nr | Regel | Quelle |
|---|---|---|
| 1 | Instrumentierung nur mit OpenTelemetry (Go SDK im Server, OTel-SDKs in Python und TypeScript in den Adaptern). | B11 |
| 2 | Export nur an einen OpenTelemetry Collector. Kein Prozess spricht direkt mit einem Hersteller-Backend. | B11, ADR 0034 |
| 3 | Keine Nachrichteninhalte, Prompts, Dateinamen, Suchbegriffe, Tokens oder Geheimnisse in Telemetrie. Durchsetzung an der Quelle und im Collector (Allowlist). | B11, [privacy.md](../observability/privacy.md) |
| 4 | Telemetrie ist asynchron mit begrenzter Warteschlange. Ist sie voll, verwirft der Prozess und zählt `bothy.telemetry.dropped`. Ein Export-Fehler ändert nie das Ergebnis einer Anfrage. Verwirft ein Prozessor des SDK ohne Rückmeldung, nennt das README von `platform/otel` diese ungezählte Stelle. | R-Telemetrie |
| 5 | `gen_ai.*`-Attribute stehen nur in einem Paket je Codebasis (Server, SDK). Ihr Status ist "Development"; ein Wechsel der Konvention ist eine Änderung an genau dieser Stelle. | R-Telemetrie, [signals.md](../observability/signals.md) 6 |
| 6 | Das Audit-Log ist keine Telemetrie: eigene Tabelle, gleiche Transaktion, nie gesampelt, nie über den Collector. | B11, [audit-vs-telemetry.md](../observability/audit-vs-telemetry.md) |
| 7 | Die In-App-Statistik liest eigene Tabellen, nie das Telemetrie-Backend. | [admin-statistics.md](../observability/admin-statistics.md) |
| 8 | Menschen erscheinen in Telemetrie nur als Pseudonym (HMAC mit `telemetry.pseudonym_key_ref`). Metriken tragen nie Personen-Attribute. | [privacy.md](../observability/privacy.md) |
| 9 | Jedes Attribut einer Metrik hat eine endliche, vom Betreiber begrenzte Wertemenge. IDs stehen nur in Spans und Logs. | [signals.md](../observability/signals.md) 2 |

![Telemetrie-Pipeline](../observability/diagrams/telemetry-pipeline.svg)

## 3. Signale

Der Katalog in [signals.md](../observability/signals.md) ist die einzige Liste aller Namen,
Einheiten und Attribute. Kurzfassung:

| Bereich | Kernsignale |
|---|---|
| REST | `http.server.request.duration` (Semantic Conventions, stabil) |
| Sync | `bothy.delivery.duration`, `bothy.gateway.connect.duration`, `bothy.gateway.catchup.duration`, `bothy.event.fanout.lag`, `bothy.db.event_seq.lock_wait` |
| Agenten | `bothy.run.start.latency`, `bothy.run.duration`, `bothy.run.finished`, `bothy.run.steps`, `bothy.run.tokens`, `bothy.run.cost`, `bothy.agent.consumer.lag` |
| Push | `bothy.push.sent`, `bothy.push.skipped`, `bothy.push.fetch` |
| Datenbank | Empfänger `postgresql` im Collector, `db.client.operation.duration`, `bothy.db.pool.connections` |
| Sicherung | `bothy.backup.last_success.timestamp`, `bothy.backup.restore_test.*`, `bothy.storage.volume.*` |

Traces folgen einer Anfrage über W3C Trace Context: App, REST, Ereignis-Log, Fan-out, Adapter,
Laufzeit. Am öffentlichen Listener beginnt der Server einen eigenen Trace und verknüpft einen
eingehenden `traceparent` als Span-Link; Eltern-Kontext wird er nur mit
`telemetry.traces.trust_remote_parent`. `correlation_id` in Ereignissen und Audit ist die Trace-ID
der auslösenden Anfrage; jede Anfrage hat eine, auch mit `telemetry.enabled = false`.
Für die Weitergabe an Adapter trägt der Umschlag das Feld `trace_context` (W3C `traceparent`,
gespeichert in `event_log.trace_context`; 03, 04 Abschnitt 2.2).

![Trace-Weitergabe](../observability/diagrams/trace-propagation.svg)

## 4. Datenschutz

1. Verbot an der Quelle: Instrumentierung setzt nur Attribute aus dem Katalog.
2. Zweite Linie im Collector: `transform/scrub` löscht bekannte Inhaltsfelder, die
   Allowlist (`redaction`) entfernt jeden anderen Schlüssel und ersetzt Werte, die wie Token oder
   E-Mail-Adresse aussehen, durch HMAC-SHA-256.
3. Prüfung: Ein Kanarien-Test schickt Inhalte mit einem Geheimwort durch den echten Pfad und
   sucht das Wort in allen exportierten Daten. Ein Treffer bricht das Gate.
4. Aufbewahrung im lokalen Stack: `BOTHY_PROMETHEUS_RETENTION`, `BOTHY_LOKI_RETENTION`,
   `BOTHY_TEMPO_RETENTION` (Compose-Variablen). Bei externen Backends gilt der Vertrag mit dem
   Anbieter; der Owner trägt ihn ins Verarbeitungsverzeichnis ein (Kapitel 20).

Einzelheiten: [privacy.md](../observability/privacy.md).

## 5. Collector und Backends

Die Basis [infra/otel/collector.yaml](../../infra/otel/collector.yaml) empfängt, filtert und
verteilt. Overlays unter `infra/otel/exporters/` hängen Backends an: `local` (Prometheus, Loki,
Tempo), `newrelic`, `grafana`, `datadog`, `azure-monitor`, `sentry`, `none`. Zwei Overlays lassen
sich kombinieren. Einzelheiten: [collector.md](../observability/collector.md),
[backends.md](../observability/backends.md).

## 6. SLOs und Alarme

| SLO | SLI | Ziel | Alarm |
|---|---|---|---|
| `bothy-api/requests-availability` | REST ohne 5xx | siehe Datei | Page und Ticket |
| `bothy-sync/delivery-latency` | Live-Zustellung innerhalb der Schwelle | siehe Datei | Page und Ticket |
| `bothy-sync/reconnect` | Upgrade bis `ready` innerhalb der Schwelle | siehe Datei | Page und Ticket |
| `bothy-push/provider-acceptance` | Annahme beim Anbieter | siehe Datei | Page und Ticket |
| `bothy-agents/run-start` | `queued` bis `running` innerhalb der Schwelle | siehe Datei | Page und Ticket |
| `bothy-agents/turn-duration` | abgeschlossene Läufe innerhalb der Schwelle | siehe Datei | nur Ticket |

Ziele und Schwellen stehen nur in den Sloth-Dateien unter `infra/observability/slo/`; die
Tabelle in [slos.md](../observability/slos.md) gibt sie wieder und ein Check hält beide gleich.
Burn-Rate-Alarme nach dem Mehrfenster-Verfahren (Startwerte 14,4, 6 und 1; Sloth ergänzt 3).
Alarme gehen in einen Meldungskanal der Instanz und für `page` zusätzlich an einen externen
Kanal; ein Watchdog meldet einen toten Alarmweg. Einzelheiten: [slos.md](../observability/slos.md),
[alerting.md](../observability/alerting.md).

## 7. Dashboards

Sechs Grafana-Dashboards als Dateien unter `infra/observability/grafana/dashboards/`, geladen
per Datei-Provisioning: Server, Zustellung und Sync, Agenten-Läufe, Push, Datenbank und
Sicherung, SLO-Übersicht. Einzelheiten: [dashboards.md](../observability/dashboards.md).

## 8. In-App-Statistik

Die Admin-Konsole zeigt Nutzung je Bereich, Läufe, Werkzeugaufrufe, Kosten und Fehlerraten aus
eigenen Tabellen. Zähler zu Menschen erscheinen nur ab `privacy.stats_min_group_size` ([20-datenschutz.md](20-datenschutz.md)); Werte je Person
gibt es nicht. Einzelheiten: [admin-statistics.md](../observability/admin-statistics.md).

## 9. Apps

1. Es gibt kein offizielles OpenTelemetry-SDK für Dart
   (https://opentelemetry.io/blog/2026/dart-flutter-opentelemetry/, abgerufen 2026-10-10)
   [belegt]. Die Apps senden darum keine Spans. Sie setzen `traceparent` an jeder Anfrage; der
   Server verknüpft ihn als Span-Link, sodass App-Fehlerberichte und Server-Traces auffindbar
   verbunden sind.
2. Trace-ID und Sampling-Entscheid trifft der Server (`telemetry.traces.trust_remote_parent`
   ist standardmäßig aus). Grund: Ein Client darf weder erzwingen, dass alles aufgezeichnet wird,
   noch die `correlation_id` wählen, die im Audit steht.
3. Absturzberichte sendet eine App nur, wenn der Herausgeber der App ein Ziel konfiguriert hat
   (Sentry, [backends.md](../observability/backends.md)). Berichte enthalten keine Inhalte; die
   App entfernt Nachrichtentexte, Eingaben und URLs mit Parametern vor dem Versand (Kapitel 15).

## 10. Tests, die diese Regeln halten

1. Kanarien-Test: Inhalte mit Geheimwort durch API, Adapter und Collector; das Wort darf in keinem
   Export stehen (Gate).
2. Allowlist-Check: Jeder Attributschlüssel im Code steht im Katalog, jeder Schlüssel der
   Collector-Allowlist steht im Katalog.
3. Kardinalitäts-Test: Metrik-Attribute nur aus endlichen Mengen; keine IDs an Metriken.
4. Namens-Test: Jede in Dashboards, Regeln und SLOs genutzte Metrik steht im Katalog.
5. `promtool check rules`, `amtool check-config`, `otelcol-contrib validate`, Grafana-Provisioning
   im Portabilitätstest.
6. SLO-Docs-Check: Ziele und Schwellen in [slos.md](../observability/slos.md) gleich den
   Sloth-Dateien; erzeugte Regeln gleich einer frischen Erzeugung.

## 11. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `telemetry.enabled` | `true` | Telemetrie an oder aus |
| `telemetry.traces.sampler_ratio` | `1.0` | Anteil der Traces, die das SDK aufzeichnet; der Collector sampelt danach nach |
| `telemetry.traces.trust_remote_parent` | `false` | `true`: ein eingehender `traceparent` wird Eltern-Kontext (seine Trace-ID wird `correlation_id`, sein Sampling-Flag gilt); `false`: nur Span-Link, eigener Trace |
| `telemetry.logs.exporter` | `both` | `otlp`, `stdout` oder `both` |
| `telemetry.logs.level` | `info` | kleinste Stufe, die geschrieben wird |
| `telemetry.pseudonym_key_ref` | `file:<secrets.dir>/telemetry_pseudonym_key` | Schlüssel für HMAC-Pseudonyme; mindestens 32 Byte |
| `telemetry.dashboard_url` | leer | Link aus der Admin-Konsole zum Telemetrie-Backend |
| `stats.cache_ttl` | `5m` | so lange gilt ein berechnetes Statistik-Ergebnis |
| `stats.max_range` | `400d` | größter abfragbarer Zeitraum |

## 12. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-OB-01 | Fan-out im Collector über forward-Connectoren; jedes Backend ist ein Overlay mit eigenen Pipelines. | Mehrere Backends ohne Listen-Konflikte beim Zusammenführen; ohne Overlay bleibt die Basis gültig. |
| R-OB-02 | Datenschutz doppelt: Verbot an der Quelle und Allowlist im Collector, geprüft mit Kanarien-Test. | Ein Fehler im Code allein darf keine Inhalte nach außen bringen. |
| R-OB-03 | Personen nur als HMAC-Pseudonym in Spans und Logs, nie an Metriken. | Fehlersuche bleibt möglich, Leistungsüberwachung einzelner Menschen nicht. |
| R-OB-04 | Tail-Sampling im Collector: alle Fehler, alle langsamen, Anteil vom Rest; SDK zeichnet vollständig auf. | Fehler gehen nie durch zufälliges Sampling verloren. |
| R-OB-05 | Der Server entscheidet Trace-ID und Sampling, nicht der Client; ein eingehender `traceparent` ist standardmäßig nur ein Span-Link. | Ein Client kann keine Vollaufzeichnung erzwingen und die `correlation_id` im Audit nicht wählen. |
| R-OB-06 | Histogramme mit festen Grenzen; jede SLO-Schwelle ist eine Grenze. | Ein SLI aus Buckets ist nur an Grenzen exakt. |
| R-OB-07 | `correlation_id` ist die Trace-ID der auslösenden Anfrage. | Ereignisse, Audit und Traces lassen sich ohne zweite ID verbinden. |
| R-OB-08 | Statusseite und Statistik hängen nicht vom Observability-Stack ab. | Der Stack ist optional; Betriebsfunktionen der App nicht. |
| R-OB-09 | Laufzeit-Texte in Ops-Artefakten (Alarm-Texte, Dashboard-Titel) sind Englisch. | Betreiber weltweit; Planungsdokumente bleiben Deutsch. |
