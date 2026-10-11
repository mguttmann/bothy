# Module `chat`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Spaces, sidebar state, channels with behavior, skills, project binding, DM participants, forum tags, threads with thread sequence.

## Port (root package `internal/chat`)

- Inbound: `Structure` (spaces, channels, read-only), `Threads` (create for `messaging`, `AppendToThread`, message count on deletion, close, open, move, info), `AgentConfig`.
- Emits in-tx events: `ThreadMoved`, `ChannelArchived`, `ChannelReadOnlySet`, `SpaceArchived`.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: `authz.StructureSource`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `authz`.

Reason: the operator is an active agent principal, DM participants are people; implements `StructureSource`.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`spaces`, `user_space_states`, `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`, `threads`, `thread_tags`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.read_only.set`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read`.

## Events and WebSocket ops

- Durable: `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved`, `thread.moved_out`, `space.created`, `space.updated`, `space.archived`, `channel.created`, `channel.updated`, `channel.settings_changed`, `channel.archived`, `sidebar.updated`, `agent.config_changed`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Notes for the build

- `Threads.AppendToThread` locks the thread row and assigns the thread sequence (03, 6.2); `messaging` calls it in its transaction.
- The channel prompt, skills and project binding never appear in events; `channel.settings_changed` carries only `settings_version` (04, 2.2).
- A thread can be moved only when the effective operator stays the same (03, `threads`).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [01-product.md](../../../../docs/spec/01-product.md), [07-permissions.md](../../../../docs/spec/07-permissions.md), [09-agents.md](../../../../docs/spec/09-agents.md)
