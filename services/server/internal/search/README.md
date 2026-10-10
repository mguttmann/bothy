# Modul `search`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Suchabfrage über Nachrichten, Thread-Titel und Dateien mit Rechtefilter, Rangfolge.

## Port (Wurzelpaket `internal/search`)

- Eingehend: `Search`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `chat`, `messaging`, `files`.

Grund: Thread-Titel, Nachrichten, Dateinamen und Transkripte.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

Keine eigenen Tabellen; Daten anderer Module nur über deren Ports.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`search.query`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hinweise für den Bau

- Rechtefilter kommt aus `Authorizer.Scope`; jede Abfrage verknüpft mit `channel_id = ANY(...)` (03, Abschnitt 5). Keine eigene Rechte-Logik.
- Liest Nachrichten, Thread-Titel und Dateien nur über die Ports von `messaging`, `chat`, `files`.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [14-suche.md](../../../../docs/spec/14-suche.md)
