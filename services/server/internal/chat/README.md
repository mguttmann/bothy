# Module `chat`

Status: spaces and the sidebar (M1-52), channels and forum tags (M1-53), operators with I1 (M1-54) and threads (M1-55) built; `createThread` is built in `messaging` (M1-56).
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Spaces, sidebar state, channels with behavior, skills, project binding, DM participants, forum tags, threads with thread sequence.

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
- Sidebar (04 section 14): agents and apps get `403 forbidden` (reason `subject_kind_not_allowed`); `collapsed` is
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
- Migrations `00025_chat_channels.sql` (`channels`, `channel_skill_bindings`, `forum_tags`,
  `threads` as 03 section 4.5, without `threads.search_language`, `threads.title_search` and its
  index (M3-13) and without the foreign key `threads.parent_message_id -> messages` (M1-56); and
  `grants.target_channel_id -> channels(id)` `NOT VALID`) and `00026_grants_channel_fk_validate.sql`.
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

## Threads (M1-55)

- Port `chat.Threads` (`threads.go`): `Create`, `AppendToThread`, `Info` and `View` for
  `messaging` (M1-56 sends messages and enables `createThread` through them; `View` is the
  thread of its `createThread` answer), and `List`, `Get`, `SetTitle`, `Close`,
  `Reopen` and `SetTags` for the six handlers; the DTOs `Thread`, `ThreadInfo`, `ParentMessage`,
  `CreateThreadCmd`, `AppendCmd`, `Appended`, `ListThreadsQuery`, `ThreadPage`, `SetTitleCmd` and
  `SetTagsCmd`. `Register` adds `thread.updated`, `thread.closed` and `thread.reopened`.
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
- Migration `00027_chat_thread_tags.sql`: `thread_tags` as 03; `bothy_app` may read, insert and
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

Rulings: docs/DECISIONS.md M1-55. Tests: `domain/thread_test.go`,
`adapters/httpapi/threads_test.go` (PostgreSQL 18, real authz and chat source, response
validation) and `migrations/chat_thread_tags_test.go`.

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
