# Module `agents`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Agents, backend policy and tool class (configured), reported state, consumer cursors with fencing, command register, agent sessions, dev environments (milestone).

## Code

- `internal/agents` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/agents`)

- Inbound: `Agents` (administration, policy, tool class), `Presence`, `Consumers` (cursor, ack, takeover with fencing), `Commands` (register), `Sessions`, `DevEnvironments` (milestone).
- Emits in-tx events: `AgentDisabled`.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: `authz.AgentPolicySource` (policy, tool class, readiness of the dev environment), kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `authz`, `chat`.

Reason: create the agent principal, the credential holder is a person; implements `AgentPolicySource`; thread for sessions, where commands apply.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`, `dev_environments`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.tool_class.set`, `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`, `dev_environment.read`, `dev_environment.manage`, `dev_environment.report`.

## Events and WebSocket ops

- Durable: `agent_session.changed`, `command.changed`, `agent.updated`.
- Ephemeral: `agent.presence_changed`.
- WebSocket ops (client to server): `ack`.

## Notes for the build

- The heartbeat state lives in memory and is stored at most once per `agent.heartbeat_persist_interval` (03, 4.4).
- `acked_seq` is stored at the latest after every `agent.ack_persist_interval` (04, 15.2); with several nodes, with fencing (02, section 16).
- Rebinding an agent session is a compare-and-swap on `agent_sessions.version`.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [09-agents.md](../../../../docs/spec/09-agents.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)
