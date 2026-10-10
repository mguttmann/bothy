# Modul `status`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Statusseite, Komponenten, Störungen, automatische Prüfungen.

## Port (Wurzelpaket `internal/status`)

- Eingehend: `Status`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `agents.AgentDisabled`.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `agents`.

Grund: Agenten-Präsenz als Status-Komponente.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`status.public.read`, `status.read`, `status.manage`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `status.updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `status.probe` | periodisch | `maintenance` |

## Hinweise für den Bau

- Öffentliche Statusseite nur bei `status.public_enabled` (05, `status.public.read`).
- Agenten-Komponenten folgen der Präsenz aus `agents`.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [16-betrieb.md](../../../../docs/spec/16-betrieb.md), [17-observability.md](../../../../docs/spec/17-observability.md)
