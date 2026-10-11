# Modul `terminal`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Rechner und Kopplung, Erklärung der LLM-Anmeldung (I6), Terminal-Sitzungen, Sitzungs-Marken, Steuerverbindungen der Host-Dienste, Relais der Ströme.

## Code

- `internal/terminal` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

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

`hosts`, `terminal_sessions`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `host.updated`, `terminal_session.changed`.
- Kurzlebig: `host.presence_changed`.
- WebSocket-Ops (Client an Server): keine im Gateway; eigene Kanäle mit eigenen Ops (05, Abschnitt 7).

## Hinweise für den Bau

- Das Modul entsteht in M2 (PLAN, M2-39 bis M2-48). In M1 hält das Register nur seine Tabellen,
  Aktionen, Ereignisse und Abhängigkeiten vollständig; die Terminal-Aktionen bleiben bis M2
  aufgeschoben (PLAN, P-24).
- Terminal-Inhalte stehen nie im Server, in der Telemetrie, im Audit, in der Suche, im Push oder im
  Kontext eines Agenten; der Inhalt läuft Ende-zu-Ende verschlüsselt zwischen Gerät und Rechner,
  der Server ist Relais und Autorisierungspunkt (24-terminal.md, Abschnitte 1 und 5.1).
- Die Ströme laufen über eigene Kanäle mit eigenen Ops, nicht über das Gateway (05-aktionen.md,
  Abschnitt 7); Relais und Host-Verbindungen liegen nur im Adapter `wsrelay`.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [24-terminal.md](../../../../docs/spec/24-terminal.md), [07-rechte.md](../../../../docs/spec/07-rechte.md), [ADR 0068](../../../../docs/adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md)
