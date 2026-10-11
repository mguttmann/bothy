# Module `status`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Status page, components, incidents, automatic checks, backup runs.

## Code

- `internal/status` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/status`)

- Inbound: `Status`, `BackupRuns` (for `bothy-backupd`).
- Emits in-tx events: none.
- Subscribes to in-tx events: `agents.AgentDisabled`.
- Implements source ports and cross-cutting ports: none.

## Dependencies

May use the ports of these modules: `agents`.

Reason: agent presence as a status component.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`, `backup_runs`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`status.public.read`, `status.read`, `status.manage`.

## Events and WebSocket ops

- Durable: `status.updated`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `status.probe` | periodic | `maintenance` |

## Notes for the build

- Public status page only with `status.public_enabled` (05, `status.public.read`).
- Agent components follow the presence from `agents`.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [16-operations.md](../../../../docs/spec/16-operations.md), [17-observability.md](../../../../docs/spec/17-observability.md)
