# Modul `sync`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Ereignis-Log (Anhängen, Tail, Nachholen, Schwärzen, Partitionen), Momentaufnahme, Idempotenz-Schlüssel, Gelesen-Stand, Entwürfe, persönliche Einstellungen, Cursor der Worker-Verbraucher.

## Port (Wurzelpaket `internal/sync`)

- Eingehend: `Tail`, `CatchUp`, `Snapshot`, `Consumers` (Worker-Cursor), `ReadStates`, `Drafts`, `Settings`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `messaging.MessagePosted` (Entwurf leeren, Erwähnungszähler), `chat.ThreadMoved`.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `eventlog.Appender`, kernel `idempotency.Store`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `chat`, `agents`, `messaging`, `runs`.

Grund: Zähler `event_seq`; Momentaufnahme aus Struktur, Präsenz, laufenden Läufen; Abo auf Nachrichten.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`event_log`, `idempotency_keys`, `read_states`, `drafts`, `user_settings`; beantragt: `event_consumer_cursors`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `sync.partition_maintenance` | periodisch, Partitionen anlegen und nach `sync.event_retention` abhängen | `maintenance` |
| `sync.idempotency_cleanup` | periodisch | `maintenance` |

## Hinweise für den Bau

- Implementiert `kernel/eventlog.Appender`: Nummern aus `event_seq` in Phase 3.2 der Unit of Work, danach `pg_notify` nur mit der Nummer (02, 8.1).
- Der Log-Tail nutzt eine eigene, nicht gepoolte Verbindung für `LISTEN` (02, 11.2).
- Die Momentaufnahme enthält keine Thread-Inhalte (04, 6.3).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [04-sync-protokoll.md](../../../../docs/spec/04-sync-protokoll.md), [15-client-architektur.md](../../../../docs/spec/15-client-architektur.md)
