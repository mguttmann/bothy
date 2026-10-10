# Modul `stats`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Statistik für Konsole: Nutzung je Bereich, Läufe, Werkzeugaufrufe, Kosten, Fehlerraten.

## Port (Wurzelpaket `internal/stats`)

- Eingehend: `Stats`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `chat`, `agents`, `messaging`, `runs`, `billing`.

Grund: liest Kennzahlen über Ports.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

Keine. Das Modul liest über die Ports anderer Module (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`stats.read`, `client_metrics.submit`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hinweise für den Bau

- Kennzahlen ohne Inhalte; Instanz-Sicht nur für Owner, Bereich mit `manage` (05, `stats.read`).
- Abgrenzung zur Telemetrie: `stats` zeigt Fachzahlen in der Konsole, Betriebsmetriken laufen über OpenTelemetry (17-observability.md).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [01-produkt.md](../../../../docs/spec/01-produkt.md), [17-observability.md](../../../../docs/spec/17-observability.md)
