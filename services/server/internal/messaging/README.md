# Module `messaging`

Status: sending built (M1-56): `createMessage` and `createThread`, the tables `messages` and `mentions`, the in-transaction event `MessagePosted` and the durable event `message.created`. Reading, editing, reactions, pins, templates and the export follow with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Messages, revisions, mentions, reactions, pins, templates, channel export.

## Code

- `internal/messaging` (`messaging.go`): the port. `Messages` with `Create` and `CreateThread`,
  the DTOs `Message`, `PrincipalRef`, `CreateMessageCmd`, `Created`, `CreateThreadCmd` and
  `ThreadCreated`, the in-transaction event `MessagePosted`, and `Register` (declares
  `message.created`). The port depends on no other module (02 section 7.2): `ThreadCreated` names
  the thread by id, and the handler reads it through `chat.Threads.View` (no `thread.read`, so
  an app with only `messages:write` gets the thread it created; a replay authorizes
  `message.create` on it first).
- `domain/`: the message row (`Message`), `OriginFor` (09 section 3.2), `CheckClientMsgID`,
  `CheckContent`, `CheckMentionCount`, the mention scanner `ScanMentions` with `CodeMask` (the code
  structure of CommonMark 0.31.2), `RequestHash` (04 section 5.1 point 3), `SystemLine` and
  `TruncateForEvent` (04 payload rule 2).
- `app/`: `Service` with `Create` and `CreateThread`, the outgoing port `Store` and
  `ErrClientMsgIDTaken`. `Config` binds `message.max_content_bytes`, `message.max_mentions` and
  `sync.event_payload_max_bytes` (`wire.MessagingConfig`).
- `adapters/postgres/`: `queries/messages.sql` and `queries/mentions.sql`, the sqlc output in
  `sqlcgen/` and `Repository` (implements `app.Store`); the mentions of a message are one
  `INSERT ... SELECT FROM unnest`.
- `adapters/httpapi/`: `Messages`, the handlers of `createMessage` and `createThread`; a
  repetition answers `200` with `Idempotent-Replay: true`.
- Migrations `00032_messaging_messages.sql` (`messages` and `mentions` as 03 section 4.6 with
  the deviations its header names, and `threads.parent_message_id -> messages(id)` deferred and
  `NOT VALID`) and `00033_threads_parent_fk_validate.sql` (validates it).
- The test world of both handlers is `services/server/test/messagingworld`.

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

## Sending (M1-56)

`createMessage` (04 sections 5.1 and 16.1):

1. `chat.Threads.Info`, then `message.create` on the thread; an unknown and an invisible thread
   give the same `404`, `read` only `403`, a read-only or archived channel
   `403 channel_read_only`, an app without `messages:write` `403 scope_missing`.
2. The body: `client_msg_id` a UUIDv7 (`not_uuid_v7`); `kind` absent or `text` (`voice` and
   `card` are `not_enabled`, so are non-empty `attachment_file_ids`, non-empty `components` and any
   `run_id`); `content_md` with a visible character (`empty`), without U+0000 (`invalid`) and at
   most `message.max_content_bytes` UTF-8 bytes (`413 payload_too_large`), stored byte for byte;
   `reply_to_message_id` a message of the same thread (`not_in_thread`, the same for an unknown
   id); at most `message.max_mentions` distinct handles (`too_many_mentions`).
3. One unit of work: the lookup of `(author, client_msg_id)` answers the original with
   `Idempotent-Replay` or `422 idempotency_key_reuse` by the canonical hash; otherwise the
   mentions are resolved, `chat.Threads.AppendToThread` hands out `thread_seq` and reopens a
   closed thread (then the system line takes that number and a second append gives the message
   its own), the rows are written, `MessagePosted` is published (system line first) and
   `message.created` appended. A concurrent identical request that committed first makes the
   insert fail on the unique index; the unit rolls back with its thread number and a new unit
   answers like a repetition.

Canonical hash (`client_payload_sha256`): SHA-256 over `bothy-msg-v1`, then context, kind,
content, reply target and the attachment ids joined by `,`, each as `uvarint(length)` and bytes.
The context is `thread:<id>` for `createMessage` and
`new-thread:<channel>|<parent>|<title>|<sorted tag ids>` for the first message of `createThread`.

Mentions (03 `messages` "Content format", 07 section 4.6): `ScanMentions` finds `@` and a handle
`[a-z0-9][a-z0-9._-]*` (case-insensitive) at the start of the text or after a character that is
not a letter, digit, `_`, `.`, `-`, `@` or `/`, cuts trailing `.`, `-` and `_`, and skips code
spans and fenced and indented code blocks. `CodeMask` follows the block structure of CommonMark
(block quotes, list items, lazy continuation, headings, thematic breaks) and the inline rules of
backslash escapes, autolinks and code spans; the tests hold every code-span and code-block
example of the specification. Raw HTML is not part of content format 1, so HTML tags are text
(docs/DECISIONS.md M1-56/2). Each distinct handle is looked up once among active people and
agents (`identity.Directory.ActiveByHandles`, one query). A person becomes a mention when the
author may read the person (`user.read`) or the person may read the thread (`thread.read` for the
person, or the person is an active owner, who reads every thread); an agent when the author may read it (`agent.read`). The checks run in filter mode, so a
token that stays text writes no audit entry. `@everyone` and `@here` are handles like any other.
Every resolved principal gets a `mentions` row, the author and people without access to the
channel included; `mention_ids` is sorted, and a `mention_ids` in the request must equal it as a
set (`mention_mismatch`).

`createThread` (02 section 4.6): `thread.create` on the channel, then in one unit of work the
repetition lookup of `first_message`, the parent message (a missing or deleted one keeps a zero
thread, which `chat` answers with `not_in_main_thread` like a message of another channel),
`chat.Threads.Create`, the forum rule (`required_for_forum_post`), `message.create` on the new
thread by its channel path and the flow of `createMessage` with `CreatedThread`. Without
`first_message` the `Idempotency-Key` stage makes the request idempotent.

Events: `message.created` per message with `message.read` as receive action, scope space,
channel and thread, resource the message with version 1, `client_msg_id` echoed (not for a system
line) and the payload `{message: Message}` with empty attachments, reactions and components,
`pinned: false` and no `my_reactions`. A payload that is not smaller than
`sync.event_payload_max_bytes` gets the longest valid UTF-8 prefix of `content_md` that fits,
`content_truncated: true` and the envelope flag `truncated`; REST answers are never cut.

In-transaction event `MessagePosted`: published for every new message, system lines included,
inside the writing transaction; a subscriber error rolls everything back. It carries ids, the
thread kind and sequence, author and origin, `AuthorDeviceID` (people only), the reply target,
the sorted mentions, `Silent`, `ClearDraft`, `CreatedThread` and `ReopenedThread`, never content.
No module subscribes in M1-56 (sync in M1-61, runs in M2, notify in M3).

System codes (`messages.system_code`, CHECK of 03): `thread.reopened`, written when a message
reopens a closed thread, with origin `system`, empty content, `silent`, the author of the
reopening message and the sequence right before it. The CHECK also allows `agent_session.resume_failed`,
which M2-12 writes (R-DM-42).

## Notes for the build

- Idempotency through the pair of author and `client_msg_id` (04, 5.1).
- Deletion empties the content and redacts the payload of the events in the same transaction (03, 6.5).
- A mention of a principal without read permission is stored but triggers nothing (03, `mentions`).

Rulings: docs/DECISIONS.md M1-56.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [06-api.md](../../../../docs/spec/06-api.md), [14-search.md](../../../../docs/spec/14-search.md)
