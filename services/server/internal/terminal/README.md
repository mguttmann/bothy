# Modul `terminal`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Rechner und Kopplung, Erklärung der LLM-Anmeldung (I6), Terminal-Sitzungen, Sitzungs-Marken, Steuerverbindungen der Host-Dienste, Relais der Ströme.

## Port (Wurzelpaket `internal/terminal`)

- Eingehend: `Hosts` (Verwaltung, Kopplung, Erklärung), `Sessions` (Öffnen, Anhängen, Schließen, Liste), `Relay` (Strom- und Host-Verbindungen, nur im Adapter `wsrelay`).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (Sitzungen beenden).
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.HostSource` (Zustand, Inhaber, LLM-Erklärung, Server-Host des Rechners).

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `identity`, `authz`, `agents`.

Grund: Signer für Sitzungs-Marken (Zweck `terminal`); Menschen, Geräte, Owner, Verbrauch des Step-up; Freigaben auf Rechner und I6; Bereitschaft der Dev-Umgebung (Etappe).

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`hosts`, `host_session_users`, `terminal_sessions`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `host.updated`, `terminal_session.changed`.
- Kurzlebig: `host.presence_changed`.
- WebSocket-Ops (Client an Server): keine im Gateway; eigene Kanäle mit eigenen Ops (05, Abschnitt 7).

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `terminal.sweep` | periodisch | `maintenance` |

## Hinweise für den Bau

- Gebaut ab M2 (PLAN, Block Terminal); bis dahin sind die Aktionen im Katalog aufgeschoben (PLAN P-24).
- Der Server ist Relais und Autorisierungspunkt und liest keinen Terminal-Inhalt (Noise-Strom Ende zu Ende, 24-terminal.md).
- Invariante I6 wird bei jeder Freigabe auf einem Rechner und bei jedem `terminal.open` geprüft (10-backend-richtlinie.md, 24-terminal.md).
- Kein Terminal-Inhalt in Telemetrie, Audit, Suche oder Push.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [24-terminal.md](../../../../docs/spec/24-terminal.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
