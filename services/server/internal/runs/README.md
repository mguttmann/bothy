# Modul `runs`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Auslöse-Entscheidung, Läufe und Lauf-Dokument, Schritte, Nachträge, Komponenten, Interaktionen, Befehlsausführung, Aufruf-Marken.

## Port (Wurzelpaket `internal/runs`)

- Eingehend: `Runs` (Lesen, Start, Patch, Ende, Abbruch, Übersicht), `Delivery` (Zustellvorbereitung mit I2 und Marke), `Interactions`, `CommandExec`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `messaging.MessagePosted`, `messaging.MessageDeleted`, `agents.AgentDisabled`, kernel `authorize.AccessChanged`.
- Implementiert Quellen-Ports und Querschnitts-Ports: `messaging.ComponentReader`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `chat`, `agents`, `messaging`, `billing`.

Grund: Marke signieren; Bediener, Erwähnungsregel, Projektbindung; Richtlinie und Sitzung; Anker und `trigger_state`; Kosten buchen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`agent.invoke`, `run.read`, `run.overview.read`, `run.start`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.invoke`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked`.
- Kurzlebig: `run.updated`.
- WebSocket-Ops (Client an Server): `run.update`.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `runs.cancel_timeout` | periodisch, `agent.cancel_grace` | `maintenance` |
| `runs.stall_check` | periodisch, `run.stall_after` | `maintenance` |
| `runs.component_expiry` | periodisch, `component.default_ttl` | `maintenance` |

## Hinweise für den Bau

- Zustandsübergänge sind Compare-and-swap auf `state` und `version` (03, `runs`).
- Live-Box-Patches speichern ohne `event_log` und gehen kurzlebig als `run.updated` an fokussierte Verbindungen (04, 11).
- Die Aufruf-Marke entsteht bei jeder Zustellung neu, nach `agent.invoke` und I2 mit aktuellen Rechten; scheitert die Prüfung, wird der Lauf `failed` (fail closed, 04, 15.3).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [09-agenten.md](../../../../docs/spec/09-agenten.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
