# Module `terminal`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Hosts and pairing, declaration of the LLM login (I6), terminal sessions, terminal session tokens, control connections of the host services, relay of the streams.

## Code

- `internal/terminal` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/terminal`)

- Inbound: `Hosts` (administration, pairing, declaration), `Sessions` (open, attach, close, list), `Relay` (stream and host connections, only in the adapter `wsrelay`).
- Emits in-tx events: none.
- Subscribes to in-tx events: kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (end sessions).
- Implements source ports and cross-cutting ports: `authz.HostSource` (the host's state, assignee, LLM declaration, server host).

## Dependencies

May use the ports of these modules: `instance`, `identity`, `authz`, `agents`.

Reason: signer for terminal session tokens (purpose `terminal`); people, devices, owners, consumption of the step-up; grants on hosts and I6; readiness of the dev environment (milestone).

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`hosts`, `host_session_users`, `terminal_sessions`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate`.

## Events and WebSocket ops

- Durable: `host.updated`, `terminal_session.changed`.
- Ephemeral: `host.presence_changed`.
- WebSocket ops (client to server): none on the shared WebSocket; separate connections with their own ops (05, section 7).

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `terminal.sweep` | periodic | `maintenance` |

## Notes for the build

- Built from M2 on (PLAN, Block terminal); until then the actions in the catalogue are deferred (PLAN P-24).
- The server is the relay and the authorization point and reads no terminal content (Noise stream end to end, 24-terminal.md).
- Invariant I6 is checked at every grant on a host and at every `terminal.open` (10-backend-policy.md, 24-terminal.md).
- No terminal content in telemetry, audit, search or push.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [24-terminal.md](../../../../docs/spec/24-terminal.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)
