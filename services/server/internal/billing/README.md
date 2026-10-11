# Modul `billing`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Preistabelle, Kostenbuchungen, Summen je Lauf, Thread, Bereich, Agent, Abgleich.

## Code

- `internal/billing` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/billing`)

- Eingehend: `Costs` (Buchen, Korrektur, Summen), `Prices`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: keine.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: keine.

Grund: Kostenzeilen tragen Kopien von Thread, Kanal, Bereich und Agent (03); keine Rückfrage nötig.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`price_table`, `cost_entries`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile`, `cost.reconcile.submit`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `billing.reconcile` | `cost.reconcile` oder periodisch, falls eingeschaltet | `default` |

## Hinweise für den Bau

- Beträge in Mikro-Einheiten mit Währung; Korrekturen nur als neue Zeile mit `supersedes_id` (03, 4.9).
- Bei `billing_mode = personal` sind Beträge nur Information.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
