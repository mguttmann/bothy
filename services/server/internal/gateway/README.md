# Module `gateway`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

WebSocket endpoint, gateway sessions, hub, focus, typing, forwarding of ephemeral events, agent connections.

## Code

- `internal/gateway` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/gateway`)

- Inbound: none for other modules.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `eventlog.EphemeralPublisher`.

## Dependencies

May use the ports of these modules: `chat`, `agents`, `runs`, `sync`, `stats`.

Reason: thread to channel for focus and typing; consumer cursor and presence; run patches and delivery preparation; tail, catch-up, snapshot; live frame for `stats.subscribe`.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

None. The module reads through the ports of other modules (03-data-model.md, principle 12).

## Actions (05-actions.md)

`gateway.connect`, `gateway.focus`, `typing.start`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: `typing.started`.
- WebSocket ops (client to server): `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop`.

## Notes for the build

- Per connection one read goroutine and one write goroutine with a bounded queue; an overflow closes with `4014` (04, 7.2).
- Gateway sessions live only in the process; `resume` on another node ends with `4006` (02, section 10).
- At shutdown, first save the agent cursors, then close all connections with `4013` (02, 15.3).
- The only module that may import `github.com/coder/websocket`.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [04-sync-protocol.md](../../../../docs/spec/04-sync-protocol.md), [06-api.md](../../../../docs/spec/06-api.md)
