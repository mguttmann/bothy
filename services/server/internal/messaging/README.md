# Modul `messaging`

Status: sending built (M1-56): `createMessage` and `createThread`, the tables `messages` and `mentions`, the in-transaction event `MessagePosted` and the durable event `message.created`. Reading built (M1-57): `listThreadMessages` and `getMessage`. Editing, deleting and revisions built (M1-58): `updateMessage`, `deleteMessage`, `listMessageRevisions`, the table `message_revisions`, the in-transaction event `MessageDeleted` and the durable events `message.updated` and `message.deleted`. Mentions and templates built (M1-60): `listMyMentions`, the port `Mentions` with `CountUnread`, the table `message_templates` and its six operations. Reactions and pins built (M1-59): `addReaction`, `removeReaction`, `listReactions`, `pinMessage`, `unpinMessage`, `listPins`, the tables `reactions` and `pins` and the durable events `reaction.added`, `reaction.removed`, `pin.added` and `pin.removed`. The port `ReplyTargets` and the first subscriber of `MessagePosted`, sync for `clear_draft`, came with M1-61. The ports of runs built (M2-05): `SetTriggerState`, `CreateRunAnchor`, `run_id` and the trigger state in every representation, and the order "append `message.created`, then publish `MessagePosted`". The export follows with its ticket.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Nachrichten, Revisionen, Erwähnungen, Reaktionen, Anheften, Vorlagen, Kanal-Export.

## Code

- `internal/messaging` (`messaging.go`, `mentions.go`, `templates.go`, `reactions.go`): the port. `Messages` with `Create`, `CreateThread`,
  `History`, `Get`, `Update`, `Delete` and `Revisions`, the DTOs `Message`, `PrincipalRef`,
  `CreateMessageCmd`, `Created`, `CreateThreadCmd`, `ThreadCreated`, `HistoryQuery` (with
  `HistoryMode`), `HistoryPage`, `UpdateCommand` and `RevisionView`, the in-transaction events
  `MessagePosted` and `MessageDeleted`, and `Register` (declares `message.created`,
  `message.updated` and `message.deleted`). `Update`, `Delete` and `Revisions` read the caller
  from the context (kernel `actor`). The port depends on no other module (02 section 7.2): `ThreadCreated` names
  the thread by id, and the handler reads it through `chat.Threads.View` (no `thread.read`, so
  an app with only `messages:write` gets the thread it created; a replay authorizes
  `message.create` on it first). `Mentions` (`ListMine`, `CountUnread`) and `Templates` (`List`,
  `ListMine`, `CreatePersonal`, `CreateForChannel`, `Update`, `Delete`) take the caller from the
  context (M1-60). `Reactions` (`Add`, `Remove`, `List` with `ReactionListQuery`, `ReactionPage`,
  `ReactionView`) and `Pins` (`Pin`, `Unpin`, `List` with `PinView`) take the caller from the
  context too (M1-59); `Message` carries `Reactions` (`ReactionCount`), `MyReactions` and `Pinned`.
- `domain/`: the message row (`Message`), `OriginFor` (09 section 3.2), `CheckClientMsgID`,
  `CheckContent`, `CheckMentionCount`, the mention scanner `ScanMentions` with `CodeMask` (the code
  structure of CommonMark 0.31.2), `RequestHash` (04 section 5.1 point 3), `SystemLine` and
  `TruncateForEvent` (04 payload rule 2); for reading `ValidateHistoryRequest`, `PickAround`,
  `PageFlags` and `CutForList`; for M1-58 `Revision`, the edit rules `Unchanged` and
  `StoresRevision`, the one tombstone function `Tombstone` and `RedactPayload` (04 section 2.2
  rule 5), the `eventlog.RedactFunc` of a deletion; the template row `Template` with `Apply` (no-op
  detection), `TemplateRules` (name and content), `LimitReached`, `ScopeLockKey` and the mention
  keyset `MentionKey` (M1-60); for M1-59 `ReactionGroup` with `OrderSummary` and `OwnInOrder`
  (the order of `reactions` and `my_reactions`), the keyset `ReactionKey`, `InvalidEmoji`,
  `ReactionLimitReached` and `PinLimitReached`.
- `app/`: `Service` with `Create` and `CreateThread` (`create.go`, `threads.go`), `History` and
  `Get`, `Update`, `Delete` and `Revisions` (`edit.go`), `Service.Mentions()`,
  `Service.Templates()`, `Service.Reactions()` and `Service.Pins()` (the other ports over the same
  service; `reactions.go`, `pins.go`), the batch loader `decorate` of the one representation
  (`view.go`), the outgoing ports `Store` (with `HistoryRead`, `HistoryRows`, `ReactionRead`,
  `ReactionRow` and `PinRow`), `Cursors`, `SymbolNormalizer` and `ErrorCounter`, and
  `ErrClientMsgIDTaken`.
  `Deps` holds the kernel ports `eventlog.Redactor` (implemented by sync) and `auditing.Recorder`
  besides those of M1-56. `Config` binds `message.max_content_bytes`, `message.max_mentions`,
  `sync.event_payload_max_bytes`, `message.list_content_max_bytes`, `template.name_max_chars`,
  `template.max_per_scope`, `api.page_size_default`, `api.page_size_max`,
  `reaction.emoji_max_bytes`, `reaction.max_distinct_per_message` and `pin.max_per_thread`
  (`wire.MessagingConfig`).
- `adapters/postgres/`: `queries/messages.sql`, `queries/mentions.sql`, `queries/templates.sql`,
  `queries/revisions.sql`, `queries/reactions.sql`, `queries/pins.sql` and `queries/history.sql` (`ListThreadNewest`, `ListThreadBefore`,
  `ListThreadAfter`, `ListThreadAround`, `GetVisibleMessage`), the sqlc output in `sqlcgen/` and
  `Repository` (implements `app.Store`); the mentions of a message are one `INSERT ... SELECT FROM
  unnest`. Every statement of M1-58 and M1-59 carries `channel_id = ANY(@visible)` or joins
  `messages` with it. `predicate_test.go` holds `TestQueriesCarryVisibleChannelPredicate` with its allowlist.
- `adapters/httpapi/`: `Messages`, the handlers of `createMessage`, `createThread`,
  `listThreadMessages`, `getMessage`, `listMyMentions` and the template operations (a repetition
  answers `200` with `Idempotent-Replay: true`) and of `updateMessage` (with `ETag`),
  `deleteMessage` and `listMessageRevisions` (`edit.go`) and of the reaction and pin operations
  (`reactions.go`); `Cursors`, the cursor codec of platform/httpserver.
- `adapters/unicode/`: `Normalizer`, the `SymbolNormalizer` of the reaction symbol rule with
  `github.com/rivo/uniseg` (UAX #29) and `golang.org/x/text/unicode/norm` (NFC), the only
  third-party libraries of the rule (M1-59).
- Migrations `00032_messaging_messages.sql` (`messages` and `mentions` as 03 section 4.6 with
  the deviations its header names, and `threads.parent_message_id -> messages(id)` deferred and
  `NOT VALID`), `00033_threads_parent_fk_validate.sql` (validates it),
  `00034_message_templates.sql` (`message_templates` as 03 section 4.6) and
  `00038_messaging_message_revisions.sql` (`message_revisions` as 03 states it, M1-58) and
  `00041_messaging_reactions_pins.sql` (`reactions` and `pins` as 03 states them, M1-59).
- The test world of both handlers is `services/server/test/messagingworld`.

## Metrics (M1-68)

`bothy.message.created` counts every inserted message, the reopen line and the run anchor included, with
`bothy.message.origin` and `bothy.message.kind` once its unit of work committed
(`txn.AfterCommit`), so a rolled-back message counts nothing. The use cases record through the
port `app.Metrics` (`app/metrics.go`); `adapters/otelmetrics` implements it with the declaration
`metrics.MessageCreated` of the metric registry, and `wire.MessagingWrites.Metrics` nil means that
adapter on the global meter provider.

## Port (Wurzelpaket `internal/messaging`)

- Eingehend: `Messages` (Anlegen, Thread mit erster Nachricht anlegen, Bearbeiten, Löschen, Verlauf, `SetTriggerState`, Anker), `Reactions`, `Pins`, `Templates`, `Mentions` (Liste, ungelesene je Thread), `Export`; deklariert `ComponentReader`.
- Gibt In-Tx-Ereignisse aus: `MessagePosted`, `MessageDeleted`.
- Abonniert In-Tx-Ereignisse: `chat.ThreadMoved`.
- Implementiert Quellen-Ports und Querschnitts-Ports: `files.AttachmentLocator`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`, `files`.

Grund: Erwähnungen auflösen; Thread-Sequenz und Wiederöffnen; Anhänge binden und lesen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`messages`, `message_revisions`, `mentions`, `reactions`, `pins`, `message_templates`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`thread.create`, `message.read`, `message.create`, `message.update`, `message.delete`, `message.revisions.read`, `reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`, `mention.list`, `template.read`, `template.manage`, `export.channel`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `message.created`, `message.updated`, `message.deleted`, `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

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
   its own), the rows are written, `message.created` is appended for the unit (system line
   first, then the message) and only then `MessagePosted` is published in the same order
   (ruling D-M2-05-3): a subscriber that appends a message to the same thread, the run anchor,
   must not overtake the message in the log (04 section 10). A concurrent identical request that committed first makes the
   insert fail on the unique index; the unit rolls back with its thread number and a new unit
   answers like a repetition.
4. After the unit of work the `201` body is the stored row read again through the one
   representation (`stored`), like the replay: a subscriber may have changed the row in the same
   unit (runs sets `trigger_state = received`, version 2; 04 section 16.5). The first message of
   `createThread` is read the same way.

Canonical hash (`client_payload_sha256`): SHA-256 over `bothy-msg-v1`, then context, kind,
content, reply target and the attachment ids joined by `,`, each as `uvarint(length)` and bytes.
The context is `thread:<id>` for `createMessage` and
`new-thread:<channel>|<parent>|<title>|<sorted tag ids>` for the first message of `createThread`.

Mentions (03 messages "Inhaltsformat", 07 section 4.6): `ScanMentions` finds `@` and a handle
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
`pinned: false` and no `my_reactions`; `run_id`, `trigger_state`, `trigger_state_at` and
`trigger_state_reason` are members only when set, never `null` (ADR 0053), in events and in REST
alike, and a tombstone keeps them because they are not content (03 `messages`). A payload that is not smaller than
`sync.event_payload_max_bytes` gets the longest valid UTF-8 prefix of `content_md` that fits,
`content_truncated: true` and the envelope flag `truncated`; REST answers are never cut.

In-transaction event `MessagePosted`: published for every new message, system lines included,
inside the writing transaction; a subscriber error rolls everything back. It carries ids, the
thread kind and sequence, author and origin, `AuthorDeviceID` (people only), the reply target,
the sorted mentions, `Silent`, `ClearDraft`, `CreatedThread` and `ReopenedThread`, never content.
`Kind` is `text`, `system` or `run` (the anchor). sync subscribes since M1-61 (it deletes the
author's draft for `ClearDraft`, so `draft.deleted` follows `message.created`); runs since M2-05
(the acceptance); notify in M3.

`ReplyTargets.InThread` (M1-61, `replies.go`) answers whether a message lies in a thread, deleted
ones included, with the rule of `createMessage`; an unknown id is false. It does not authorize:
sync checks the reply target of a thread draft with it after `draft.write`.

System codes (`messages.system_code`, CHECK of 03): `thread.reopened`, written when a message
reopens a closed thread, with origin `system`, empty content, `silent`, the author of the
reopening message and the sequence right before it. M2-12 adds `agent_session.resume_failed`.

## Runs (M2-05)

The two ports of runs run only inside the caller's unit of work (internal otherwise) and never
authorize: runs accepted the trigger or changed the run.

`SetTriggerState(ctx, SetTriggerStateCmd{MessageID, State, Reason})` (09 section 6): `State` is a
value of schema `TriggerState`, `Reason` is non-empty exactly for `failed` and `cancelled` (03
check). One `UPDATE` sets the three columns (`trigger_state_at` from the service clock) and raises
`version` by one, only for a message of origin `human` whose state or reason differ; without a row
the message is read: unknown or not of a person is internal, the same state and reason writes
nothing and appends nothing (D-M2-05-17). A change appends `message.updated` like an edit (the
representation and truncation of `message.created`, reactions and pin decorated, a tombstone in
its tombstone form), scope from `chat.Threads.Info`.

`CreateRunAnchor(ctx, RunAnchorCmd{ThreadID, RunID, AgentID})` (09 section 5.4): the next
`thread_seq` through `chat.Threads.AppendToThread` without reopen (as the principal of the unit,
which chat requires; the agent only outside a request), then the row of `domain.RunAnchor`: kind
`run`, origin `agent`, the agent as author, empty content, `silent`, `run_id`, a server
`client_msg_id` and the hash over the context `run-anchor:<run id>` and kind `run`, the instance
language, no mentions. `bothy.message.created` counts it, `message.created` follows without the
`client_msg_id` echo, then `MessagePosted` with `Kind run`, `Origin agent`, `AuthorKind agent` and
no device. It returns `AnchorRef{MessageID, ThreadSeq}`. The deferred foreign key `messages.run_id`
lets the run and its anchor be written in either order.

Lock order: neither port takes the thread lock for a message change. runs holds its `runs` row
and then updates the message (`SetTriggerState` last in the acceptance), so both orders of runs
take `runs` before `messages`; the anchor's `AppendToThread` locks the thread row like any new
message.

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
`403 subject_kind_not_allowed`, docs/DECISIONS.md M1-60/1), the channels of `message.read` from
`Authorizer.Scope`, then `mentions JOIN messages` with `channel_id = ANY(visible)` and
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
[14-suche.md](../../../../docs/spec/14-suche.md) 3.1, computed in the unit of work of the insert
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

## Thread moves (M1-62)

`Subscribers.OnThreadMoved` (`app/subscribers.go`) is the in-transaction subscriber of
`chat.ThreadMoved` (internal/wire registers it before the one of sync): it runs `MoveThreadMessages`
(`queries/threads_move.sql`, `UPDATE messages SET channel_id = $to WHERE thread_id = $thread`) in
the transaction of the move, deleted messages included. A copy only: no version, no event; clients
learn the new channel from `thread.moved`. The statement carries no visible-channel predicate by
design and stands in the allowlist of `adapters/postgres/predicate_test.go`: it writes only the
rows of the thread that chat locked and authorized with `thread.move` in the same unit of work
(07 section 7.4 rule 1: writes are outside the lint). A message sent after the move gets the
channel of the thread then (`Threads.AppendToThread`), the target. Tests:
`test/integration/move_test.go`.

## Editing, deleting and revisions (M1-58)

Every operation first gets the visible channels of the caller (`Authorizer.Scope` for
`message.read`) and reads the message only in them; a missing and an invisible message take the
same path through `authorize` and give the same `404`. The message resource names thread, channel
and author, so authz decides `write` and author (`message.update`) and `(write and author) or
manage` (`message.delete`).

`updateMessage` (precedence `428` > `404` > `403` > `410` > `412` > `413` > `400`): the HTTP chain
answers a missing `If-Match` with `428` before any lookup; then `authorize(message.update)` (a
read-only channel is `403 channel_read_only`, a system line `403`); in the unit of work the
message row lock, `410 message_deleted`, `412 version_conflict` with the current message in
`current` (an `If-Match` that is not a version never matches), the content rules of sending and
the mentions resolved again from the resulting content (a given `mention_ids` must equal them).
The same content and mention set, an empty body included, answers the current message and its
`ETag` without a write. Otherwise the previous content becomes revision `max + 1` (revision 1 is
the original; not for a run anchor), the row gets the content, `edited_at` and the next version,
removed mentions go, added ones are inserted, and `message.updated` carries the message like
`message.created` (cut above `sync.event_payload_max_bytes`, no `client_msg_id`). An edit never
publishes `MessagePosted` and never changes `trigger_state` (only `SetTriggerState` does). The `Idempotency-Key` stage answers a
repetition with the stored response.

`deleteMessage`: `authorize(message.delete)`; an already deleted message is `204` without a
write, an event or an audit entry. The unit of work locks the thread (`chat.Threads.
LockForMessageChange`) and then the message row (a message deleted meanwhile ends it without a
write), remembers the mentions, writes the tombstone (`content_md = ''`, `deleted_at`,
`deleted_by`, next version), removes the revisions, mentions, reactions and the pin (M1-59), counts the message down (`chat.Threads.DecrementMessageCount`), redacts the
payloads of every stored event with resource `message` and this id through
`eventlog.Redactor` with `domain.RedactPayload`, publishes `MessageDeleted` and appends
`message.deleted` (payload `MessageDeletedPayload`, after the redaction, so never redacted). A
deletion of another principal's message writes an `allow` audit entry with only `thread_id`,
`channel_id` and `author_principal_id`; denials take the deny path of authz.

`listMessageRevisions`: `authorize(message.revisions.read)`, the revisions ascending and
unpaginated, `{"items": []}` for a deleted message.

Lock order: the thread row (through `chat`) before the message row; several message rows in
ascending `id`; the counters last (03 section 6.3, enforced by the unit of work). A thread move
(M1-62) also locks the thread first, so a deletion and a move cannot deadlock; an edit locks only
the message row, which serialises edits, deletions and (from M1-59) reactions and pins of one
message, and revision numbers come from `max(revision) + 1` under it.

Redacted payloads (04 section 2.2 rule 5): `message.created` and `message.updated` become
`{"message": T}` with `T` the event form of the tombstone (every stored field, `content_md` empty,
no attachments, mentions, reactions or components, `pinned` false, `deleted_at` and the new
version, no `my_reactions`, no `content_truncated`); `reaction.added` and `reaction.removed` keep
`message_id`, `thread_id` and `principal_id` with `emoji: ""` and `count: 0`; `pin.added` and
`pin.removed` stay; any other type fails the deletion. The envelope columns never change.

## Reactions and pins (M1-59)

A reaction is one Unicode symbol of a person, agent or app on a message, never a status: no
operation touches `trigger_state` or `messages.version`. A pin is moderation with `manage`.

Writes check in this order: the symbol (`SymbolNormalizer`: valid UTF-8, NFC, exactly one
extended grapheme cluster of at most `reaction.emoji_max_bytes` bytes, no Cc, Cs, Zs, Zl, Zp or
Bidi control, not only Cf and Mn; `400 invalid_emoji` at `#path/emoji` before any lookup), then
the message in the visible channels with `authorize` (`reaction.add` on the message,
`reaction.remove` on the caller's reaction, `pin.create` and `pin.delete` on the message; a
missing and an invisible message give the same `404`), then in the unit of work the message row
lock and `deleted_at` (`410 message_deleted` for adding and pinning, `204` without a change for
removing and unpinning). The row lock serialises the reaction and pin changes of one message, so
the `count` of `reaction.added` and `reaction.removed` is exact; pinning also takes the thread
lock first (the lock order of deleting), so `pin.max_per_thread` is exact. One more distinct
symbol than `reaction.max_distinct_per_message` and one more pin are `400 validation_failed` with
`limit_reached`. Repetitions answer `204` without a write or an event.

The events carry resource kind `message`, scope space, channel and thread, the receive action
`message.read` (`reaction.*`) or `thread.read` (`pin.*`) and no resource version, because they
change none (04 section 9 item 3, `actions.yaml`; docs/DECISIONS.md M1-59/1); their payloads hold
absolute values. Every `Message` representation gets `reactions` (ordered by the first reaction
of each symbol, then bytewise by symbol), `pinned` and, only in REST, `my_reactions`, with one
statement per field set for a whole page; a tombstone has none.

`listReactions` reads the message like `getMessage`, validates the `emoji` filter
(`#query/emoji`) and pages by `(created_at, principal_id, emoji)` with a cursor bound to the
list, the message and the filter; the principals come from one directory read. `listPins` checks
`thread.read`, reads the pins of the live messages in the visible channels newest first and
checks each with `message.read` in filter mode (a mismatch is dropped, logged and counted as
`filter_mismatch`).

## Incoming webhooks (M2-27)

`CreateMessageCmd` has two fields for the posts of incoming webhooks (module apps); a thread with
its first message takes them from `CreateThreadCmd.FirstMessage`. Their zero values keep the
behaviour of M1-56.

- `SkipMentions`: the mention scanner does not run, `MentionIDs` is ignored, no `mentions` row is
  written and `MessagePosted.MentionedIDs` is empty (R-EX-09); the content stays unchanged.
- `HashContext`: when not empty it replaces the context part of the canonical hash
  (`thread:<id>` or `new-thread:...`). apps sets the context of the webhook, its body thread and
  its title, so a `client_msg_id` repeated through another webhook of the same app is
  `422 idempotency_key_reuse`, not a replay.

Tests: `app/incoming_test.go` (`TestCreate_SkipMentions`, `TestCreate_HashContextOverride`,
`TestCreateThread_HashContextOverride`).

## Hinweise für den Bau

- Idempotenz über das Paar aus Autor und `client_msg_id` (04, 5.1).
- Löschen leert den Inhalt und schwärzt die Nutzlast der Ereignisse in derselben Transaktion (03, 6.5).
- Eine Erwähnung eines Prinzipals ohne Leserecht wird gespeichert, löst aber nichts aus (03, `mentions`).

Rulings: docs/DECISIONS.md M1-56, M1-57, M1-58, M1-59 and M1-60.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [06-api.md](../../../../docs/spec/06-api.md), [14-suche.md](../../../../docs/spec/14-suche.md)
