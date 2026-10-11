# Module `sync`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Event log (append, tail, catch-up, redaction, partitions), snapshot, idempotency keys, read state, drafts, personal settings, cursors of the worker consumers.

## Port (root package `internal/sync`)

- Inbound: `Tail`, `CatchUp`, `Snapshot`, `Consumers` (worker cursors), `ReadStates`, `Drafts`, `Settings`.
- Emits in-tx events: none.
- Subscribes to in-tx events: `messaging.MessagePosted` (clear the draft), `chat.ThreadMoved` (copies of `channel_id`).
- Implements source ports and cross-cutting ports: kernel `eventlog.Appender`, kernel `eventlog.Redactor`, kernel `idempotency.Store`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `instance`, `identity`, `chat`, `agents`, `messaging`, `runs`.

Reason: counter `event_seq`; principal of the snapshot (handle, display name); snapshot from structure, presence, active runs; subscription to messages.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`event_log`, `event_consumer_cursors`, `idempotency_keys`, `read_states`, `drafts`, `user_settings`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up`.

## Events and WebSocket ops

- Durable: `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `sync.partition_maintenance` | periodic, create partitions and detach them after `sync.event_retention` | `maintenance` |
| `sync.idempotency_cleanup` | periodic | `maintenance` |

## Notes for the build

- Implements `kernel/eventlog.Appender`: numbers from `event_seq` in phase 3.2 of the Unit of Work, after that `pg_notify` only with the number (02, 8.1).
- The log tail uses its own, non-pooled connection for `LISTEN` (02, 11.2).
- The snapshot contains no thread contents (04, 6.3).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [04-sync-protocol.md](../../../../docs/spec/04-sync-protocol.md), [15-client-architecture.md](../../../../docs/spec/15-client-architecture.md)
