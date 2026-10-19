# Modul `stats`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Statistik für Konsole und Apps: Nutzung je Bereich, Läufe, Werkzeugaufrufe, Kosten, Fehlerraten; eigene Messung (Zähler im Speicher, Flush, Aufbewahrung), SLO-Ampel, Live-Rahmen `stats.live`, Timeline.

## Code

- `internal/stats` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/stats`)

- Eingehend: `Stats` (Abfragen, Timeline, Ampel, Backend-Ansicht), `Live` (Basis-Rahmen je Intervall, Filter je Empfänger).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `measure.Recorder`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `chat`, `agents`, `messaging`, `runs`, `billing`.

Grund: liest Kennzahlen über Ports.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`stats_minutes`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`stats.read`, `client_metrics.submit`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: `stats.live`.
- WebSocket-Ops (Client an Server): `stats.subscribe`, `stats.unsubscribe` (über den Hub des Gateways).

## Hinweise für den Bau

- Kennzahlen ohne Inhalte; Instanz-Sicht nur für Owner, Bereich mit `manage` (05, `stats.read`).
- Abgrenzung zur Telemetrie: `stats` zeigt Fachzahlen in der Konsole, Betriebsmetriken laufen über OpenTelemetry (17-observability.md).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [01-produkt.md](../../../../docs/spec/01-produkt.md), [17-observability.md](../../../../docs/spec/17-observability.md)

## Client metrics (M4-31)

`submitClientMetrics` (`POST /api/v1/client-metrics`, action `client_metrics.submit`, `rl.write`)
takes the app metrics of docs/observability/signals.md 4.6a (ADR 0057):

- `clientmetrics.go` (port): `ClientMetrics` with `Submit` and the batch types.
- `domain/`: the closed list of names and kinds, the per-entry filter (unknown name, mismatched
  `kind`, bucket bounds outside 4.7, negative counts, a negative counter or a non-finite value
  discard only that entry) and the attribute normalization (`platform`, `density`, `theme`,
  `app_major` from `app_version` or `other`; `close_code` only for `client.reconnects`, `other`
  outside the codes of 04 6.4).
- `app/`: `SubmitClientMetrics` reads `privacy.client_metrics` of the person (`403
  feature_disabled` when off), filters, records and answers `202`; it attaches no principal or
  device and writes no table.
- `adapters/httpapi`: the handler of the generated interface; `adapters/otelmetrics`: the sink over
  the instruments `bothy.client.*` and `bothy.client_metrics.discarded`.

Histogram path (docs/DECISIONS.md D-M4-31-1): the buckets go to `metrics.AggregatedSet` in
`internal/platform/otel/metrics`, an `sdkmetric.Producer` on the OTLP metric reader, which exports
them as cumulative OTLP histogram data points with the bounds of 4.7. Count and bucket counts are
exact; the sum is an upper bound (each value at its bucket's upper bound), minimum and maximum are
absent. The app clamps a value above the top bound into the top bucket.
