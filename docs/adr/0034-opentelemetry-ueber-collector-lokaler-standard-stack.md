---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team F (Betrieb)
consulted: Team A (Architektur), Team C (Sicherheit), Team D (Agenten-Plattform)
informed: alle Teams
---

# OpenTelemetry über einen Collector, lokaler Standard-Stack

## Context and Problem Statement

B11 legt OpenTelemetry fest und verlangt Export nur über einen Collector, ohne Inhalte. Betreiber
nutzen unterschiedliche Backends (Grafana, New Relic, Datadog, Azure Monitor, Sentry) oder keins.
Eine Einzelinstanz auf einem Mac soll trotzdem sofort Dashboards und Alarme haben. Wie fließt
Telemetrie, und was ist der Standard?

## Decision Drivers

* Backend-Wechsel ohne Code
* Datenschutz unabhängig vom Backend durchgesetzt (keine Inhalte, Pseudonyme)
* Telemetrie blockiert nie den Nachrichtenpfad
* eine Einzelinstanz bekommt ohne externes Konto Dashboards, SLOs und Alarme
* mehrere Backends gleichzeitig möglich (z. B. Metriken in Grafana, Traces auch in Sentry)

## Considered Options

* SDKs exportieren an einen Collector; Backends als Overlays des Collectors; lokaler Standard-Stack
  aus Prometheus, Loki, Tempo, Grafana und Alertmanager als Compose-Profil
* SDKs exportieren direkt an das jeweilige Backend
* Klassisch ohne OpenTelemetry: Prometheus-Scrape von `/metrics` und strukturierte Logs

## Decision Outcome

Chosen option: "Collector mit Overlays und lokalem Standard-Stack", weil nur der Collector eine
Stelle bietet, an der Datenschutz-Filter, Sampling und Temporalität für jedes Backend gleich
gelten.

1. Basis [infra/otel/collector.yaml](../../infra/otel/collector.yaml): OTLP-Empfang,
   `memory_limiter`, Inhaltsfilter, Allowlist mit HMAC-Pseudonymen, Tail-Sampling, `batch`,
   Verteilung über `forward`-Connectoren.
2. Overlays unter `infra/otel/exporters/` hängen eigene Pipelines an; zwei lassen sich kombinieren.
3. Lokaler Standard: Compose-Profil `observability` und Overlay `local`. Metriken gehen per OTLP an
   Prometheus (kumulativ), Logs an Loki, Traces an Tempo.
4. Die Statusseite und die In-App-Statistik hängen nicht vom Stack ab.

### Consequences

* Gut, weil ein Backend-Wechsel eine Zeile in `.env` und eine Geheimnis-Datei ist.
* Gut, weil Datenschutz einmal geprüft wird (Kanarien-Test gegen den echten Collector).
* Gut, weil der Server nur einen Export-Pfad kennt (OTLP an den Collector).
* Schlecht, weil der Collector ein weiterer Dienst ist, dessen Ausfall Telemetrie kostet (nicht
  den Betrieb).
* Schlecht, weil Tail-Sampling alle Spans eines Traces im selben Collector braucht; mehrere
  Collector brauchen Lastverteilung nach Trace-ID.
* Neutral, weil der lokale Stack eine Instanz bedient und keine Hochverfügbarkeit hat.

### Confirmation

* `otelcol-contrib validate` der Basis allein und mit jedem Overlay im Gate (am 2026-10-10 mit
  0.162.0 lokal grün, auch drei Paare).
* Kanarien-Test durch den echten Collector (am 2026-10-10 lokal: kein Inhalt im Export,
  E-Mail und Bearer-Wert als HMAC).
* Namens-Test: Metriken in Dashboards, Regeln und SLOs stehen im Katalog
  ([signals.md](../observability/signals.md)).
* Grafana-Provisioning und `promtool check` im Portabilitätstest.

## Pros and Cons of the Options

### Collector mit Overlays

* Gut, weil Filter, Sampling und Temporalität an einer Stelle liegen.
* Gut, weil Hersteller-Exporter (Datadog, Azure) nur im Collector gebraucht werden.
* Schlecht, weil Listen beim Zusammenführen ersetzt werden; Overlays müssen eigene Pipelines
  nutzen (gelöst mit `forward`-Connectoren).

### Direkter Export aus den SDKs

* Gut, weil ein Dienst weniger.
* Schlecht, weil jeder Prozess Zugangsdaten jedes Backends bräuchte.
* Schlecht, weil Datenschutz-Filter in jeder Sprache (Go, Python, TypeScript) gleich gebaut
  werden müssten.

### Klassisch ohne OpenTelemetry

* Gut, weil Prometheus-Scrape einfach und verbreitet ist.
* Schlecht, weil Traces über App, Server, Adapter und Laufzeit fehlen.
* Schlecht, weil B11 OpenTelemetry verlangt.

## More Information

* Regeln: [17-observability.md](../spec/17-observability.md); Backends:
  [backends.md](../observability/backends.md); Collector: [collector.md](../observability/collector.md).
* Prometheus als OTLP-Ziel: https://prometheus.io/docs/guides/opentelemetry/ (abgerufen
  2026-10-10).
* Grafana-Stack statt Alternativen: Prometheus, Loki, Tempo und Grafana nehmen OTLP direkt an und
  sind als Images für amd64 und arm64 verfügbar (Docker-Hub-Abfrage am 2026-10-10). Andere
  Komplett-Stacks wurden nicht verglichen; ein Wechsel wäre nur ein neues Overlay.
