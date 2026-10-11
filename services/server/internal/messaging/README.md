# Module `messaging`

Status: planned (M0). This directory contains no code yet. The normative source is
[docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Messages, revisions, mentions, reactions, pins, templates, channel export.

## Port (root package `internal/messaging`)

- Inbound: `Messages` (create, create a thread with its first message, edit, delete, history, `SetTriggerState`, anchor), `Reactions`, `Pins`, `Templates`, `Mentions` (list, unread per thread), `Export`; declares `ComponentReader`.
- Emits in-tx events: `MessagePosted`, `MessageDeleted`.
- Subscribes to in-tx events: `chat.ThreadMoved`.
- Implements source ports and cross-cutting ports: `files.AttachmentLocator`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `chat`, `files`.

Reason: resolve mentions; thread sequence and reopening; bind and read attachments.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`messages`, `message_revisions`, `mentions`, `reactions`, `pins`, `message_templates`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`thread.create`, `message.read`, `message.create`, `message.update`, `message.delete`, `message.revisions.read`, `reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`, `mention.list`, `template.read`, `template.manage`, `export.channel`.

## Events and WebSocket ops

- Durable: `message.created`, `message.updated`, `message.deleted`, `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Notes for the build

- Idempotency through the pair of author and `client_msg_id` (04, 5.1).
- Deletion empties the content and redacts the payload of the events in the same transaction (03, 6.5).
- A mention of a principal without read permission is stored but triggers nothing (03, `mentions`).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [06-api.md](../../../../docs/spec/06-api.md), [14-search.md](../../../../docs/spec/14-search.md)
