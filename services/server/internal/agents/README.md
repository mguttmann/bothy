# Modul `agents`

Status: Gerüst (M1-05). Das Port-Paket existiert; Schnittstellen und Anwendungsfälle kommen mit den Tickets der Etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Agenten, Backend-Richtlinie und Werkzeugklasse (Soll), gemeldeter Ist-Zustand, Verbraucher-Cursor mit Fencing, Befehlsregister, Agenten-Sitzungen, Dev-Umgebungen (Etappe).

## Code

- `internal/agents` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `domain/`, `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Port (Wurzelpaket `internal/agents`)

- Eingehend: `Agents` (Verwaltung, Richtlinie, Werkzeugklasse), `Presence`, `Consumers` (Cursor, Bestätigung, Übernahme mit Fencing), `Commands` (Register), `Sessions`, `DevEnvironments` (Etappe).
- Gibt In-Tx-Ereignisse aus: `AgentDisabled`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.AgentPolicySource` (Richtlinie, Werkzeugklasse, Bereitschaft der Dev-Umgebung), kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `authz`, `chat`.

Grund: Agent-Prinzipal anlegen, Zugangsdaten-Inhaber ist Mensch; implementiert `AgentPolicySource`; Thread für Sitzungen, Geltung von Befehlen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`, `dev_environments`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.tool_class.set`, `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`, `dev_environment.read`, `dev_environment.manage`, `dev_environment.report`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `agent_session.changed`, `command.changed`, `agent.updated`.
- Kurzlebig: `agent.presence_changed`.
- WebSocket-Ops (Client an Server): `ack`.

## Hinweise für den Bau

- Heartbeat-Zustand lebt im Speicher und wird höchstens je `agent.heartbeat_persist_interval` gespeichert (03, 4.4).
- `acked_seq` wird spätestens je `agent.ack_persist_interval` gespeichert (04, 15.2); bei mehreren Knoten mit Fencing (02, Abschnitt 16).
- Umbinden einer Agenten-Sitzung ist ein Compare-and-swap auf `agent_sessions.version`.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [09-agenten.md](../../../../docs/spec/09-agenten.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
