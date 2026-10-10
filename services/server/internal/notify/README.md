# Modul `notify`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Benachrichtigungsstufen, Ruhezeiten, Push-Registrierungen, Push-Verbraucher und Versand.

## Port (Wurzelpaket `internal/notify`)

- Eingehend: `Preferences`, `QuietHours`, `PushRegistrations`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `identity.DeviceRevoked`, `identity.SessionEnded` (Registrierungen der Sitzung deaktivieren), `chat.ChannelArchived`, `chat.SpaceArchived`.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`, `messaging`, `sync`.

Grund: Geräte und Zeitzone; Ziel-Pfad für Einstellungen; Erwähnungen; Gelesen-Stand.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`notification_preferences`, `quiet_hours`, `push_registrations`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`notification_pref.update`, `quiet_hours.update`, `push.register`, `push.unregister`, `push.deliver`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `notification_pref.updated`, `quiet_hours.updated`.
- Kurzlebig: `notification.created`.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `notify.push_send` | Verbraucher `push` | `delivery` |

## Hinweise für den Bau

- Push trägt nie Inhalt; vor jedem Versand prüft der Job `push.deliver` (05, R-AZ-11).
- Der Verbraucher `push` erzeugt Jobs und rückt seinen Cursor in derselben Transaktion vor (02, 11.5).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [13-push-benachrichtigungen.md](../../../../docs/spec/13-push-benachrichtigungen.md)
