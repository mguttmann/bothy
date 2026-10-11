# Module `runs`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Trigger decision, runs and run document, steps, follow-ups, components, interactions, command execution, invocation tokens.

## Port (root package `internal/runs`)

- Inbound: `Runs` (read, context, start, resume, patch, end, cancel, overview), `Delivery` (delivery preparation with I2, guard rules and token), `Interactions`, `CommandExec` (including autocompletion).
- Emits in-tx events: none.
- Subscribes to in-tx events: `messaging.MessagePosted`, `messaging.MessageDeleted`, `files.FileFinalized` (write the held-back invocation), `agents.AgentDisabled`, `chat.ChannelReadOnlySet`, kernel `authorize.AccessChanged`.
- Implements source ports and cross-cutting ports: `messaging.ComponentReader`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `instance`, `chat`, `agents`, `messaging`, `billing`.

Reason: sign the token; operator, mention rule, project binding; policy and session; anchor and `trigger_state`; post costs.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.token.renew`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke`.

## Events and WebSocket ops

- Durable: `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked`.
- Ephemeral: `run.updated`, `command.autocomplete_requested`.
- WebSocket ops (client to server): `run.update`, `command.autocomplete_result`.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `runs.cancel_timeout` | periodic, `agent.cancel_grace` | `maintenance` |
| `runs.stall_check` | periodic, `run.stall_after` | `maintenance` |
| `runs.component_expiry` | periodic, `component.default_ttl` | `maintenance` |
| `runs.invocation_hold_timeout` | scheduled, `file.invocation_hold_max` or `voice.transcript_hold_max` | `default` |

## Notes for the build

- State transitions are compare-and-swap on `state` and `version` (03, `runs`).
- Live box patches are stored without `event_log` and go out as ephemeral `run.updated` to focused connections (04, 11).
- The invocation token is created anew at every delivery, after `agent.invoke` and I2 with current permissions; if the check fails, the run becomes `failed` (fail closed, 04, 15.3).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [09-agents.md](../../../../docs/spec/09-agents.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)
