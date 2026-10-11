# Modul `importer`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Import-Quellen, Stapel mit Herkunftsschlüssel (`import.apply`), Eigentumsliste und Schreibschutz gespiegelter Kanäle; der Trockenlauf liegt im Werkzeug.

## Code

- `internal/importer` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/importer`)

- Eingehend: `Imports` (Quellen, Stapel über `import.apply`).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`, `agents`, `messaging`, `files`.

Grund: legt Menschen, Struktur, Nachrichten, frühere Sitzungs-Referenzen und Dateien an; setzt den Schreibschutz der gespiegelten Kanäle.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`import_sources`, `import_items`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`import.read`, `import.manage`, `import.apply`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `import.progress`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hinweise für den Bau

- Liest die Quelle nur; Trockenlauf vor dem echten Lauf (ADR 0043).
- `import_items` wird vor dem Anlegen `pending` und in derselben Transaktion wie das Zielobjekt `posted` (03, 4.11).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [22-import-migration.md](../../../../docs/spec/22-import-migration.md)
