# Module `messaging`

Status: sending built (M1-56): `createMessage` and `createThread`, the tables `messages` and `mentions`, the in-transaction event `MessagePosted` and the durable event `message.created`. Reading built (M1-57): `listThreadMessages` and `getMessage`. Editing, deleting and revisions built (M1-58): `updateMessage`, `deleteMessage`, `listMessageRevisions`, the table `message_revisions`, the in-transaction event `MessageDeleted` and the durable events `message.updated` and `message.deleted`. Mentions and templates built (M1-60): `listMyMentions`, the port `Mentions` with `CountUnread`, the table `message_templates` and its six operations. Reactions and pins built (M1-59): `addReaction`, `removeReaction`, `listReactions`, `pinMessage`, `unpinMessage`, `listPins`, the tables `reactions` and `pins` and the durable events `reaction.added`, `reaction.removed`, `pin.added` and `pin.removed`. The port `ReplyTargets` and the first subscriber of `MessagePosted`, sync for `clear_draft`, came with M1-61. The ports of runs built (M2-05): `SetTriggerState`, `CreateRunAnchor`, `run_id` and the trigger state in every representation, and the order "append `message.created`, then publish `MessagePosted`". Channel export built (M3-11): `exportChannel`, the port `Export` and the NDJSON stream. Agent answers built (M2-19): `run_id` of agents, `MessagePosted.RunID`. The reads of the consumer push of notify built (M3-19): `LastAnchorBefore`, `AnchorOf`, `LastAgentMessageBefore` and `Newest`. Cards built (M2-15): `kind = card` with components for agents, the source ports `ComponentReader` and `ComponentWriter` that runs implements, the port methods `LockCard`, `ComponentsChanged` and `PrincipalOf`, the request hash `bothy-msg-v2` and the components in every representation of a message. Attachments built (M3-07): `attachment_file_ids` in `createMessage` and `createThread`, the column `messages.has_attachments`, the attachments in every representation and their deletion with the message, all through the port `files.Attachments` (section "Attachments (M3-07)"). The read operations of the search coordinator built (M3-14): `SearchReads` with `SearchMessages` and `HitMessages` (section "Read operations for the search coordinator (M3-14)"). App cards built (M2-25): apps send `kind = card` with components, and the port method `Authors`.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Messages, revisions, mentions, reactions, pins, templates, channel export.

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
  `components.go` (M2-15): the source ports `ComponentWriter` and `ComponentReader` with
  `ComponentInput`, `CardInput`, `CheckedCard`, `Component` and `CardFacts`; `Message.Components`
  and `CreateMessageCmd.Components` and `RunID`; `Messages.LockCard`, `ComponentsChanged`,
  `PrincipalOf` and (M2-25) `Authors`.
- `domain/`: the message row (`Message`, with `HasAttachments` since M3-07), `OriginFor` (09
  section 3.2), `CheckClientMsgID`, `CheckContent`, `CheckMentionCount`, the body rules of M3-07
  (`CheckAttachments`, `CheckNewContent`, the pointers `AttachmentsPointer` and
  `FirstMessagePointer`), the mention scanner `ScanMentions` with `CodeMask` (the code
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
  `Get`, `Update`, `Delete` and `Revisions` (`edit.go`), the card rules, `LockCard`,
  `ComponentsChanged` and the component part of the representation (`cards.go`, M2-15), `Service.Mentions()`,
  `Service.Templates()`, `Service.Reactions()` and `Service.Pins()` (the other ports over the same
  service; `reactions.go`, `pins.go`), the batch loader `decorate` of the one representation
  (`view.go`), the outgoing ports `Store` (with `HistoryRead`, `HistoryRows`, `ReactionRead`,
  `ReactionRow` and `PinRow`), `Cursors`, `SymbolNormalizer` and `ErrorCounter`, and
  `ErrClientMsgIDTaken`.
  `Deps` holds the kernel ports `eventlog.Redactor` (implemented by sync) and `auditing.Recorder`
  besides those of M1-56, and the port `files.Attachments` (`Deps.Files`, M3-07; nil keeps
  `attachment_file_ids` refused as `not_enabled` for test worlds without files). `Config` binds
  `message.max_content_bytes`, `message.max_mentions`, `sync.event_payload_max_bytes`,
  `message.list_content_max_bytes`, `template.name_max_chars`, `template.max_per_scope`,
  `api.page_size_default`, `api.page_size_max`, `reaction.emoji_max_bytes`,
  `reaction.max_distinct_per_message`, `pin.max_per_thread` and `message.max_attachments`
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
  `00040_messaging_message_revisions.sql` (`message_revisions` as 03 states it, M1-58) and
  `00041_messaging_reactions_pins.sql` (`reactions` and `pins` as 03 states them, M1-59), and
  `00067_messaging_has_attachments.sql` with `00068_messaging_has_attachments_validate.sql`
  (`messages.has_attachments` and the CHECK `messages_has_attachments_deleted_check`, added
  `NOT VALID` and validated separately, M3-07), which close the `has_attachments` deviation the
  header of 00032 names.
- The test world of both handlers is `services/server/test/messagingworld`.

## Metrics (M1-68)

`bothy.message.created` counts every inserted message, the reopen line and the run anchor included, with
`bothy.message.origin` and `bothy.message.kind` once its unit of work committed
(`txn.AfterCommit`), so a rolled-back message counts nothing. The use cases record through the
port `app.Metrics` (`app/metrics.go`); `adapters/otelmetrics` implements it with the declaration
`metrics.MessageCreated` of the metric registry, and `wire.MessagingWrites.Metrics` nil means that
adapter on the global meter provider.

## Preview read (M3-22)

- Port: `Messages.ForPreview(ctx, messageID)` returns `PreviewMessage` (thread, channel, author, kind, `content_md`, `content_format`, `has_attachments`, the deletion and the principals of the `mentions` rows, sorted); `found` is false for an unknown message, and a tombstone comes back with `Deleted` and without content, attachments and mentions. It never authorizes: the preview fetch of `notify` checks `message.read` and `push.deliver` itself, the job `notify.gateway_send` checks `push.deliver`. It reads in the ambient transaction of ctx when there is one.
- Code: `messaging.go`, `app/pushreads.go`; test `app/pushreads_test.go`.

## Port (root package `internal/messaging`)

- Inbound: `ImportMessages` (M8-11), `Messages` (create, create a thread with its first message, edit, delete, history, `SetTriggerState`, anchor, `AppendSystemLine` since M2-12, `LastAnchorBefore`, `AnchorOf`, `LastAgentMessageBefore`, `Newest`, `ForPreview`), `Reactions`, `Pins`, `Templates`, `Mentions` (list, unread per thread), `Export`, `SearchReads` (`SearchMessages`, `HitMessages` for the search coordinator, M3-14); declares `ComponentReader` and `ComponentWriter` (M2-15, implemented by runs in `internal/runs/adapters/messagingsource`). `Messages` offers runs `LockCard`, `ComponentsChanged` and `PrincipalOf` (M2-15) and `Authors` (M2-25).
- Emits in-tx events: `MessagePosted`, `MessageDeleted`.
- Subscribes to in-tx events: `chat.ThreadMoved`.
- Implements source ports and cross-cutting ports: kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `chat`, `files`.

Reason: resolve mentions; thread sequence and reopening; bind and read attachments (`files.Attachments`; delete them with the message, M3-07).

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
   `403 channel_read_only`, an app without `messages:write` `403 scope_missing`. With
   attachments the resource carries `HasAttachments`, so authz also needs `upload` (`403
   forbidden`, reason `capability_missing`; M3-07).
2. The body: `client_msg_id` a UUIDv7 (`not_uuid_v7`); `kind` absent, `text` or (M2-15) `card`
   with the card rules of "Cards and components" below (`voice` is `not_enabled`, so is `run_id`
   with any kind but `card`); at most `message.max_attachments` ids in `attachment_file_ids`
   (`limit_reached` on the list, M3-07); `content_md` with a visible character (`empty`; a message
   with at least one attachment may have an empty or white space only content), without U+0000 (`invalid`) and at
   most `message.max_content_bytes` UTF-8 bytes (`413 payload_too_large`), stored byte for byte;
   `reply_to_message_id` a message of the same thread (`not_in_thread`, the same for an unknown
   id); at most `message.max_mentions` distinct handles (`too_many_mentions`).
3. One unit of work: the lookup of `(author, client_msg_id)` answers the original with
   `Idempotent-Replay` or `422 idempotency_key_reuse` by the canonical hash; otherwise the
   mentions are resolved, `chat.Threads.AppendToThread` hands out `thread_seq` and reopens a
   closed thread (then the system line takes that number and a second append gives the message
   its own), the rows are written (with `has_attachments`), the attachments are bound through
   `files.Attachments.Attach` (M3-07), `message.created` is appended for the unit (system line
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
A request with `components` or `run_id` uses `bothy-msg-v2` (M2-15, D-M2-15-7): the fields of v1
in v1 order, then `run_id` (empty when absent) and `CheckedCard.Canonical`, each as
`uvarint(length)` and bytes; every other request keeps `bothy-msg-v1` byte for byte, so a message
stored before M2-15 still replays.

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
line) and the payload `{message: Message}` with its attachments (M3-07), empty reactions, the components of
a card (M2-15; `[]` for every other message), `pinned: false` and no `my_reactions`; `run_id`, `trigger_state`, `trigger_state_at` and
`trigger_state_reason` are members only when set, never `null` (ADR 0053), in events and in REST
alike, and a tombstone keeps them because they are not content (03 `messages`). A payload that is not smaller than
`sync.event_payload_max_bytes` gets the longest valid UTF-8 prefix of `content_md` that fits,
`content_truncated: true` and the envelope flag `truncated`; REST answers are never cut.

In-transaction event `MessagePosted`: published for every new message, system lines included,
inside the writing transaction; a subscriber error rolls everything back. It carries ids, the
thread kind and sequence, author and origin, `AuthorDeviceID` (people only), the reply target,
the sorted mentions, `Silent`, `ClearDraft`, `CreatedThread`, `ReopenedThread` and
`HasAttachments` (M3-07: runs holds the agent invocation while an attachment is not final),
never content.
`Kind` is `text`, `system` or `run` (the anchor). sync subscribes since M1-61 (it deletes the
author's draft for `ClearDraft`, so `draft.deleted` follows `message.created`); runs since M2-05
(the acceptance); notify in M3.

`ReplyTargets.InThread` (M1-61, `replies.go`) answers whether a message lies in a thread, deleted
ones included, with the rule of `createMessage`; an unknown id is false. It does not authorize:
sync checks the reply target of a thread draft with it after `draft.write`.

System codes (`messages.system_code`, CHECK of 03): `thread.reopened`, written when a message
reopens a closed thread, with origin `system`, empty content, `silent`, the author of the
reopening message and the sequence right before it; `agent_session.resume_failed` (M2-12,
R-DM-42), the fail-open line of an agent session, written through `AppendSystemLine`.
`domain.IsSystemCode` holds the codes of the CHECK.

## Runs (M2-05)

`PrincipalRefs` (M2-16) renders principals of the directory as `PrincipalRef` without authorizing,
for the invoker of a command (`invoked_by`, `triggered_by`), because `runs` may not use `identity`.

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

`TriggerMessageOf(ctx, messageID)` (M2-08): the trigger facts of one message for the delivery
check of runs, without content and without an authorization: place, author as `PrincipalRef`,
origin, kind, the stored mentions and the trigger state, and since M2-11 its `thread_seq` and
whether it is a tombstone (`Deleted`), which the end of a run reads for its follow-ups.
`not_found` for an unknown message.

The reads of the consumer push of notify (M3-19 design notes 5 and 9 d; 13 section 2.1), in
`app/pushreads.go` over the store port `app.PushReadStore` (part of `Store`) and the queries of
`queries/pushreads.sql`. They return positions only (`AnchorRef{MessageID, RunID, ThreadSeq}` or
`MessageRef{ID, ThreadSeq}`), never content, and `found = false` instead of an error when no row
matches. None authorizes and none carries the visible channel predicate: the consumer has no
principal and reads like the log tail, and notify checks every hint with `push.deliver` (07
section 7.4 rule 1); `predicate_test.go` lists them with this reason.

- `LastAnchorBefore(ctx, threadID, beforeThreadSeq)`: the newest run anchor (`kind = 'run'`,
  the only rows with `run_id`) of the thread with `thread_seq` below the bound (query
  `LastAnchorBefore`, backwards over `messages_thread_seq_key`).
- `AnchorOf(ctx, runID)`: the anchor of the run (query `AnchorOfRun`, over `messages_run_id_key`).
- `LastAgentMessageBefore(ctx, threadID, agentID, afterThreadSeq, beforeThreadSeq)`: the newest
  message of the agent strictly between the bounds with `origin = 'agent'`, `kind IN ('text',
  'voice', 'card')` and `deleted_at IS NULL` (query `LastAgentMessageBefore`).
- `Newest(ctx, threadID)`: the newest message of the thread that is not deleted and whose kind is
  neither `run` nor `system`, the target of a bundled hint (query `NewestMessage`).

Lock order: neither port takes the thread lock for a message change. runs holds its `runs` row
and then updates the message (`SetTriggerState` last in the acceptance), so both orders of runs
take `runs` before `messages`; the anchor's `AppendToThread` locks the thread row like any new
message.

## Cards and components (M2-15)

`runs` owns `message_components`; `messaging` owns `messages` and may not import `runs`. So
`messaging` declares two source ports in `components.go` and runs implements both in
`internal/runs/adapters/messagingsource`; `internal/wire` passes the `Source` as
`MessagingWrites.Components` (nil: every message shows `components: []` and a card is
`not_enabled`).

- `ComponentWriter.Check(ctx, CardInput)` validates the components of a request without database
  access (`component.v1`, the server fields) and returns `CheckedCard` with `Canonical`, the input
  of the request hash, and the components as they will be stored; `ComponentWriter.Attach(ctx,
  messageID, CheckedCard)` stores them in the ambient unit of work and returns them in position
  order. Errors are runs' (`400 validation_failed` per finding, `400 validation_failed` with `run_mismatch` on `/run_id` or `409 conflict` with reason `run_not_active`,
  `403 feature_disabled`).
- `ComponentReader.ForMessages(ctx, messageIDs)` returns the components of the given messages in
  position order, in the ambient transaction when there is one. messaging names only messages that
  `authorize` checked or that it loaded for an event of the same transaction (07 7.4 rule 1).

Card rules of `createMessage` (`cards.go` `cardRules`, after the decoder, D-M2-15-5): non-empty
`components` require `kind = card` (`invalid` on `/kind`); `kind = card` requires a component
(`required` on `/components`); agents and (M2-25) apps send cards, people get `invalid` on
`/components`; `run_id` follows the answer rules of M2-19 for every kind and on an agent's card
also binds the components, while on an app's card it is always `400 validation_failed` with
`run_mismatch` on `/run_id` (no run has an app as its agent, 11 section 2); `content_md` keeps the rule of M1-56. Then `ComponentWriter.Check` and the size rule:
the largest `message.updated` payload of the card (every component disabled, used and expiring,
`reply_to_message_id`, `edited_at`, pinned, the most distinct reactions of the longest symbol,
`message.max_mentions` mentions, the author with the longest handle and display name and an
avatar), with an empty content, `content_truncated` and the longest form of every ID and number,
built from configured bounds only (D-M2-15-28), must fit
`sync.event_payload_max_bytes`, else `too_long` on `/components`, because components cannot be cut
(04 section 2.2 rule 2; D-M2-15-26). `Messages.CardFits` checks the same reserve for
`updateComponent` after a new definition is written. In the unit of work the
message row is inserted first, then `Attach`, then `message.created` with the components is
appended and `MessagePosted` is published (in that order, D-M2-05-3); `201` carries them, a replay `200` the current ones. `messages.run_id` stays NULL for a
card (D-M2-15-6).

App cards (M2-25): an app sends `kind = card` with components through `createMessage` like any
other message, so `message.create` needs `write` on the place and `messages:write`; apps never
reach a dm channel (05 section 2.2). Posting needs no `components:write`, because 05 section 2.6
names that scope only for `component.update` and `interaction.respond`; without it the app's
components are inactive (authz README, "Apps as owners of commands and components (M2-25)") and
the app can neither change nor answer them. Every other card rule above applies unchanged (the
definition checks of `ComponentWriter.Check`, the server-set fields, the size rule and the request
hash `bothy-msg-v2`). The header of an app card is built from server fields only: the author,
`origin = app` and the component fields the server sets; text that imitates a system header stays
content (SEC-OUT-3, SEC-OUT-4), and an app card never carries a run. The reply that the job
`runs.app_dispatch` writes for a `200` of an app's interaction address goes through the same
`Create` with the app as actor and no token: it has `origin = app`, follows these rules and is
idempotent by `client_msg_id` when the job runs again (04 section 5.1).

Port methods for runs (none authorizes; runs decided):

- `LockCard(ctx, messageID)` locks the message row (`SELECT ... FOR UPDATE`, no visibility
  predicate) in the ambient unit of work and returns `CardFacts` (place, author and its kind,
  kind, `Deleted`, `Version`); `not_found` for an unknown message, internal outside a unit of work.
- `ComponentsChanged(ctx, messageID)` raises `messages.version` by one and appends
  `message.updated` with the representation of the card, its current components included; the
  caller holds the lock of `LockCard`. It returns the new version; internal outside a unit of work
  and for a missing or deleted card (D-M2-15-1, 02 section 4.7).
- `PrincipalOf(ctx, principalID)` returns the `PrincipalRef` of a principal the way
  `TriggerMessageOf` returns an author: `triggered_by` of `agent.invocation` for a run an
  interaction opened (D-M2-15-23).
- `Authors(ctx, messageIDs)` (M2-25) returns the author principal of each given message as a map
  from message ID to author ID, in one batched read of `author_principal_id` in the ambient unit
  of work; it checks nothing and has no visibility predicate. runs' `Interactions.DisableAuthoredBy`
  reads the cards with active components and no run (`run_id IS NULL`) and keeps those of one
  author through it, when `app.suspend` or `app.uninstall` disables the components of an app.

Representation: a new card's `201` answer and its `message.created` carry the components that
`Attach` returned; every other builder of `Message` (a replay of `createMessage`, `getMessage`,
`listThreadMessages`, `updateMessage`, the payloads of `message.updated`) fills `components`
through `decorate` with one `ForMessages` call per answer or page, never one per message, and only
for live cards; every other message, and every tombstone, has `components: []`.
`Component` carries no form values (they live only in `interactions`), so no `message.*` event
carries them.

## System lines of other modules (M2-12)

`AppendSystemLine(ctx, SystemLine{ThreadID, AuthorID, Code})` writes a system line inside the
caller's unit of work and never authorizes; runs calls it from `OnSessionAttached` when an
adapter reports `bind_failed: session_missing` (09 9.5 item 4), with the code
`messaging.SystemAgentSessionResumeFailed` and the run's agent as author. It follows the reopen
line of `createMessage` (brief M1-56 D6): the next `thread_seq` through `chat.AppendToThread`
without `ReopenIfClosed` (a system line never reopens a closed thread, R-DM-42, and writes no
`thread.reopened`), `domain.SystemLine` (kind and origin `system`, empty content, silent, a
server `client_msg_id`, no mentions, no `trigger_state`), `created_at` now, the instance search
language, `message.created` without the `client_msg_id` echo, then `MessagePosted` with `Kind`
`system`, which the acceptance of runs ignores (SR4). The caller principal of `AppendToThread`
is the principal of the request, as for the run anchor, and the author outside a request. It
returns the written line. Errors: internal outside a unit of work and for a code that is not a
system code of 03; `not_found` for an unknown thread.

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
## Read operations for the search coordinator (M3-14)

The module `search` owns no tables; it reads messages only through the port `SearchReads`
(`search.go`, [14-search.md](../../../../docs/spec/14-search.md) section 10, R-SU-09), which
`*app.Service` implements (`app/search.go`) over the store port `app.SearchReadStore` (part of
`Store`) and the queries of `queries/search.sql`. Neither operation authorizes: the coordinator
passes the visible channels of `Authorizer.Scope` and checks every delivered hit with
`message.read` again (14 4.3); both statements carry `m.channel_id = ANY(@channel_ids::uuid[])`
(07 section 7.4 rule 1, AZ-P12).

- `SearchMessages(ctx, searchkit.Query)` runs one statement (14 4.8): the live rows of the visible
  channels whose `search_vector` matches the tsquery the search module built, with the filters of
  the query (`ThreadIDs` on `thread_id`, nil restricts nothing and an empty list keeps nothing;
  `AuthorID` on `author_principal_id`; `After` and `Before` on `created_at`, half open; `HasFile`
  on `has_attachments`, true with and false without attachments) and `created_at` at or before
  the reference time of the first page. The newest `search.max_candidates` of them are the
  candidates; one extra row reports `CandidatesCapped` and is never ranked or returned, and the
  flag comes from the candidate relation, so an empty page after the keyset still reports it.
  `relevance` scores `ts_rank(search_vector, tsq, search.rank_normalization)` divided by
  `1 + age / search.recency_scale` with the age measured from the reference time, never from
  `now()`, and orders by score, then id, descending; `newest` orders by `created_at`, then id,
  descending, with score 0. The page starts strictly after `Query.From`, reads the page size plus
  one row and sets `Page.Next` to the position of its last hit exactly when that row exists.
  Only the rows of the page get a headline: `ts_headline` in `messages.search_language` over
  `content_md` cut at `search.headline_input_max_chars`, with the options of
  `Limits.HeadlineOptions` and the random markers, split with `searchkit.Split`, so content never becomes markup
  of the excerpt: `ts_headline` drops the HTML tags of the content (its parser reads them as tags)
  and keeps entities and any marker look-alike as text. This is the only read of `search_language` and `search_vector`; it returns
  no message row (D-M3-13-10). A statement that hit the statement timeout of the search unit
  (`search.statement_timeout`) returns `searchkit.ErrTimeout`, a statement the caller cancelled
  the context error, anything else an internal error.
- `HitMessages(ctx, messageIDs, channelIDs)` returns the hit objects: the live messages of
  `messageIDs` in the channels of the kept hits, rendered by the code path of `History`
  (`listThreadMessages`): content cut at `message.list_content_max_bytes` with
  `content_truncated`, attachments, reactions, own reactions of the principal of the request, pin
  and components. The result follows the order of `messageIDs`; a missing, deleted or foreign id
  is absent and a repeated id appears once.

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
write), remembers the mentions, writes the tombstone (`content_md = ''`, `has_attachments =
false`, `deleted_at`, `deleted_by`, next version), removes the revisions, mentions, reactions and
the pin (M1-59), then, only when the locked row had `has_attachments`, the attachments through
`files.Attachments.DeleteForMessage` (M3-07), counts the message down (`chat.Threads.DecrementMessageCount`), redacts the
payloads of every stored event with resource `message` and this id through
`eventlog.Redactor` with `domain.RedactPayload`, publishes `MessageDeleted` and appends
`message.deleted` (payload `MessageDeletedPayload`, after the redaction, so never redacted). A
deletion of another principal's message writes an `allow` audit entry with only `thread_id`,
`channel_id` and `author_principal_id`; denials take the deny path of authz.

`listMessageRevisions`: `authorize(message.revisions.read)`, the revisions ascending and
unpaginated, `{"items": []}` for a deleted message.

Lock order: the thread row (through `chat`) before the message row; several message rows in
ascending `id`; then (deletion with attachments, M3-07) the `files` rows of the attachments in
ascending `id` and the rows of `message_attachments`, both through `files`; the counters last
(03 section 6.3, enforced by the unit of work). A thread move
(M1-62) also locks the thread first, so a deletion and a move cannot deadlock; an edit locks only
the message row, which serialises edits, deletions and (from M1-59) reactions and pins of one
message, and revision numbers come from `max(revision) + 1` under it.

Redacted payloads (04 section 2.2 rule 5): `message.created` and `message.updated` become
`{"message": T}` with `T` the event form of the tombstone (every stored field, `content_md` empty,
no attachments, mentions, reactions or components, `pinned` false, `deleted_at` and the new
version, no `my_reactions`, no `content_truncated`); `reaction.added` and `reaction.removed` keep
`message_id`, `thread_id` and `principal_id` with `emoji: ""` and `count: 0`; `pin.added` and
`pin.removed` stay; any other type fails the deletion. The envelope columns never change.

## Attachments (M3-07)

The flag: `messages.has_attachments` is `true` exactly when the message was created with at least
one attachment and is not deleted (03 `messages`, R-DM-43). The INSERT of a send writes it, the
tombstone UPDATE resets it, and the CHECK `messages_has_attachments_deleted_check` refuses a
deleted message with the flag. Every read that builds `domain.Message` returns it
(`queries/messages.sql`, `history.sql`, `mentions.sql`, `export.sql`). `messaging` never reads
`message_attachments`; `files` owns that table and never reads `messages` (03 principle 12).

The create path (`app/create.go`, also the first message of `createThread`, `app/threads.go`):
`message.create` with `HasAttachments` (`+upload`), the body rules above, the request hash with
the ids in request order (unchanged encoding: an empty list hashes as before), then in the unit of
work after the repetition lookup: `AppendToThread`, the INSERT with `has_attachments`, the
mentions, `files.Attachments.Attach` with the message id, its thread, the channel of the row just
inserted (the channel `AppendToThread` returned under the thread lock), the author, the ids and
the pointer of the list (`/attachment_file_ids`, in `createThread`
`/first_message/attachment_file_ids`), then `message.created` and `MessagePosted` with
`HasAttachments`. A refusal of `files` (`409 conflict`, reason `file_not_attachable`, `detail`
the pointer of the entry) rolls the unit back with the message and its thread number. A
repetition attaches nothing and answers the stored message with its attachments. A message
without attachments makes no `files` call and no extra statement (`perf.message_create_p99`).
Every caller of the create use case (incoming webhooks, the run anchor, system lines) goes
through the same path; the anchor and system lines never have attachments.

The representation (`app/view.go`): `Message.HasAttachments` from the row, `Message.Attachments`
never nil. `decorate`, the one decoration of `stored`, `History`, `Get`, the mention list, the
export and `SetTriggerState`, fills the attachments with one `files.Attachments.ForMessages` call
per list, only for live messages with the flag, none when no message has it; a tombstone shows
`[]`. The `201` of a send and its `message.created` use the result of `Attach` instead of a read.
The file is the `files.File` of the attachment mapped into the port (`Attachment`,
`AttachedFile`, `AttachedThumbnail`) without `scope_channel_id`, which an attached file never
shows (after a move it would name a channel the reader may not see); `sha256` only when `files`
set it (`upload_state` complete), `thumbnails` always an array, no JSON `null` (ADR 0053). The
event form (`messageJSON`) and the REST form (`MessageREST`) list them in position order. An edit
never changes the attachments; `message.updated` carries them like `message.created`. The redacted
payload of a deleted message lists none.

Deleting (`app/edit.go`): after the tombstone, the revisions, mentions, reactions and the pin, and
only when the locked row had the flag, `files.Attachments.DeleteForMessage` removes the rows of
`message_attachments` and sets `files.deleted_at` in the same unit of work, without
`file.updated`.

Lock order of a send with attachments: thread, message (insert), `files` rows in ascending `id`,
`message_attachments` (insert), then what subscribers of `MessagePosted` lock (`runs`). Of a
deletion: thread, message, `files` rows in ascending `id`, `message_attachments`.

Tests: `domain/attachments_test.go` (`TestAttachmentBodyRules`), `app/attachments_test.go` over a
fake `files.Attachments` (`TestCreate_NoFilesCallWithoutAttachments` and the authorization,
hash, replay, limit, `createThread`, deletion and edit cases), `adapters/postgres/attachments_test.go`
(`TestRepository_HasAttachments`, `TestHasAttachments_CheckRejectsDeletedWithFlag`) and
`adapters/httpapi/attachments_test.go` (`TestMessageREST_Attachments`). The end-to-end tests over
HTTP with real files live outside the module.

## Reactions and pins (M1-59)

A reaction is one Unicode symbol of a person, agent or app on a message, never a status: no
operation touches `trigger_state` or `messages.version`. A pin is moderation with `manage`.

Writes check in this order: the symbol (`SymbolNormalizer`: valid UTF-8, NFC, exactly one
extended grapheme cluster of at most `reaction.emoji_max_bytes` bytes, no Cc, Cs, Zs, Zl, Zp or
Bidi control, not only Cf and Mn; `400 validation_failed` (reason `invalid_emoji`) at `#path/emoji`
before any lookup), then the message in the visible channels with `authorize` (`reaction.add` on
the message, `reaction.remove` on the caller's reaction, `pin.create` and `pin.delete` on the message; a
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

## Channel export (M3-11)

`exportChannel` (`GET /api/v1/channels/{channel_id}/export?from=...&to=...`, `export.channel`,
`rl.export`) streams one `Message` per line as `application/x-ndjson`. The checks run in this
order, and nothing reaches the client before the last one:

1. The range rule without any lookup: `from` strictly before `to`, else `400 validation_failed`
   with one entry at `#query/to` and code `invalid`; no upper bound on the length
   (D-M3-11-1).
2. `authorize(export.channel)` on the channel in request mode: an unknown and an invisible
   channel give the same `404`; read without `export` `403 forbidden` with reason
   `capability_missing`, a principal without a session of a human `403 forbidden` with reason
   `step_up_not_possible`, no fresh step-up `401 step_up_required`. `authz` records the denials.
3. `Authorizer.Scope(message.read)`, then exactly one allow entry `export.channel` with only
   `from` and `to`, recorded with `auditing.Recorder.Record` in its own unit of work and committed
   before the status line (D-M3-11-4).

The stream walks thread pages (`ListExportThreadIDs`) and message pages
(`ListExportThreadMessages`), both of `api.page_size_max` rows and both with
`m.channel_id = @channel_id AND m.channel_id = ANY(@visible_channel_ids)`: threads ascending by
their UUIDv7 id, the messages of a thread ascending by `thread_seq`, so `(thread_id, thread_seq)`
is a total, resumable keyset (D-M3-11-3). A batch is one message page. Each line is the shared
representation with the full `content_md`, without `content_truncated` and `my_reactions`, and a
tombstone at its `thread_seq` (D-M3-11-2).

Re-check per batch (07 section 8.2 item 2; D-M3-11-5, D-M3-11-9): there is no snapshot across
batches. Before every page the stream starts a fresh request memo
(`authorize.WithoutRequestScope`, then `authorize.WithRequestScope`), checks the credentials
through `CredentialCheck` (`kernel/actor.Authenticator.Revalidate`; a revoked session or device
raises no rights version), checks `channel.read` in filter mode and takes `Scope(message.read)`
again; a refusal ends the stream with `ErrExportAborted`. The page is read and built in a short
read-only unit of work that ends before its lines are written. Every row passes `message.read` in
filter mode on the same memo; a refused row is dropped, logged with only the action and the
reason and counted in `bothy.authz.errors` as `filter_mismatch`. The capability `export`
and the step-up are checked only at the start. `internal/wire` binds `CredentialCheck` lazily
(`lazyCredentials`), because the session step that builds the Authenticator runs after the chat
step.

Abort (D-M3-11-6): the handler writes `200` with only `Content-Type`; the other headers come from
the chain. After the first byte an aborted stream, a read error, the deadline or a shutdown logs
only a cause code and the route pattern and panics with `http.ErrAbortHandler`, so HTTP/1.1 ends
without the final chunk and a client never takes a cut export for a complete one. A write error
(client gone) ends the stream quietly; a range without messages gives `200` with an empty body.
The route flag `Download` gives the response `http.transfer_timeout` (D-M3-11-7). Nothing is
stored in `files` (D-M3-11-8).

## Imported messages (M8-11)

`messaging.ImportMessages` (`importmessages.go`, implemented by `app.ImportMessagesPort`, which
`Service.ImportMessages()` returns) runs only inside the unit of work of an import batch and
authorizes nothing (`import.apply` decided). `Append` checks every message (content at most
`message.max_content_bytes`, else `400 validation_failed` with the code `too_long` at `/content_md`; no U+0000; a visible character
unless it has files; at most `message.max_attachments`; the reply target in the same thread),
takes the numbers of all of them in one `chat.ImportThreads.Reserve` (row lock, no reopen) and
inserts each with `queries/import.sql` (`InsertImportedMessage`, behind the store port
`app.ImportStore`, `Deps.Imported`): `origin = import`, `kind = text`, `silent`, no mentions, no
`trigger_state`, a server `client_msg_id`, the request hash over `import:<item id>`,
`import_item_id`, `created_at` and `edited_at` of the source without a revision, the instance
search language (14 3.1). Files attach through `files.Attachments.Attach` with the mapped author.
It never publishes `MessagePosted` and never appends `message.created`, so no run, invocation,
notification or push follows (05 5 rule 1, 04 15.8); `bothy.message.created` counts them with
origin `import`. `Pin` pins a live message of the thread under `pin.max_per_thread` with the given
`pinned_by` and no `pin.added`; `ThreadOf` answers the thread of a message for the starter link
of an imported thread. `Deps.ImportThreads` is the port of chat; nil answers internal.

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

## Run context windows (M2-13)

- Port methods `Messages.ContextWindow(ctx, ContextQuery)` (`ContextPage`: items ascending with
  full content, `HasOlder`) and `Messages.ContextMessage(ctx, id)` (one message with full content,
  a tombstone as stored; `ok` false for unknown and refused alike) in `app/context.go`, for the run
  context of `runs`.
- `ListContextWindow` (`queries/context.sql`) is one statement with `LIMIT` and an `EXISTS` for
  `HasOlder` over `(thread_id, thread_seq)`; it filters tombstones and the excluded author in SQL,
  so the limit counts only selectable messages. Neither it nor `GetMessage` of `ContextMessage`
  carries the visible-channel predicate: `runs` authorized `run.context.read` on the run of the
  thread first, and every returned row passes `message.read` in filter mode (a refused row of the
  window is dropped, logged and counted as `filter_mismatch`). Both stand in the allowlist of
  `TestQueriesCarryVisibleChannelPredicate` and `tools/lint/sql_visibility.yaml`.

## Agent answers and announcements (M2-19)

- `MessageCreate.run_id` is enabled. The body rule (`domain.CheckRunID`) refuses it for every
  principal that is not an agent: `400 validation_failed`, pointer `/run_id`, `errors[].code`
  `run_mismatch` with `domain.RunMismatchDetail`. An agent's `run_id` passes unchanged into
  `MessagePosted.RunID`; messaging does not check the run, because it may not import `runs` (02
  4.4). The acceptance subscriber of `runs` checks it in the same unit of work and vetoes it (see
  the runs README).
- `MessagePosted.RunID` is the run the message belongs to: the anchored run for `kind = run`
  (`CreateRunAnchor`), the `run_id` of the request for an agent's answer, nil otherwise.
- `run_id` of an answer is never stored (`messages.run_id` names only the run of a live box
  anchor, 03) and is not part of the request hash (04 5.1 point 3), so a replay of an answer is
  `200` with `Idempotent-Replay: true` also after its run ended, and an answer refused with
  `run_not_active` can be sent again with the same `client_msg_id` without `run_id`.
- `createThread` refuses `first_message.run_id` for every principal (`domain.CheckFirstMessageRunID`,
  pointer `/first_message/run_id`, `run_mismatch`) before the unit of work: a new thread has no run.
- Announcements without a trigger (09 14) are ordinary `createMessage` or `createThread` calls of
  the agent with its own rights (a grant with `write` or the operator rule); a report channel is a
  normal channel. Limit of 09 14: the server checks only where an agent writes, never what it
  writes; nothing here inspects content.

## Notes for the build

- Idempotency through the pair of author and `client_msg_id` (04, 5.1).
- Deletion empties the content and redacts the payload of the events in the same transaction (03, 6.5).
- A mention of a principal without read permission is stored but triggers nothing (03, `mentions`).

Rulings: docs/DECISIONS.md M1-56, M1-57, M1-58, M1-59 and M1-60, D-M3-11-1 to D-M3-11-10 in docs/decisions.d/M3-11.md, D-M2-15-1 to D-M2-15-30 in docs/decisions.d/M2-15.md, and the rows `D-M3-07-<n>` in docs/decisions.d/M3-07.md (empty content with attachments, `scope_channel_id` absent once attached, no `file.updated` on deletion with the message).
Rulings: docs/DECISIONS.md M1-56, M1-57, M1-58, M1-59 and M1-60, D-M3-11-1 to D-M3-11-10 in docs/decisions.d/M3-11.md D-M2-15-1 to D-M2-15-30 in docs/decisions.d/M2-15.md and the rulings of M2-25 in docs/decisions.d/M2-25.md.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [06-api.md](../../../../docs/spec/06-api.md), [14-search.md](../../../../docs/spec/14-search.md)
