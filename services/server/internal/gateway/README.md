# Modul `gateway`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

WebSocket-Endpunkt, Gateway-Sitzungen, Hub, Fokus, Tippen, Weitergabe kurzlebiger Ereignisse, Agenten-Verbindungen.

## Port (Wurzelpaket `internal/gateway`)

- Eingehend: keiner für andere Module.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `eventlog.EphemeralPublisher`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `chat`, `agents`, `runs`, `sync`.

Grund: Thread zu Kanal für Fokus und Tippen; Verbraucher-Cursor und Präsenz; Lauf-Patches und Zustellvorbereitung; Tail, Nachholen, Momentaufnahme.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

Keine eigenen Tabellen; Daten anderer Module nur über deren Ports.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`gateway.connect`, `gateway.focus`, `typing.start`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: `typing.started`.
- WebSocket-Ops (Client an Server): `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop`.

## Hinweise für den Bau

- Je Verbindung eine Lese- und eine Schreib-Goroutine mit begrenzter Warteschlange; Überlauf schließt mit `4014` (04, 7.2).
- Gateway-Sitzungen leben nur im Prozess; `resume` auf einem anderen Knoten endet mit `4006` (02, Abschnitt 10).
- Beim Shutdown zuerst Agenten-Cursor sichern, dann alle Verbindungen mit `4013` schließen (02, 15.3).
- Einziges Modul, das `github.com/coder/websocket` importieren darf.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [04-sync-protokoll.md](../../../../docs/spec/04-sync-protokoll.md), [06-api.md](../../../../docs/spec/06-api.md)
