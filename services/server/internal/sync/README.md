# Modul `sync`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Ereignis-Log (Anhängen, Tail, Nachholen, Schwärzen, Partitionen), Momentaufnahme, Idempotenz-Schlüssel, Gelesen-Stand, Entwürfe, persönliche Einstellungen, Cursor der Worker-Verbraucher.

## Code

- `internal/sync` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/sync`)

- Eingehend: `Tail`, `CatchUp`, `Snapshot`, `Consumers` (Worker-Cursor), `ReadStates`, `Drafts`, `Settings`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `messaging.MessagePosted` (Entwurf leeren), `chat.ThreadMoved` (Kopien von `channel_id`).
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `eventlog.Appender`, kernel `eventlog.Redactor`, kernel `idempotency.Store`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `identity`, `chat`, `agents`, `messaging`, `runs`.

Grund: Zähler `event_seq`; Prinzipal der Momentaufnahme (Handle, Anzeigename); Momentaufnahme aus Struktur, Präsenz, laufenden Läufen; Abo auf Nachrichten.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`event_log`, `event_consumer_cursors`, `idempotency_keys`, `read_states`, `drafts`, `user_settings`.

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
