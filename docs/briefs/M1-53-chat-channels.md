# M1-53: Channels and forum tags

Status: ready | Depends on: M1-52 | Milestone: M1

## Goal

After this ticket the server manages text and forum channels inside spaces: list, create, read,
rename, reorder and archive them, every text channel with exactly one main thread, and forum tags
with restricted tags. Channels become part of the real `authz.StructureSource`, so `authorize`
resolves channel paths, and a channel created in a visible space is visible and catch-up relevant
at once, without a new permission version. Operators (M1-54), threads (M1-55) and messages (M1-56)
build directly on this.

## Read first

- `docs/spec/03-data-model.md`: "Principles" (all 13 points), "Modules", section "Structure (`chat`)"
  tables `channels`, `channel_skill_bindings`, `forum_tags`, `threads`; "Transaction rules"
  (6.1 global sequence, 6.3 lock order, 6.4 I1); decisions R-DM-10, R-DM-11, R-DM-12, R-DM-21,
  R-DM-25; "Configuration values of this chapter".
- `docs/spec/04-sync-protocol.md`: "Envelope", "Global, commit-ordered sequence",
  "Idempotent writes" (5.2), "Catch-up" 7.2 (fully visible spaces),
  "Visibility on permission changes" 8.1 and 8.2, "Delivery guarantee".
- `docs/spec/05-actions.md`: rules 1.1, 1.2, 1.3, 1.5, 1.7, 1.8; "Target of a resource";
  "Effective level"; 'Columns "Owner", "Step-up", "Audit", "Permissions"'; "Decision"; table
  "Spaces, channels, sidebar" rows `space.read`, `channel.read`, `channel.create`,
  `channel.update`, `channel.reorder`, `channel.archive`, `forum_tag.manage`; section 6 rows
  `channel.created`, `channel.updated`, `channel.archived`, `thread.created`; decisions R-AZ-01,
  R-AZ-06, R-AZ-12.
- `docs/spec/07-permissions.md`: "Targets and inheritance", "The `authorize` function" (4.1 reasons,
  4.2 flow, 4.3 visibility, 4.4 invariants), "Filtering in SQL" (7.2 snapshot, 7.3 cache
  validity, 7.4 query pattern), "Immediate effect on revocation", "Required tests" AZ-P1, AZ-P4,
  AZ-P8.
- `docs/spec/02-architecture.md`: "Modules of the monolith" 4.1 to 4.6, "Layers within a module",
  "Code layout", "Transactions and Unit of Work", "Request path", "Permission cache" (11.3),
  "Alignment with the core contract" row "catch-up with fully visible spaces; the cache follows
  structure events".
- `docs/spec/06-api.md`: "Basic rules", "Authorization in the API", "Errors", "Pagination"
  point 6, "Idempotency", "Concurrency", endpoint table "Channels".
- `docs/spec/19-security.md`: SEC-IN-1, SEC-IN-4, SEC-T07. `docs/spec/01-product.md` 3.1 and
  3.2 row "Operator per channel visible in the channel header".
- `docs/api/openapi.yaml`, operationIds: `listSpaceChannels`, `createChannel`, `getChannel`,
  `updateChannel`, `moveChannel`, `archiveChannel`, `listForumTags`, `createForumTag`,
  `updateForumTag`, `moveForumTag`, `deleteForumTag`. Schemas: `Channel`, `ChannelView`, `ChannelViewList`,
  `ChannelCreate`, `ChannelUpdate`, `ChannelOperator`, `MovePosition`, `ReasonBody`, `ForumTag`,
  `ForumTagList`, `ForumTagCreate`, `ForumTagUpdate`, `ForumSort`, `ChannelCreatedEvent`,
  `ChannelUpdatedEvent`, `ChannelArchivedEvent`, `ThreadCreatedEvent`, `Problem`.
- `docs/api/asyncapi.yaml`: messages `event.channel.created`, `event.channel.updated`,
  `event.channel.archived`, `event.thread.created`.
- ADRs: 0008, 0009, 0013, 0015, 0017, 0018; 0050, 0052, 0053 (written by M1-05 and M1-16).
- Code of merged tickets you build on: `services/server/internal/chat` (M1-52: spaces, sidebar,
  fractional index helper, palette validator, the adapter implementing `authz.StructureSource`),
  `internal/authz` (M1-27 to M1-29: snapshot cache, `authorize`, `Scope`, `RightsChange` phase
  hook), `internal/kernel/{txn,eventlog,auditing,authorize,actor,ids,errs}`,
  `internal/platform/httpserver`, `services/server/api/operations.yaml`, the display-name validator
  of SEC-T07 (M1-26), the response validator used in integration tests.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/PLAN.md` sections "Way of working per ticket" and
  "M1 Foundation".

## Fixed requirements that apply

Owner build rules (verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Note on rule 3: in this repository `docs/api/openapi.yaml` is the source and the server interface
is generated from it (ADR 0013); the drift check `make codegen-check` keeps routes and document
identical, which is how the rule is met.

Rules for every brief:

6. Test first: every behaviour change starts with a failing test (red), then code (green), then
   cleanup. Keep the red output for the report.
7. No dashes U+2013 or U+2014 in any file you write; use a hyphen or rephrase.
8. White label: no company, person, host or domain names except the examples of the
   specification (example.com, Alex, Sam, Robin, the example spaces and agents of 07 "Examples").
9. Limits and windows only through named configuration keys from
   `docs/operations/configuration.md`; never a literal number in code or docs that a key covers.

Product requirements implemented here:

10. Channel kinds `text` and `forum`, each in exactly one space; `dm` cannot be created through
    these operations (03 `channels`, R-DM-10).
11. Every text channel has exactly one thread with `kind = 'main'`, created in the same
    transaction; forum channels have none (R-DM-11).
12. Channel and tag order is a fractional index; moving changes exactly one row (03 "Principles"
    point 5).
13. Channel names are unique per space, case-insensitive, among non-archived channels; tag names
    unique per channel among non-deleted tags (03 unique indexes). Channel names at most
    `channel.name_max_chars`, topics at most `channel.topic_max_chars`, tag names at most
    `forum_tag.name_max_chars` (03 9). Display texts
    with control or bidi control characters are rejected (SEC-IN-4, SEC-T07).
14. No read permission gives `404`, identical to an unknown ID; lists filter in SQL and then run
    `authorize` per item (05 rule 1.2, 07 7.4).
15. `channel.archive`: permission `manage`, step-up, audit, and a permission change with
    `access.changed` (05 table, rule 1.7).
16. A new channel in a visible space is visible and catch-up relevant without a new permission
    version (02 "Alignment with the core contract", 04 7.2 and 8.1).
17. Forum tags are managed with `manage`; tags may be `restricted` (03 `forum_tags`, 05
    `forum_tag.manage`).
18. Every durable event uses the common envelope without JSON `null` (04 2.2, ADR 0053).

## Scope

Create or change exactly these (paths relative to `services/server/` unless they start with `docs/`):

1. `migrations/<next number>_chat_channels.sql` (goose, same numbering format as the existing
   files): tables `channels`, `channel_skill_bindings`, `forum_tags`, `threads` exactly as 03
   defines them, with these deliberate deviations, each commented in the file:
    - `threads.search_language`, `threads.title_search` and its GIN index are not created here;
      M3-13 adds them (plan row M3-13).
    - The foreign key `threads.parent_message_id -> messages(id)` is not created here because
      `messages` does not exist yet; M1-56 adds it as `DEFERRABLE INITIALLY DEFERRED`.
    - All other columns, defaults, CHECKs, unique and partial indexes of the four tables are
      created as written in 03, including `channels.read_only`, the behaviour columns and
      `settings_version` (their operations come in M2-02 and M8-03; the columns exist now so no
      later migration rewrites the table).
    - Privileges for the application role follow the pattern of the existing migrations
      (`grep -n GRANT migrations/*.sql`).
    - Add the foreign key `grants.target_channel_id -> channels(id) ON DELETE RESTRICT` (03
      `grants`), which M1-27 could not create before `channels` existed.
2. `internal/chat` (port): extend the `Structure` port with the channel and forum tag use cases
   below and their DTOs; add the in-transaction event type `ChannelArchived{ChannelID, SpaceID,
   ArchivedAt}` (02 4.3). No subscriber is registered in this ticket.
3. `internal/chat/domain`: `Channel`, `ChannelKind`, `OperatorMode` (read side only: `inherit`,
   `none`, `agent`), name and topic validation, `ForumTag`, the archive transition, and the
   `MovePosition` rule.
4. `internal/chat/app`: use cases `ListSpaceChannels`, `CreateChannel`, `GetChannel`,
   `UpdateChannel`, `MoveChannel`, `ArchiveChannel`, `ListForumTags`, `CreateForumTag`,
   `UpdateForumTag`, `MoveForumTag`, `DeleteForumTag`.
5. `internal/chat/adapters/postgres`: `queries/channels.sql`, `queries/forum_tags.sql`,
   `queries/threads_main.sql` (insert of the main thread only), sqlc output, repository.
6. `internal/chat/adapters/httpapi`: handlers for the eleven operations above.
7. The chat adapter that implements `authz.StructureSource` (find it with
   `grep -rn "StructureSource" internal/chat`): the channel part, see "Design notes" D7.
8. `internal/authz` and `internal/kernel/authorize`, only as far as "Design notes" D8 and D9
   require (archived cap, effective permissions for `ChannelView`), each with tests. Permissions of
   channels created after a snapshot and the list scope fields are built by M1-28 (07 7.2,
   ruling D5 there); this ticket only adds the integration test against real channels. The
   target-based `RightsChange` comes from M1-29.
9. `api/operations.yaml`: enable the eleven operations; regenerate `api/rest` and `api/routes` with
   the generator target documented in `services/server/api/README.md` (M1-17).
10. `internal/platform/config` schema: register `channel.name_max_chars`,
    `channel.topic_max_chars` and `forum_tag.name_max_chars` (positive integers, defaults from 03
    section 9) in the typed `chat` config, the same way M1-52 registers `space.name_max_chars`;
    add `channel.history_backfill_limit_default` and `channel.history_backfill_limit_max` with the
    defaults of 03 if they are not there yet.
11. `internal/wire`: register handlers and the new in-transaction event type; update the golden
    file of subscriptions.
12. Documentation: `internal/chat/README.md` (status, ports, tables, actions, events),
    `services/server/CHANGELOG.md` (Unreleased), `docs/DECISIONS.md` (rulings D1, D3, D6, D8
    below); the description of `archiveChannel` in `docs/api/openapi.yaml` already states the rule
    of 07 "4.5".
    07 "4.1", "4.5" and 06 (problem code `channel_read_only` with `reason: channel_archived`)
    already state D2; if the M1-03 generator owns the 06 problem block, keep its source in step
    (D10).

Public interface (pseudo-Go; follow the naming and error conventions already used in
`internal/chat` for spaces):

```go
// Structure (excerpt added by this ticket). All methods run inside an ambient unit of work
// when one exists, else they open their own (kernel/txn).
ListSpaceChannels(ctx, spaceID uuid.UUID) ([]ChannelView, error)
CreateChannel(ctx, CreateChannelCmd) (Channel, error)            // kind, name, topic, forum_default_sort, after_channel_id
GetChannel(ctx, channelID uuid.UUID) (ChannelView, error)
UpdateChannel(ctx, UpdateChannelCmd) (Channel, error)            // expected version from If-Match
MoveChannel(ctx, MoveChannelCmd) (Channel, error)                // after_id | before_id | none
ArchiveChannel(ctx, ArchiveChannelCmd) (Channel, error)          // reason
ListForumTags(ctx, channelID uuid.UUID) ([]ForumTag, error)
CreateForumTag(ctx, CreateForumTagCmd) (ForumTag, error)
UpdateForumTag(ctx, UpdateForumTagCmd) (ForumTag, error)
MoveForumTag(ctx, MoveForumTagCmd) (ForumTag, error)               // channel id, tag id, MovePosition
DeleteForumTag(ctx, channelID, tagID uuid.UUID) error
```

The actor comes from `kernel/actor` in the context, never from a parameter.

## Out of scope

- Setting operators (`setSpaceOperator`, `setChannelOperator`), the operator rule and the I1 served
  targets: M1-54. This ticket only reads `operator_inherit`, `operator_agent_id` and the space
  default to fill `Channel.operator`.
- Thread operations, `thread_tags`, thread paths in `StructureSource`: M1-55. Removing `thread_tags`
  rows when a tag is deleted is added by M1-55.
- Messages and `createThread`: M1-56. Moving threads: M1-62.
- Channel behaviour (`getChannelBehavior`, `putChannelBehavior`), skills, `channel.settings_changed`:
  M2-02. Project binding: M2-03. Setting `read_only` (`setChannelReadOnly`): M8-03; the column
  exists with default `false` and nothing in M1 sets it.
- Direct messages and `dm_participants`: V3.
- Search columns and indexes: M3-13. Notification preference cleanup on archive: M3 (`notify`
  subscribes to `ChannelArchived` later).
- `kernel/personaldata.Participant` for chat tables: M7 (P-14).
- Applying structure events to the gateway cache (`Authorizer.Observe`): M1-65. REST catch-up and
  snapshot: M1-63.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-53-chat-channels`, created from the base
  branch the launcher names. One writer per worktree.
- Conventional Commits with a module scope (for example `feat(chat): channels and forum tags`),
  every commit with DCO sign-off (`git commit -s`). Squash to meaningful commits before the merge
  (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the module README named above.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0. PostgreSQL 18 runs as a throwaway container through
  `internal/platform/pg/pgtest` on a random port; a missing container runtime fails the test, it
  never skips (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end by their literal path.
- Never read or print secrets or `.env` files.
- No network calls except package downloads for pinned tools.
- Do not lower a threshold, disable a gate, skip or delete a test.

## Design notes

D0. Start check. Run `make check` on the fresh worktree before changing anything. If it is red,
stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that M1-52 is merged
(`internal/chat` has spaces and a `StructureSource` adapter); otherwise stop with
`Verdict: BLOCKED dependency M1-52 missing`.

D1. Channels.

- `createChannel`: action `channel.create` on the space (permission `manage`, audit). Body
  `ChannelCreate`. `kind` is `text` or `forum`. `name`: trim leading and trailing Unicode white
  space, then at least one character; reject C0 and C1 control characters and bidi controls
  (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069) with `400 validation_failed`,
  field error `/name` code `invalid_characters`; reuse the shared validator `kernel/textcheck`
  (M1-26) instead of writing a second one. The same check applies to `topic`, which is at most
  `channel.topic_max_chars` characters (`/topic`, `too_long`). `forum_default_sort` only
  for `forum`, else `400` (`/forum_default_sort`, `not_allowed_for_kind`). `after_channel_id`
  must be a non-archived channel of the same space, else `400` (`/after_channel_id`,
  `not_a_sibling`); absent means at the end. Defaults from 03; `history_backfill_limit` is set to
  `channel.history_backfill_limit_default`; `operator_inherit = true`. A duplicate name gives
  `409 conflict`. An archived space gives `409 conflict`.
- For `kind = 'text'` the same transaction inserts the main thread: `kind = 'main'`,
  `title = ''`, `title_source = 'auto'`, `state = 'open'`, `operator_agent_id` NULL,
  `parent_message_id` NULL, `created_by` = actor.
- Events in this order: `channel.created` (scope `{space_id, channel_id}`, resource
  `{channel, id, version}`, payload `{channel}`), then for text channels `thread.created` (scope
  with `thread_id`, payload `{thread}` in the `Thread` schema with `tag_ids: []`). Audit allow
  entry with details `{space_id, channel_id, kind, name}`.
- `Channel` representation: `operator = {mode, agent_id?, effective_agent_id?}` where mode is
  `inherit` when `operator_inherit`, else `agent` when `operator_agent_id` is set, else `none`;
  `effective_agent_id` = space default when inheriting, else `operator_agent_id`; absent fields
  are omitted, never `null`. `main_thread_id` for text channels. `read_only`, `archived_at`,
  `version`, `settings_version`, `created_at`. Never the prompt or behaviour fields.
- `getChannel`: action `channel.read`; `ETag: "<version>"`; returns `ChannelView` with the
  caller's effective `level` and `capabilities` on the channel (D9). Archived channels are
  returned with `archived_at`.
- `listSpaceChannels`: first `authorize(space.read, space)`, invisible space gives `404`; then
  `Scope(channel.read)`; SQL filter `space_id = $space AND archived_at IS NULL AND (id = ANY($visible)
  OR space_id = ANY($fully_visible_spaces) OR $instance_wide)`, ordered by `sort_key`; then
  `authorize(channel.read)` per item in filter mode `authorize.Filter(ctx)` (M1-28; no denial audit, 07 4.2), an item that fails is dropped and counted as an
  alarm (07 7.4 rule 3). Unpaginated (06 "Pagination" point 6). Archived channels are not listed.
- `updateChannel`: action `channel.update` (`manage`, audit), `If-Match` required (`428`
  without, `412 version_conflict` with `current` on mismatch, through the helpers of M1-18). At
  least one field, else `400`. `default_notification_level` and
  `clear_default_notification_level: true` together give `400`. Unchanged values give `200` with
  the current channel, no version bump, no event. A change bumps `version` and emits
  `channel.updated`. Audit details: changed field names, old and new `name`; never the topic text.
- `moveChannel`: action `channel.reorder` on the space (`manage`, audit). `MovePosition`: both
  `after_id` and `before_id` give `400`; a neighbour that is not a non-archived channel of the same
  space gives `400`; none moves to the start. Compute the key with the fractional index helper of
  M1-52; exactly one row changes. Bumps `version`, emits `channel.updated`.

D2. Archived channels (07 "4.5 Archived spaces and channels", reason `channel_archived` in 07
"4.1"):

1. `archiveChannel` (action `channel.archive`: `manage`, step-up, audit, permissions) needs
   `ReasonBody`; it sets `archived_at`, bumps `version`, emits `channel.archived` (payload
   `{channel}`) and publishes `ChannelArchived` in the transaction. It is irreversible; 05 has no
   unarchive action. Archiving an archived channel returns `200` with the current channel and
   writes nothing.
2. On an archived channel, and on every channel of an archived space, the effective level of
   every principal, owners included, is at most `read`. `authorize` denies every action whose
   "Permission" column needs `write` or `trigger` on a target in that channel with decision
   `forbidden`, reason `channel_archived`. The HTTP answer is `403` with problem code
   `channel_read_only` and `reason: channel_archived` (same client reaction as rule 1.8, no new
   problem code). Reading stays allowed for everyone who could read before.
3. Actions with permission `manage` or `owner` pass `authorize`, but chat rejects every state
   change of an archived channel or its tags (`updateChannel`, `moveChannel`, forum tag create,
   update, move, delete) with `409 conflict`.
4. Permission change: the unit of work runs with the `RightsChange` option. Affected principals
   are all principals whose effective level on the channel was above `read` before the change
   (direct grants, group members, owners, the operator rule). `authz` writes `access.changed`
   for each with a `changed` entry `{kind: channel, id, level: read, previous_level}` and bumps
   `permission_version` (04 "8.1" and 02 "11.3": archiving is a structure change and a
   permission change).
5. `listSpaceChannels` omits archived channels; `getChannel` still returns them.

D3. Forum tags.

- `listForumTags`: action `channel.read`; non-deleted tags ordered by `sort_key`.
- `createForumTag`, `updateForumTag`, `deleteForumTag`: action `forum_tag.manage` (`manage`,
  audit). The channel must be a forum, else `400` (`/`, `not_a_forum`). Name rules as D1; unique
  case-insensitive among non-deleted tags of the channel, else `409 conflict`. `color` must be a
  category colour key of the design system (reuse the palette validator of M1-52; a hex value or
  unknown key gives `400`). `restricted` defaults to `false`. `after_tag_id` must be a non-deleted
  tag of the same channel, else `400`; absent means at the end.
- `updateForumTag`: at least one field, else `400`; `clear_color` removes the colour and together
  with `color` gives `400` (`/clear_color`, `not_allowed_with_color`); no change gives `200`
  without event.
- `moveForumTag`: action `forum_tag.manage`, body `MovePosition` with the rules of `moveChannel`
  among the non-deleted tags of the channel; sets `sort_key`.
- `deleteForumTag`: sets `deleted_at`; deleting a deleted tag gives `204`; a tag ID that never
  belonged to this visible channel gives `404`.
- Events (03 `forum_tags`, 05 section 6 row `channel.updated`): every tag create, update, move or delete
  bumps `channels.version` and emits `channel.updated`; clients reload `listForumTags` when the
  channel version changes. Audit details `{op, tag_id, name}`.

D4. Idempotency and concurrency. POST and PATCH operations listed above accept `Idempotency-Key`
(M1-20 middleware, nothing to build). Lock order inside a transaction follows 03 6.3: the
`permission_version` counter first (only `archiveChannel`), then domain rows in ascending `id`.

D5. Events go through `kernel/eventlog.Appender`; scope fields that do not apply are omitted;
`resource.version` is the version after the change; no JSON `null` anywhere (checked in
`TestCreateChannel_EventsAuditAndEnvelope`).

D6. Main thread creation without a thread module: the main-thread insert lives in the chat
repository (`queries/threads_main.sql`) because chat owns `threads`; M1-55 builds the thread use
cases on the same table. Record that this ticket creates `threads` although the plan row of M1-55
names it, because R-DM-11 needs the main thread at channel creation.

D7. `authz.StructureSource`, channel part, in the chat adapter (keep the interface names M1-28
defined; if a capability is missing, add it to the interface in `internal/authz` and to its fakes):
resolve a channel ID to `{space_id, kind, archived (own or space), read_only}`; enumerate the
non-archived channel IDs of a space and of the instance where `authz` needs them. Lookups run in
the ambient transaction when one exists, so rows written earlier in the same unit of work are seen.

D8. Permissions of channels created after a snapshot (requirement 16). M1-28 builds this (07 7.2,
ruling D5 of M1-28): `authorize` derives a channel's level at decision time from the freshly
resolved path, the snapshot caches grant data per target only, and `Scope` returns
`visible_channel_ids`, `fully_visible_space_ids` and `instance_wide`, as 04 7.2 needs; AZ-P8 in
M1-28 already covers channels created after the snapshot. This ticket does not change that core;
it adds the integration test below against the real chat `StructureSource`. If the test fails,
stop with `Verdict: BLOCKED M1-28 does not follow 07 7.2` instead of changing the cache. The
archived cap of D2 point 2 is implemented in `authorize`'s path evaluation, with
reason `channel_archived` added to the closed reason list that `audit_events.deny_reason` is tested
against. The archive runs with the target-based `RightsChange` of M1-29 (02 8.1), which computes the
affected principals of D2 point 4 inside `authz`.

D9. `ChannelView.level` and `capabilities` come from an `authz` port method that returns the
caller's effective level and capabilities on a channel (owner: `trigger` and all capabilities;
archived: `read`). If `kernel/authorize` has none, add `EffectiveRights(ctx, principal,
channelRef) (Level, []Capability, error)` with a table test.

D10. Problem mapping: unknown or invisible space or channel gives `404 not_found`; visible without
the permission gives `403 forbidden` (`level_insufficient`, `capability_missing`, `owner_only`);
missing fresh step-up gives `401 step_up_required` (agent, app and personal tokens get `403`,
`step_up_not_possible`). The problem catalogue is generated; change its source (find it with
`grep -rn "operator_mismatch" tools docs/api services/server`), regenerate with
`make api-checks`, never edit inside `BEGIN GENERATED` blocks by hand.

## Tests (write them first)

Go test names carry the specification ID where one exists, so `go test -run AZ_P8` finds it.

Unit (`internal/chat/domain`, table tests):

- `TestChannelName_TrimAndEmpty`: surrounding white space trimmed; only white space rejected.
- `TestSEC_T07_ChannelAndTagNamesRejectControlAndBidi`: every listed code point rejected in name,
  topic and tag name.
- `TestCreateChannel_ForumSortOnlyForForum`.
- `TestChannelAndTagTextLimits`: a name over `channel.name_max_chars`, a topic over
  `channel.topic_max_chars` and a tag name over `forum_tag.name_max_chars` each give
  `400 validation_failed` with field error code `too_long` on `/name` or `/topic`; a value of
  exactly the limit is accepted (limits read from the config, not hard-coded in the test).
- `TestMovePosition_ExactlyOneNeighbour`: both set rejected, none means start.
- `TestChannelOperatorRepresentation`: inherit, none, agent and the effective agent; no empty
  fields emitted.

Integration (PostgreSQL 18 via `pgtest`, real `authz`, `audit`, `sync` modules; HTTP through the
test server and test authenticator of the earlier tickets; every response checked by the response
validator):

- `TestCreateChannel_TextHasExactlyOneMainThread` (R-DM-11): one `main` row; a second direct
  insert of a main thread for the channel violates the unique index.
- `TestCreateChannel_ForumHasNoMainThread`.
- `TestCreateChannel_Rights`: `manage` on the space gives `201`; `write` without `manage` gives
  `403`; no read on the space gives `404` identical to an unknown space ID.
- `TestCreateChannel_DuplicateNameCaseInsensitive409`, and the same name is allowed again after
  the first channel is archived.
- `TestCreateChannel_ArchivedSpace409`.
- `TestCreateChannel_EventsAuditAndEnvelope`: `channel.created` then `thread.created` in `seq`
  order, scope and resource versions correct, `receive_action` values from 05 section 6, one audit
  allow entry, no JSON `null` in the stored payloads.
- `TestNewChannelVisibleWithoutNewPermissionVersion`: Alex has `read` on space Support; Alex calls
  `getSpace` to warm the snapshot; the owner creates a channel in Support; `permission_version`
  is unchanged; Alex's `getChannel` gives `200` and `listSpaceChannels` lists it; the
  `channel.created` row in `event_log` carries `space_id` and `channel_id`; `Scope` for Alex lists
  Support in `fully_visible_space_ids`.
- `TestListSpaceChannels_FiltersInvisibleAndArchived`: channel grant only on one channel of the
  space lists only that channel; archived channels absent; level and capabilities per item.
- `TestGetChannel_ForeignAndUnknownIdentical404` (AZ-P4 style): status, headers and body equal
  after removing `correlation_id` and `instance`.
- `TestUpdateChannel_IfMatch`: `428`, `412` with `current`, `200` with version plus one and
  `channel.updated`; no-op gives no event; clearing the notification default works.
- `TestMoveChannel_ChangesExactlyOneRow`: row versions of all other channels unchanged.
- `TestArchiveChannel_StepUp`: stale step-up gives `401 step_up_required`; fresh gives `200`.
- `TestArchiveChannel_RightsChange`: `permission_version` plus one; `access.changed` for Alex
  (`trigger` to `read`) and the owner; none for a principal that only had `read`.
- `TestArchiveChannel_WritesDeniedForEveryone`: after archiving, `authorize(thread.create)` for the
  owner and for Alex gives `forbidden` with reason `channel_archived`; `channel.read` still allowed;
  the audit deny entry carries `channel_archived`.
- `TestArchiveChannel_Idempotent` and `TestArchivedChannel_Mutations409` (update, move, tag create).
- `TestForumTags_CRUD`: permissions (`manage` needed, read-only member gets `403`, outsider `404`),
  `restricted`, palette colour accepted, hex colour `400`, duplicate name `409`, text channel
  `400 validation_failed` with `errors[].code` `not_a_forum`, delete twice `204`, `clear_color` removes the colour, `moveForumTag` changes
  the order, each change emits `channel.updated` with a higher channel version.
- `TestAZ_P1_RouteCount`: the existing route count test passes with the eleven new operations.

Commands that must pass (from the repository root unless noted):

```
cd services/server && go test -race ./internal/chat/... ./internal/authz/... ./internal/kernel/...
make codegen-check
make arch
make api-lint
make api-checks
make register-check
make config-doc-check
make migrations-lint
make docs
make check
```

## Acceptance criteria

- [ ] The migration creates the four tables as in 03 with exactly the two documented deviations;
      `make migrations-lint` is green.
- [ ] The eleven operations are enabled in `api/operations.yaml`, generated code has no drift, and
      every registered route has a known action (AZ-P1).
- [ ] Every text channel has exactly one main thread; forum channels none.
- [ ] A channel created in a visible space is visible over REST without a new permission version,
      and its event rows carry `space_id`.
- [ ] Invisible and unknown IDs give identical `404` answers.
- [ ] `archiveChannel` needs fresh step-up, bumps `permission_version`, writes `access.changed`,
      and afterwards no principal, owner included, can write or trigger in the channel
      (`403 channel_read_only`, reason `channel_archived`), while reading stays possible.
- [ ] Forum tags follow D3, including the `channel.updated` ruling.
- [ ] Names with control or bidi characters are rejected (SEC-T07 part channels and tags).
- [ ] 07 has the subsection "Archived spaces and channels" and the reason row; 06 and openapi.yaml
      describe the archived case; `docs/DECISIONS.md` has rulings D1 (validation), D2, D3 (tag
      event), D6, D8.
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation in the same change: `internal/chat/README.md` current (status,
  ports, tables, actions, events, in-transaction events), a doc comment on every public symbol
  (purpose, parameters, return value, errors, example where useful), comments explain why.
- No new architecture decision beyond the rulings above; if you take one, write an ADR (MADR 4,
  next free number, headings and text in English) and list it in the report.
- `services/server/CHANGELOG.md` has an entry under "Unreleased".
- `make check` green on the final commit.
- Local review: `make review` and a code review over the whole diff; every finding fixed with a
  test.
- Red team: a fresh, read-only reviewer runs `docs/quality/red-team-gate.md` over
  `git diff <base>...HEAD`; record its last line (`Urteil: JA` or `Urteil: NEIN <reason>`). On
  `NEIN` fix, add a test per finding and have the delta checked again.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove` the worktree.

## Report

Write `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), files changed with line counts.
3. Test results with counts (passed, failed, skipped must be 0), the red output of the first
   failing tests, coverage of `internal/chat`.
4. Review and red team verdicts.
5. What is not proven (for example cache behaviour across processes, which M1-65 tests).
6. Only real owner decisions, each with a recommendation. A finding in your own diff is fixed,
   not reported.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
