# Modul `messaging`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Nachrichten, Revisionen, Erwähnungen, Reaktionen, Anheften, Vorlagen, Kanal-Export.

## Code

- `internal/messaging` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/messaging`)

- Eingehend: `Messages` (Anlegen, Thread mit erster Nachricht anlegen, Bearbeiten, Löschen, Verlauf, `SetTriggerState`, Anker), `Reactions`, `Pins`, `Templates`, `Mentions` (Liste, ungelesene je Thread), `Export`; deklariert `ComponentReader`.
- Gibt In-Tx-Ereignisse aus: `MessagePosted`, `MessageDeleted`.
- Abonniert In-Tx-Ereignisse: `chat.ThreadMoved`.
- Implementiert Quellen-Ports und Querschnitts-Ports: `files.AttachmentLocator`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`, `files`.

Grund: Erwähnungen auflösen; Thread-Sequenz und Wiederöffnen; Anhänge binden und lesen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`messages`, `message_revisions`, `mentions`, `reactions`, `pins`, `message_templates`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`thread.create`, `message.read`, `message.create`, `message.update`, `message.delete`, `message.revisions.read`, `reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`, `mention.list`, `template.read`, `template.manage`, `export.channel`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `message.created`, `message.updated`, `message.deleted`, `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hinweise für den Bau

- Idempotenz über das Paar aus Autor und `client_msg_id` (04, 5.1).
- Löschen leert den Inhalt und schwärzt die Nutzlast der Ereignisse in derselben Transaktion (03, 6.5).
- Eine Erwähnung eines Prinzipals ohne Leserecht wird gespeichert, löst aber nichts aus (03, `mentions`).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [06-api.md](../../../../docs/spec/06-api.md), [14-suche.md](../../../../docs/spec/14-suche.md)
