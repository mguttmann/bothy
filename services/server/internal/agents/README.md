# Modul `agents`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Agenten, Backend-Richtlinie (Soll), gemeldeter Ist-Zustand, Verbraucher-Cursor, Befehlsregister, Agenten-Sitzungen.

## Port (Wurzelpaket `internal/agents`)

- Eingehend: `Agents` (Verwaltung, Richtlinie), `Presence`, `Consumers` (Cursor, Bestätigung, Übernahme), `Commands` (Register), `Sessions`.
- Gibt In-Tx-Ereignisse aus: `AgentDisabled`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.AgentPolicySource`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `authz`, `chat`.

Grund: Agent-Prinzipal anlegen, Zugangsdaten-Inhaber ist Mensch; implementiert `AgentPolicySource`; Thread für Sitzungen, Geltung von Befehlen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.heartbeat`, `agent.events.consume`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`.

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
