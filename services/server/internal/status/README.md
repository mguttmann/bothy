# Module `status`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Status page, components, incidents, automatic checks, backup runs.

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
