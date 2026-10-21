# Modul `chat`

Status: spaces and the sidebar (M1-52), channels and forum tags (M1-53), operators with I1 (M1-54), threads (M1-55), moving threads (M1-62), channel behaviour with agent configuration (M2-02) and the project binding with readiness (M2-03) built; `createThread` is built in `messaging` (M1-56); `Threads.InfoForShare` for the read states and drafts of sync (M1-61); `Threads.Counters` for the unread counters of the snapshot of sync (M1-63); `Threads.UnreadCandidates` for the badge of notify (M3-19).
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Bereiche, Seitenleisten-Zustand, Kanäle mit Verhalten, Skills, Projektbindung, DM-Teilnehmer, Forum-Tags, Threads mit Thread-Sequenz.

## Code

- `internal/chat` (`chat.go`): the port. `Structure` with its space and sidebar part (M1-52):
  `ListSpaces`, `GetSpace`, `CreateSpace`, `UpdateSpace`, `MoveSpace`, `ArchiveSpace`,
  `PutSidebarSpace` and `ListSidebarStates` (for `getMySidebar`, M1-61); the DTOs `Space`,
  `SpaceView`, `SidebarState`, `Precondition` (the parsed If-Match) and one command per body; the
  in-transaction event `SpaceArchived{SpaceID}`; `Register` declares the durable events
  `space.created`, `space.updated`, `space.archived` and `sidebar.updated`.
- `domain/`: the space entity (every change bumps `Version` once), the checks of name
  (`SpaceName`: trimmed, not blank, at most `space.name_max_chars` Unicode characters, SEC-IN-4),
  colour and icon (keys of `kernel/designkeys` only), reason, and the sidebar key rule
  (`SidebarKey`). `domain/fracindex`: `KeyBetween(a, b)` and `Valid(key)` of the base-62
  fractional index with integer part (docs/DECISIONS.md M1-52/2).
- `app/`: `Service`, one use case per operation, and the outgoing port `Store`. `Config` binds
  `space.name_max_chars` (`wire.ChatConfig`).
- `adapters/postgres/`: `queries/spaces.sql` and `queries/sidebar.sql`, the sqlc output in
  `sqlcgen/` (entry in `services/server/sqlc.yaml`) and `Repository` (implements `app.Store`).
  Locks and writes run only inside a unit of work; a taken name is `409 conflict` on `/name`, no
  error carries SQL text, constraint names or values.
- `adapters/httpapi/`: `Spaces`, the handlers of `listSpaces`, `createSpace`, `getSpace`,
  `updateSpace`, `moveSpace`, `archiveSpace` and `putMySidebarSpace`; `ETag` from `version` on
  `getSpace` and `updateSpace`.
- `adapters/authzsource/`: `Source`, the space and channel part of `authz.StructureSource`
  (`Space`, `SpaceArchived`, `Spaces`, `SpaceDetails`; since M1-53 `Channel`, `Channels`,
  `ChannelsOperatedBy`, `ChannelDetails`, with `Archived` set for an archived channel and for every
  channel of an archived space; since M1-54 `ServedTargets`, since M1-55 `Thread`); `wire.NewStructureSource`
  joins it with the empty dm part until V3.
- Migrations `00023_chat_spaces.sql` (`spaces`, `user_space_states` exactly as 03 section 4.5,
  sort keys `COLLATE "C"`, and `grants.target_space_id -> spaces(id)` `NOT VALID`) and
  `00024_grants_space_fk_validate.sql` (validates it). `bothy_app` may read, insert and update
  `spaces` and also delete `user_space_states`.

## The use cases (M1-52)

| Operation | Action | Rights change | Event |
|---|---|---|---|
| `listSpaces` | `space.read` (Scope, then per item in filter mode) | no | |
| `getSpace` | `space.read` | no | |
| `createSpace` | `space.create` on the instance (owner) | no | `space.created` |
| `updateSpace` | `space.update` (`manage`), If-Match | no | `space.updated` |
| `moveSpace` | `space.reorder` on the instance (owner) | no | `space.updated` |
| `archiveSpace` | `space.archive` (owner, step-up) | yes (target: the space) | `space.archived` |
| `putMySidebarSpace` | `sidebar.update` (self and read, people only) | no | `sidebar.updated` (self) |

- Visibility (07 section 4.3): `read` on the space or on one of its channels; a missing and an
  invisible space give the same `404`. `listSpaces` filters in SQL with
  `archived_at IS NULL AND ($all OR id = ANY($visible))` from `Scope` (all spaces for owners and
  instance-wide readers, built per request, so a new space needs no new rights version), then
  authorizes each row. `SpaceView` reports the effective level and capabilities from
  `authz.RightsQuery`; a space seen only through a channel reports `read` and none.
- Global order: create inserts after `after_space_id` or at the end, move after `after_id`,
  before `before_id` or at the start; both fields, the space itself and an unknown or archived
  target are `400` on the field. Create and move take the order lock (an advisory lock, then
  every space that is not archived in ascending id) and a move writes exactly its own row
  (docs/DECISIONS.md M1-52/3).
- Archive (07 section 4.5): final. A first unit authorizes and answers an archived space without
  a write (`no_change`); a second unit, a declared rights change with the space as target, sets
  `archived_at`, bumps the version, publishes `SpaceArchived` and appends `space.archived`. authz
  caps every level in the space at `read` (owners included) and writes `access.changed` with the
  space in `changed` at `read` for everyone whose level was above. `updateSpace` and `moveSpace`
  answer an archived space with `409 conflict`; `getSpace` still answers it.
- Sidebar (04 section 14): agents and apps get `403 subject_kind_not_allowed`; `collapsed` is
  required, `sort_key` must be a fractional index key, `use_global_order` removes the personal key
  and excludes `sort_key`, neither keeps the stored key. The row is upserted with version + 1 and
  the caller's device; `sidebar.updated` goes to the caller without `resource.version` (04
  section 2.2 rule 3).
- Audit: one allow entry per mutation of a space (`changed_fields`, `reason`); the sidebar is not
  audited.

Rulings: docs/DECISIONS.md M1-52.

### Tests

`domain/domain_test.go`, `domain/fracindex/fracindex_test.go` (vectors and properties),
`register_test.go`, `adapters/httpapi/*_test.go` over PostgreSQL 18 with real authz, the chat
source and the full chain with response validation against openapi.yaml (order, concurrency,
If-Match, palette keys, SEC-T07, names, visibility through a channel, visibility without a rights
version, the archive as rights change, rights per operation, events, sidebar), and
`migrations/chat_spaces_test.go`.

## Channels and forum tags (M1-53)

- Port: `Structure` adds `ListSpaceChannels`, `CreateChannel`, `GetChannel`, `UpdateChannel`,
  `MoveChannel`, `ArchiveChannel`, `ListForumTags`, `CreateForumTag`, `UpdateForumTag`,
  `MoveForumTag` and `DeleteForumTag` with the DTOs `Channel`, `ChannelView`, `ChannelOperator`,
  `ForumTag`, `MovePosition` and one command per body; the in-transaction event
  `ChannelArchived{ChannelID, SpaceID, ArchivedAt}` (no subscriber in M1). `Register` adds the
  durable events `channel.created`, `channel.updated`, `channel.archived` and `thread.created`.
- Domain: `Channel` (kinds `text`, `forum`, `dm`; `OperatorMode` read side `inherit`, `none`,
  `agent`), `ChannelName`, `ChannelTopic`, `TagName` (SEC-IN-4 through `kernel/textcheck`, limits
  `channel.name_max_chars`, `channel.topic_max_chars`, `forum_tag.name_max_chars`),
  `CheckCreateKind`, `ChannelChange`, `ForumTag`, `TagChange`, `MovePosition` with `KeyAt` (the
  fractional index), `Thread` (the main thread).
- Store: `queries/channels.sql`, `queries/forum_tags.sql`, `queries/threads_main.sql` (the insert of
  the main thread only; the thread use cases come with M1-55).
- Handlers: `Channels` in `adapters/httpapi` for the eleven operations; `ETag` on `getChannel`
  and `updateChannel`.
- Migrations `00026_chat_channels.sql` (`channels`, `channel_skill_bindings`, `forum_tags`,
  `threads` as 03 section 4.5, without `threads.search_language`, `threads.title_search` and its
  index (M3-13) and without the foreign key `threads.parent_message_id -> messages` (M1-56); and
  `grants.target_channel_id -> channels(id)` `NOT VALID`) and `00027_grants_channel_fk_validate.sql`.
  `bothy_app` may read, insert and update `channels`, `forum_tags` and `threads` and also delete
  `channel_skill_bindings`.

| Operation | Action | Rights change | Event |
|---|---|---|---|
| `listSpaceChannels` | `space.read`, then `channel.read` (Scope, per item in filter mode) | no | |
| `createChannel` | `channel.create` on the space (`manage`) | no | `channel.created`, for text `thread.created` |
| `getChannel` | `channel.read` | no | |
| `updateChannel` | `channel.update` (`manage`), If-Match | no | `channel.updated` |
| `moveChannel` | `channel.read` (visibility), `channel.reorder` on the space (`manage`) | no | `channel.updated` |
| `archiveChannel` | `channel.archive` (`manage`, step-up) | yes (target: the channel) | `channel.archived` |
| `listForumTags` | `channel.read` | no | |
| `createForumTag`, `updateForumTag`, `moveForumTag`, `deleteForumTag` | `forum_tag.manage` (`manage`) | no | `channel.updated` |

- A text channel gets exactly one main thread in the same unit of work (R-DM-11); a forum none.
- `listSpaceChannels` filters with `space_id = $space AND archived_at IS NULL AND (id = ANY($visible)
  OR space_id = ANY($fully_visible) OR $instance_wide)` from `Scope`, so a new channel in a visible
  space is listed without a new rights version (04 section 7.2).
- An archived channel, and every channel of an archived space, is read-only for everyone (authz
  caps the level at `read`; writing actions get `403 channel_read_only` with `reason:
  channel_archived`); manage changes of it and its tags are `409 conflict`.
- Every tag change bumps `channels.version` and appends `channel.updated` (03 `forum_tags`).
- Audit details carry codes and IDs only: `{space_id, channel_id, kind}` on create,
  `changed_fields` on update, `reason` on archive, `{op, tag_id}` on tag changes.
- Create and move lock the space row (the order lock of the channels of a space); every other
  change locks the channel row.

Rulings: docs/DECISIONS.md M1-53. Tests: `domain/channel_test.go`,
`adapters/authzsource/source_test.go`, `adapters/httpapi/channels_test.go` and
`migrations/chat_channels_test.go`.

## Operators (M1-54)

- Port: `Structure` adds `SetSpaceOperator` and `SetChannelOperator` with `OperatorAssignment`,
  `SetSpaceOperatorCmd` and `SetChannelOperatorCmd` (target, assignment, reason, If-Match).
- Domain: `OperatorAssignment` with `CheckForSpace` (none or agent) and `CheckForChannel`
  (inherit, none or agent; `agent_id` exactly with mode agent, field codes `not_allowed_for_mode`,
  `inherit_only_for_channels`, `agent_not_active`), `EffectiveOperator` (the effective operator of
  a channel from its columns and the space default), `Channel.Assignment`, `Channel.SetOperator`,
  `Space.Assignment`, `Space.SetOperator`.
- Store: `queries/operators.sql` (the operator updates, the version of an inheriting channel, the
  lock of the inheriting channels, the served channels and spaces of an agent after 10 section
  5.3, the facts of a thread).
- Source: `adapters/authzsource.Source` answers `ServedTargets` (D5 of the brief, docs/DECISIONS.md
  M1-54/2) and `Thread` (channel, kind, fixed operator, creator), so `authorize` applies the
  operator rule to channels and threads and the rights hook checks I1 on real targets.
- Handlers: `setSpaceOperator` on `Spaces`, `setChannelOperator` on `Channels`; `ETag` from the
  version.

| Operation | Action | Rights change | Event |
|---|---|---|---|
| `setSpaceOperator` | `space.operator.set` (owner, step-up), If-Match | yes (old and new default) | `space.updated`, `channel.updated` per inheriting channel |
| `setChannelOperator` | `channel.operator.set` (owner, step-up), If-Match | yes (old and new effective operator) | `channel.updated` |

- Order of checks: authorize, If-Match, the body, the archive state (docs/DECISIONS.md M1-54/1).
  `agent_id` must name an active agent principal with an agents row (identity directory and the
  agent policy source).
- The same assignment answers `200` and records `no_change`, without version, event or rights
  version. Otherwise the second unit of work is the declared rights change: `permission_version`
  first, then the space and its inheriting channels (ascending id) or the channel; the old and new
  operator are named through `authz.RightsChanges.AddAffected`; I1 is verified before the commit
  (`409 i1_violation`, deny entry in an own transaction).
- Audit details `{target_kind, target_id, old_mode, old_agent_id, new_mode, new_agent_id, reason}`.
- No `thread.updated`: the operator of a thread is fixed (R-DM-12); I1 still counts its channel.

Rulings: docs/DECISIONS.md M1-54. Tests: `domain/operator_test.go` and the cross-module tests in
`services/server/test/integration` (AZ-M21 to AZ-M25, AZ-M29, BR-01 to BR-03, AZ-P7 for both
actions, explanation, preview and view-as with real sources).

### Guided flow of the operator actions (M2-21)

`SetSpaceOperatorCmd` and `SetChannelOperatorCmd` carry an optional `*chat.ResolveI1`. The first
unit validates it with the body (`checkOperatorBody`); an assignment without effect switches
nothing (docs/DECISIONS.md D-M2-21-3). The second unit calls `authz.I1Resolver.ResolveI1` after the
checks under the lock (archive state, If-Match, no-op) and before the operator columns change.
`internal/wire` passes the resolver of authz (`ChatWrites.Resolver`, `Deps.Resolver`).

## Threads (M1-55)

- Port `chat.Threads` (`threads.go`): `Create`, `AppendToThread`, `Info` and `View` for
  `messaging` (M1-56 sends messages and enables `createThread` through them; `View` is the
  thread of its `createThread` answer), and `List`, `Get`, `SetTitle`, `Close`,
  `Reopen` and `SetTags` for the six handlers; the DTOs `Thread`, `ThreadInfo`, `ParentMessage`,
  `CreateThreadCmd`, `AppendCmd`, `Appended`, `ListThreadsQuery`, `ThreadPage`, `SetTitleCmd` and
  `SetTagsCmd`. `Register` adds `thread.updated`, `thread.closed` and `thread.reopened`.
  `InfoForShare` (M1-61) is `Info` with `FOR SHARE` on the thread row until the end of the
  caller's unit of work (`Store.ShareThread`), so sync copies the channel of a thread that no
  concurrent move (M1-62) changes before its commit; `ThreadInfo` carries `LastMessageSeq` and
  `UnreadFloorSeq`. sync reads the sidebar through `Structure.ListSidebarStates` and the kind and
  space of a channel draft through `Structure.GetChannel`.
- `Counters` (M1-63, `ThreadCounter`) returns `thread_id`, `channel_id`, `last_message_seq` and
  `unread_floor_seq` of the threads of the given channels with `last_message_seq >
  unread_floor_seq` (query `ThreadCounters`, `channel_id = ANY($1)`), without authorization: sync
  passes only the visible channels of the caller and checks `thread.read` per thread it lists. The
  read positions live in sync, which combines both in Go (07 section 7.4 rule 6).
- `Locate` (M2-05) returns the `ThreadInfo` of `Info` for a batch of threads in one statement
  (query `LocateThreads`, `id = ANY($1)`, joined with channel and space), ascending by id; an
  unknown id is absent. It does not authorize and reads in the ambient transaction: runs keeps only
  threads whose channel is in its `Authorizer.Scope` and authorizes per item ("Running now").
- `UnreadCandidates` (M3-19 design note 16 a, `ThreadPosition`) returns `thread_id`,
  `channel_id`, the channel kind (`text`, `forum` or `dm`), `last_message_seq` and
  `unread_floor_seq` of the threads of the given channels with `last_message_seq >
  unread_floor_seq`, ascending by thread id (query `UnreadCandidates` in `queries/unread.sql`,
  `channel_id = ANY($1)`, joined with the channel); no channel returns an empty list without a
  query. It never authorizes and applies no visibility of its own: the badge of notify passes the
  channels of the recipient's `authz` Scope for `push.deliver` and confirms every thread that counts
  with `push.deliver` (13 section 8). The store is the port `app.UnreadStore` beside `Store`
  (`Deps.Unread`, which falls back to `Store` when it implements the port, like `Behaviors`);
  without one the method answers internal.
- Port `chat.ThreadsByParent` (`threads.go`, M2-27): `ThreadByParentMessage(ctx, messageID)`
  returns the `ThreadInfo` of the thread whose `parent_message_id` is `messageID` and `found =
  false` when no thread started from it, without authorization (callers authorize first). It is a
  port of its own, so the implementations of `Threads` stay unchanged; `app.Service` implements it
  over `Store.ThreadByParentMessage` (query `GetThreadByParentMessage`). The module apps answers
  the replay of an incoming webhook post with a title in a text channel with it: the answer names
  the thread of the first answer (docs/DECISIONS.md D-M2-27-19).
- Domain (`domain/thread.go`): `Thread` with `ThreadKind`, `ThreadState`, `TitleSource`; `Close`
  (a main thread is `409 invalid_state_transition`, a closed thread unchanged), `Reopen`,
  `SetTitle`; `CheckThreadTitle` (`thread.title_max_chars` Unicode characters, SEC-IN-4 through
  `kernel/textcheck`, empty allowed, stored as given); `PlaceholderTitle` (trimmed, inner white
  space collapsed, `strings.EqualFold` against `thread.placeholder_titles`, an entry plus one space
  and ASCII digits; the empty title always) and `CreateTitleSource`; `CheckTagSet` (duplicates,
  `thread.max_tags`) and `DiffTags` with the restricted flag.
- Store: `queries/threads.sql` (read, lock, insert, save, the thread sequence and the two keyset
  lists) and `queries/thread_tags.sql`; `DeleteForumTag` deletes the `thread_tags` rows of its tag
  in the same transaction, without an event per thread (clients reload tags on `channel.updated`).
- Handlers: `Threads` in `adapters/httpapi`; `ETag` on `getThread`, `setThreadTitle` and
  `setThreadTags`. `Cursors` in the same package is the cursor codec of the list
  (`httpserver.EncodeCursor`, port `app.Cursors`).
- Migration `00028_chat_thread_tags.sql`: `thread_tags` as 03; `bothy_app` may read, insert and
  delete it.

| Operation or port call | Action | Event |
|---|---|---|
| `listChannelThreads` | `thread.read` on the channel, then per item in filter mode | |
| `getThread` | `thread.read` | |
| `setThreadTitle` | `thread.title.update`, If-Match required | `thread.updated` |
| `closeThread` | `thread.close` | `thread.closed` |
| `reopenThread` | `thread.reopen` | `thread.reopened` |
| `setThreadTags` | `thread.tags.set`, If-Match optional; restricted tags with the restricted flag (`manage`) | `thread.updated` |
| `Threads.Create` | none (messaging authorized `thread.create`); restricted tags with `thread.tags.set` | `thread.created` |
| `Threads.AppendToThread` | none (messaging authorized `message.create`); `thread.reopen` on an implicit reopen | `thread.reopened` on a reopen |
| `Threads.LockForMessageChange` | none (messaging authorized `message.delete`) | |
| `Threads.DecrementMessageCount` | none (messaging authorized `message.delete`) | |

- Thread sequence (03 section 6.2): `AppendToThread` locks the row and runs one `UPDATE threads
  SET last_message_seq = last_message_seq + 1, ... RETURNING`; a rolled back unit returns the
  number, so the committed numbers are gapless. `Create` and `AppendToThread` fail with `internal`
  outside a unit of work. `Info` and `View` do not authorize.
- Operator (R-DM-12, R-DM-21): `Create` stores the effective operator of the channel at that
  moment, `NULL` for none; a later operator change of the channel leaves it. The representation
  adds `operator_agent_id` and the current `billing_mode` from `authz.AgentPolicySource` (bound in
  `internal/wire`); main threads carry neither.
- Title source (09 section 12): a person with a title that is neither empty nor a placeholder
  creates `manual`, everything else `auto`; `setThreadTitle` by a person sets `manual`, agents and
  apps keep the source and get `409 title_manual` on a manual one; a main thread is `400`
  (`main_thread_has_no_title`).
- Tags (D5): only a forum post (`not_a_forum_post`), no duplicates (`duplicate`), only live tags of
  the forum (`unknown_tag`), at most `thread.max_tags` (`limit_reached`); the same set answers
  without an event.
- List (D6): text channels list their threads of kind `thread`, forums their posts; filters
  `state` and `tag_id` (an unknown tag gives an empty page), sort `last_activity` or `created`
  (default the channel's `forum_default_sort`), keyset `(time DESC, id DESC)`, `limit` default
  `api.page_size_default`, capped at `api.page_size_max`, no totals. The cursor is bound to caller,
  channel, filters and sort; any other combination is `400 cursor_invalid`.
- Archived channels: authz caps the level at `read`, so close, reopen, title and tags are `403
  channel_read_only` with `reason: channel_archived`; reading stays.

- Message count (M1-58): a deletion in messaging locks the thread with `LockForMessageChange`
  (`SELECT ... FOR UPDATE`, before the message row: lock order of the messaging README) and calls
  `DecrementMessageCount`, one `UPDATE threads SET message_count = message_count - 1, version =
  version + 1 WHERE message_count > 0`; no row (an unknown thread or a count of zero) is
  `internal`, so the count never goes below zero. Both fail with `internal` outside a unit of
  work. `message_count` thus counts messages without deleted ones (03 `threads`).

Rulings: docs/DECISIONS.md M1-55. Tests: `domain/thread_test.go`,
`adapters/httpapi/threads_test.go` (PostgreSQL 18, real authz and chat source, response
validation) and `migrations/chat_thread_tags_test.go`.

## Moving threads (M1-62)

- Port: `Threads.Move(ctx, a, MoveThread{ThreadID, TargetChannelID, Reason})` returns the thread
  after the move; handler `MoveThread` in `adapters/httpapi` (`moveThread`, rate-limit class
  `rl.admin`, `Idempotency-Key` through the idempotency stage of the chain). In-transaction event
  `chat.ThreadMoved{ThreadID, FromChannelID, FromSpaceID, ToChannelID, ToSpaceID, ThreadVersion}`;
  internal/wire subscribes `messaging.OnThreadMoved` and then `sync.OnThreadMoved`, which change
  the copies `messages.channel_id`, `read_states.channel_id` and `drafts.channel_id` in the same
  transaction (no version, no event). `Register` adds `thread.moved` and `thread.moved_out`.
- Order in one unit of work: read the thread and authorize `thread.move` with the descriptor
  `{kind: thread, id, TargetChannelID}` (manage on source and target; an invisible or unknown
  target is the same `404` as an unknown thread); lock the thread `FOR UPDATE` (a concurrent move
  committed in between makes it authorize again on the new place); hold the target channel and,
  when it inherits, its space `FOR SHARE` (`queries/threads_move.sql`) and compute the effective
  operator after these locks, so `channel.operator.set` and `space.operator.set` are serialized
  with the check and I1 holds (03 "I1 check"); `domain.CanMove` (`domain/move.go`): main thread
  `409 conflict`, same channel `400` on `/target_channel_id` (`same_channel`), archived target
  `409 conflict`, a dm channel `409 conflict`, another effective operator `409
  operator_mismatch` (a thread without operator moves only into a channel without one); the
  reason (`CheckReason`, SEC-IN-4); then `channel_id`, `moved_at`, `version + 1`, every
  `thread_tags` row deleted, `ThreadMoved`, `thread.moved_out` (source channel as resource without
  a version, it never names the target) and `thread.moved` (the thread with its new version, the
  full `Thread` and `from_channel_id`) in this order, and the audit entry `thread.move` with
  `from_channel_id`, `to_channel_id` and `reason`.
- No permission version and no `access.changed` (05: Permissions `no`). `parent_message_id` stays
  and then points into the source channel (03 `threads`).
- Out of scope: delivery of the two events and the permission cache of the gateway (M1-65),
  `message_attachments.channel_id` (M3-07).

Rulings: docs/DECISIONS.md M1-62. Tests: `domain/move_test.go`, `register_test.go` and the
integration tests `test/integration/move_test.go` (PostgreSQL 18, real modules through
internal/wire, HTTP through the chain with response validation).

## Search language of threads (M3-13)

`Threads.Create` and the main thread of `createChannel` set `threads.search_language` from the
creator through `identity.Directory` (`SearchLanguage` for a person, `InstanceSearchLanguage` for
an agent or app), in the unit of work of the insert ([14-suche.md](../../../../docs/spec/14-suche.md)
3.1, docs/DECISIONS.md D-M3-13-1 and D-M3-13-2). The value goes to SQL as text and is cast there;
`domain.Thread.SearchLanguage` is write-only, and the queries of `threads.sql` and
`threads_main.sql` list their columns, so no read carries `search_language` or `title_search`
(D-M3-13-10). A move (M1-62) keeps the language. `TestThreadInsertsSetSearchLanguage`
(`adapters/postgres`) fails when an `INSERT INTO threads` of `queries/*.sql` does not name
`search_language`. `wire.NewChat` takes the directory in `ChatWrites.Directory`.

## Notification paths (M3-18)

- Port: `Structure` adds `NotificationPath` and `LockNotificationPath` (the path of a notification preference target as `chat.NotificationPath`: kind, space, channel, thread, channel kind, `default_notification_level`, archived; the lock form takes `FOR SHARE` on thread, then channel, then space), `ChannelIDsOfSpace` (archived channels included) and `ThreadIDsOfChannels`, all ascending. None of them authorizes: `notify` authorizes the target with the returned path, and the two id lists run inside an archive that `space.archive` or `channel.archive` allowed. An id of another kind than the requested one is `not_found`.
- Code: `chat/notification.go`, `app/notification.go`, `adapters/postgres/notification.go`; queries `ShareSpace`, `ShareChannel`, `ChannelIDsOfSpace`, `ThreadIDsOfChannels`.

## Channel behaviour and agent configuration (M2-02)

- Port `chat.AgentConfig` (`agentconfig.go`): `GetChannelBehavior`, `PutChannelBehavior`,
  `ListServedChannels`, `GetServedChannelConfig` with `BehaviorSettings`, `ChannelBehavior`,
  `PutChannelBehaviorCmd` and `ServedChannelConfig`. `Register` adds `channel.settings_changed` and
  `agent.config_changed`. No in-transaction event.
- Port `chat.TriggerFacts` (`agentconfig.go`, M2-05): `TriggerChannel` returns kind, space, the
  effective operator after inheritance from the space (`domain.Channel.Operator`) and the five
  trigger switches of a channel for the trigger decision of runs, through `Store.Channel` (query
  `GetChannel`), without authorization; `not_found` for an unknown channel. Since M2-27
  `TriggerChannel.MainThreadID` is the main thread of a text or DM channel, zero for a forum: the
  target of an incoming webhook post, whose webhook actor holds only `messages:write`, so
  `channel.read` and `getChannel` are closed to it (docs/DECISIONS.md D-M2-27-19).
- Domain (`domain/behavior.go`): `Behavior`, `BehaviorLimits` (`channel.prompt_max_bytes`,
  `channel.history_backfill_limit_max` at request time), `ValidateBehavior` (all failures at once:
  `too_small`, `too_large`, `too_long`, `required`, `invalid_characters`, `duplicate`; no detail
  repeats the input), `ChangedFields` and `ConfigChanged`.
- Use cases (`app/behavior.go`) over the outgoing port `app.BehaviorStore`; `settingsChanged` is
  the one function that raises `settings_version` and appends the events (M2-03 calls it for the
  project binding).
- Store: `queries/behavior.sql` (one statement reads switches, prompt, `settings_version` and the
  ordered skills; the list query carries `id = ANY` and `space_id = ANY` from `Scope`); the locks
  are `ShareSpace` and `LockChannel`. `GetChannelBehavior` is an `after_authorize` exception of
  `tools/lint/sql_visibility.yaml`.
- Handlers: `Behavior` in `adapters/httpapi`; `ETag` from `settings_version`.

| Operation | Action | Event |
|---|---|---|
| `getChannelBehavior` | `channel.behavior.read` (`manage`) | |
| `putChannelBehavior` | `channel.behavior.update` (`manage`, step-up, If-Match on `settings_version`) | `channel.settings_changed`, for prompt or skills `agent.config_changed` (self, the effective operator) |
| `listServedChannels` | `agent.config.read` (Scope, then per item in filter mode) | |
| `getServedChannelConfig` | `agent.config.read` (the effective operator) | |

- Order of `putChannelBehavior` in one unit of work: authorize, space `FOR SHARE`, channel `FOR
  UPDATE`, If-Match (`412` with `current`), field rules, `409` for an archived channel, no-op
  (`200`, unchanged ETag, audit `no_change`), write, events, audit. `channels.version` stays, no
  `channel.updated`, no permission change, no `Idempotency-Key`.
- Prompt and skill references never appear in events, logs, traces, metrics, audit details or
  error bodies; the audit entry carries the changed codes, the switches, the backfill and
  `skills_count`.

Rulings: docs/DECISIONS.md D-M2-02-1 to D-M2-02-13. Tests: `domain/behavior_test.go` (with
`FuzzValidateBehavior`), `app/behavior_internal_test.go`, `test/integration/behavior` and the
canary class `manage_only` of `test/canary`.

## Project binding and readiness (M2-03)

- Port `chat.ProjectBindings` (`projectbinding.go`): `PutProjectBinding`, `DeleteProjectBinding`
  and `ApplyProjectReadiness` with `ProjectBinding`, `RunnerConfig`, `PutProjectBindingCmd` and
  `ProjectReadiness`; the caller comes from `kernel/actor` in the context. `ChannelBehavior` and
  `ServedChannelConfig` carry `ProjectBinding` for a bound forum.
- Table `project_bindings` (migration `00050`): PK `channel_id`, unique `lower(workspace_ref)`,
  `runner_config` of schema `project_runner_config.v1`, `ready`, `version`. Only forums get a row;
  the use case enforces it (`400 not_a_forum` on `#path/channel_id`).
- Domain (`domain/project_binding.go`): `ValidateProjectBinding` (limits
  `project.binding_text_max_chars` and `audit.reason_max_chars` at request time) and
  `PlanBindingPut` (`created`, `replaced` with `ready = false`, `unchanged` without a write).
- Use cases (`app/project_binding.go`) over `app.BindingStore`. Order of a PUT: the channel and
  `channel.project.bind` (owner, step-up), `not_a_forum`, the field rules, then in one unit of work
  the space `FOR SHARE`, the channel `FOR UPDATE` (`409` when archived), the binding `FOR UPDATE`,
  the optional `If-Match` (`412`, with `current` when a binding exists), the plan, the write
  (`409` for a taken `workspace_ref`), `settingsChanged` and the audit entry (`channel_id`, `op`,
  `version`, `ready_reset`, `reason`; a DELETE `channel_id` and `op`). DELETE without a binding is
  `404` without an audit entry.
- Readiness: `ApplyProjectReadiness` is the only writer of `ready` besides the resets; it applies
  only reports of the effective operator for a bound forum that is not archived, locks spaces,
  channels and bindings in this order and ascending id, and writes no event, no audit entry and no
  `settings_version`. `SetChannelOperator` and `SetSpaceOperator` reset `ready` of the bound forums
  whose effective operator changed.
- Facts: every channel query `LEFT JOIN`s `project_bindings` for `ChannelFacts.ProjectBound` and
  `ProjectReady` of `authz.StructureSource`; `agent.invoke` gives `project_not_ready` while a bound
  forum is not ready.
- Query exceptions of `tools/lint/sql_visibility.yaml`: `chat.LockProjectBinding`
  (`after_authorize`); the channel and behaviour queries keep their existing entries;
  `LockReportedBindings` carries `channel_id = ANY`.

| Operation | Action | Event |
|---|---|---|
| `putProjectBinding` | `channel.project.bind` (owner, step-up, optional If-Match on the binding version) | `channel.settings_changed`, `agent.config_changed` to the effective operator |
| `deleteProjectBinding` | `channel.project.bind` (owner, step-up) | the same |

Rulings: docs/DECISIONS.md D-M2-03-1 to D-M2-03-12. Tests: `domain/project_binding_test.go`,
`test/integration/project_binding_test.go`, `migrations/chat_project_bindings_test.go` and the
canary scenario `test/canary/scenario_project.go`.

## Port (Wurzelpaket `internal/chat`)

- Eingehend: `Structure` (Bereiche, Kanäle, Schreibschutz), `Threads` (Anlegen für `messaging`, `AppendToThread`, Nachrichtenzahl bei Löschung, Schließen, Öffnen, Verschieben, Info, Locate, UnreadCandidates), `ThreadsByParent` (M2-27), `AgentConfig`, `TriggerFacts`, `ProjectBindings`.
- Gibt In-Tx-Ereignisse aus: `ThreadMoved`, `ChannelArchived`, `ChannelReadOnlySet`, `SpaceArchived`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.StructureSource`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `authz`.

Grund: Bediener ist aktiver Agent-Prinzipal, DM-Teilnehmer sind Menschen; implementiert `StructureSource`.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`spaces`, `user_space_states`, `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`, `threads`, `thread_tags`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.read_only.set`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved`, `thread.moved_out`, `space.created`, `space.updated`, `space.archived`, `channel.created`, `channel.updated`, `channel.settings_changed`, `channel.archived`, `sidebar.updated`, `agent.config_changed`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hinweise für den Bau

- `Threads.AppendToThread` sperrt die Thread-Zeile und vergibt die Thread-Sequenz (03, 6.2); `messaging` ruft ihn in seiner Transaktion.
- Kanal-Prompt, Skills und Projektbindung stehen nie in Ereignissen; `channel.settings_changed` trägt nur `settings_version` (04, 2.2).
- Verschieben eines Threads nur bei gleichem wirksamen Bediener (03, `threads`).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [01-produkt.md](../../../../docs/spec/01-produkt.md), [07-rechte.md](../../../../docs/spec/07-rechte.md), [09-agenten.md](../../../../docs/spec/09-agenten.md)
