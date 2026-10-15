# Module `messaging`

Status: sending built (M1-56): `createMessage` and `createThread`, the tables `messages` and `mentions`, the in-transaction event `MessagePosted` and the durable event `message.created`. Reading built (M1-57): `listThreadMessages` and `getMessage`. Mentions and templates built (M1-60): `listMyMentions`, the port `Mentions` with `CountUnread`, the table `message_templates` and its six operations. Editing, reactions, pins and the export follow with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Messages, revisions, mentions, reactions, pins, templates, channel export.

## Code

- `internal/messaging` (`messaging.go`, `mentions.go`, `templates.go`): the port. `Messages` with `Create`, `CreateThread`,
  `History` and `Get`, the DTOs `Message`, `PrincipalRef`, `CreateMessageCmd`, `Created`,
  `CreateThreadCmd`, `ThreadCreated`, `HistoryQuery` (with `HistoryMode`) and `HistoryPage`, the in-transaction event `MessagePosted`, and `Register` (declares
  `message.created`). The port depends on no other module (02 section 7.2): `ThreadCreated` names
  the thread by id, and the handler reads it through `chat.Threads.View` (no `thread.read`, so
  an app with only `messages:write` gets the thread it created; a replay authorizes
  `message.create` on it first). `Mentions` (`ListMine`, `CountUnread`) and `Templates` (`List`,
  `ListMine`, `CreatePersonal`, `CreateForChannel`, `Update`, `Delete`) take the caller from the
  context (M1-60).
- `domain/`: the message row (`Message`), `OriginFor` (09 section 3.2), `CheckClientMsgID`,
  `CheckContent`, `CheckMentionCount`, the mention scanner `ScanMentions` with `CodeMask` (the code
  structure of CommonMark 0.31.2), `RequestHash` (04 section 5.1 point 3), `SystemLine` and
  `TruncateForEvent` (04 payload rule 2); for reading `ValidateHistoryRequest`, `PickAround`,
  `PageFlags` and `CutForList`; the template row `Template` with `Apply` (no-op
  detection), `TemplateRules` (name and content), `LimitReached`, `ScopeLockKey` and the mention
  keyset `MentionKey` (M1-60).
- `app/`: `Service` with `Create`, `CreateThread`, `History` and `Get`, `Service.Mentions()` and
  `Service.Templates()` (the two other ports over the same service), the outgoing ports `Store`
  (with `HistoryRead` and `HistoryRows`), `Cursors` and `ErrorCounter`, and `ErrClientMsgIDTaken`.
  `Config` binds `message.max_content_bytes`, `message.max_mentions`, `sync.event_payload_max_bytes`,
  `message.list_content_max_bytes`, `template.name_max_chars`, `template.max_per_scope`,
  `api.page_size_default` and `api.page_size_max` (`wire.MessagingConfig`).
- `adapters/postgres/`: `queries/messages.sql`, `queries/mentions.sql`, `queries/templates.sql` and
  `queries/history.sql` (`ListThreadNewest`, `ListThreadBefore`, `ListThreadAfter`,
  `ListThreadAround`, `GetVisibleMessage`), the sqlc output in `sqlcgen/` and `Repository`
  (implements `app.Store`); the mentions of a message are one `INSERT ... SELECT FROM unnest`.
  `predicate_test.go` holds `TestQueriesCarryVisibleChannelPredicate` with its allowlist.
- `adapters/httpapi/`: `Messages`, the handlers of `createMessage`, `createThread`,
  `listThreadMessages`, `getMessage`, `listMyMentions` and the template operations (a repetition
  answers `200` with `Idempotent-Replay: true`); `Cursors`, the cursor codec of platform/httpserver.
- Migrations `00032_messaging_messages.sql` (`messages` and `mentions` as 03 section 4.6 with
  the deviations its header names, and `threads.parent_message_id -> messages(id)` deferred and
  `NOT VALID`), `00033_threads_parent_fk_validate.sql` (validates it) and
  `00034_message_templates.sql` (`message_templates` as 03 section 4.6).
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

## Reading (M1-57)

`listThreadMessages` (`GET /api/v1/threads/{thread_id}/messages`, `thread.read`, `rl.read`; 06
section 6 item 5):

1. The anchors without any lookup: more than one of `before_seq`, `after_seq`, `around_seq` is
   `400 validation_failed` with one `mutually_exclusive` entry per given parameter
   (`#query/<name>`); `after_seq` below 0, `before_seq` or `around_seq` below 1, or any anchor at
   or above 2^53 is `out_of_range`.
2. `chat.Threads.Info` and `thread.read` on the thread; an unknown and an invisible thread take
   the same path and give the same `404`.
3. `Authorizer.Scope(message.read)` gives the visible channels; the one statement of the mode
   carries `m.thread_id = @thread_id AND m.channel_id = ANY(@visible_channel_ids)`.
4. Window with `L = min(limit or api.page_size_default, api.page_size_max)`: the newest page is the
   `L` highest `thread_seq`, `before_seq = X` the `L` highest below `X`, `after_seq = X` the `L`
   lowest above `X`, `around_seq = X` at most `(L - 1) / 2` below `X` and the rest from `X` on,
   where a short side gives its share to the other. Items are ascending and contiguous;
   `has_more_before` and `has_more_after` come from the same statement (rows `L + 1` per side and
   `EXISTS` for the opposite side). With no items, `before_seq = X` reports later messages
   (`thread_seq >= X`) and `after_seq = X` earlier ones (`thread_seq <= X`).
5. Every item passes `message.read` in filter mode; a refused one is dropped, logged with the
   action only and counted in `bothy.authz.errors` as `filter_mismatch` (07 section 7.4 rule 3).
6. Each item in the representation of M1-56, `content_md` cut above
   `message.list_content_max_bytes` at a UTF-8 boundary with `content_truncated: true`.

`getMessage` (`GET /api/v1/messages/{message_id}`, `message.read`, `rl.read`): the row read with the
visible channels of `message.read`, then `message.read` on it; a foreign and an unknown id give the
same `404`. When the visible read misses, an existing message is still decided by `message.read`,
so an app with a grant on the channel but without `messages:read` gets `403 scope_missing`. `200` with the full content, never `content_truncated`, and `ETag: "<version>"`; a
tombstone is `200` too. It is the reload path for content cut in lists and events.

Tombstones: a deleted message keeps its row and `thread_seq`; its form has `content_md = ""`, empty
`attachments`, `mention_ids`, `reactions` and `components`, `pinned: false` and `deleted_at`, all
other fields as stored. Logs and metrics never carry content, query strings or names.

## Mentions and templates (M1-60)

`listMyMentions` (`GET /me/mentions`): `mention.list` on the caller as resource `user` (an agent
`403 forbidden` (reason `subject_kind_not_allowed`), docs/DECISIONS.md M1-60/1), the channels of
`message.read` from `Authorizer.Scope`, then `mentions JOIN messages` with `channel_id = ANY(visible)` and
`deleted_at IS NULL`, keyset `(mentions.created_at DESC, message_id DESC)`, page size from
`api.page_size_default` and `api.page_size_max`. Every item passes `authorize(message.read)` in
filter mode; an item it refuses is dropped, logged with only the action and the reason and
counted in `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 section 7.4 rule 3). The
cursor of platform/httpserver is bound to the list name `mentions` and the caller; any other is
`400 cursor_invalid`. A mention in a channel the caller cannot read stays stored and is never
listed (AZ-M48).

`Mentions.CountUnread` has no route. It counts, in one statement, the mentions of one principal
per thread whose message is not deleted, lies in one of the given visible channels and has a
`thread_seq` above the read sequence of its thread (0 when the thread is not in the map; the
map goes in as two parallel arrays). Threads without unread mentions are absent; an empty
visible set returns nothing. `threads.unread_floor_seq` is not needed, because imports write no
mentions (M1-60/2). Consumer: `sync`, which computes `ReadState.mention_count` in read-state
responses and events (M1-61) and the snapshot (M1-63) with it, with the scope it built for the
person; no counter is stored (R-DM-32). The badge of push (M3) reads the same counts.

Templates (`message_templates`): a personal template has `owner_user_id` and no channel and is
checked against the resource `user` of its owner (`self`; only people, M1-60/1); a channel
template has `channel_id` and is checked against the channel (`template.read` with `read`,
`template.manage` with `manage`, also in a read-only channel, 05 rule 1.8).

- `listMyTemplates` and `listTemplates` are unpaginated and ordered by `lower(name)`, then `id`;
  `listTemplates` returns the caller's personal templates and those of the channel (an
  invisible or unknown channel `404`).
- Create (`Idempotency-Key`): `name` required after trimming, at most
  `template.name_max_chars` code points, no control or bidi control characters
  (`invalid_characters`); `content_md` required after trimming, at most
  `message.max_content_bytes` bytes and without U+0000; both stored as given. Under
  `pg_advisory_xact_lock` of the scope the count of templates without `deleted_at` must stay
  below `template.max_per_scope` (otherwise `400`, empty pointer, `limit_reached`).
- `updateTemplate` (`If-Match` required): `428` before any lookup, then one `404` path for a
  missing, deleted, foreign personal or invisible template (the row is read with
  `owner_user_id = caller OR channel_id = ANY(Scope(channel.read))` and locked), `403` without
  the right, `412 version_conflict` with `current`, `400` for the given fields; a change without
  effect returns `200` without a write; `ETag` is the version.
- `deleteTemplate`: like the update, but a deleted template is `204` without a write; a never
  visible one `404`.
- Templates append no event (05 section 6); clients reload them by REST (M1-60/4).

## Search language (M3-13)

Every insert of `messages` names `search_language`, the text search configuration of
[14-search.md](../../../../docs/spec/14-search.md) 3.1, computed in the unit of work of the insert
through `identity.Directory`: origin `human` takes `SearchLanguage(author)` (the mapping of
`users.locale` through `search.locale_map`), origin `agent` and `app` take
`InstanceSearchLanguage` without reading the principal, and the reopen line (origin `system`)
takes the instance language whoever caused it. The value goes to SQL as text and is cast there;
`domain.Message.SearchLanguage` is write-only. The generated `search_vector` keeps itself
current: an edit recomputes it with the stored language, a deleted message (empty content) has an
empty vector. The queries list their columns, so no read carries `search_language` or
`search_vector` (docs/DECISIONS.md D-M3-13-10). `TestMessageInsertsSetSearchLanguage`
(`adapters/postgres`) fails when an `INSERT INTO messages` of `queries/*.sql` does not name
`search_language` (later writers: M2 agent messages, M8-03 import, V3-01 DMs).

## Notes for the build

- Idempotency through the pair of author and `client_msg_id` (04, 5.1).
- Deletion empties the content and redacts the payload of the events in the same transaction (03, 6.5).
- A mention of a principal without read permission is stored but triggers nothing (03, `mentions`).

Rulings: docs/DECISIONS.md M1-56, M1-57 and M1-60.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [06-api.md](../../../../docs/spec/06-api.md), [14-search.md](../../../../docs/spec/14-search.md)
