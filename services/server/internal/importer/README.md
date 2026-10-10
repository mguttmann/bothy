# Modul `importer`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Import-Quellen, Trockenlauf, Import mit Herkunftsschlüssel.

## Port (Wurzelpaket `internal/importer`)

- Eingehend: `Imports`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`, `messaging`, `files`.

Grund: legt Menschen, Struktur, Nachrichten und Dateien an.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`import_sources`, `import_items`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`import.read`, `import.manage`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `import.progress`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `importer.run` | `import.manage` | `import` |

## Hinweise für den Bau

- Liest die Quelle nur; Trockenlauf vor dem echten Lauf (ADR 0043).
- `import_items` wird vor dem Anlegen `pending` und in derselben Transaktion wie das Zielobjekt `posted` (03, 4.11).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [22-import-migration.md](../../../../docs/spec/22-import-migration.md)
