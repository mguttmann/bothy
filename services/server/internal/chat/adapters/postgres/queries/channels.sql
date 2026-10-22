-- SPDX-License-Identifier: Apache-2.0
-- Queries on channels (chat module, M1-53; docs/spec/03-data-model.md section 4.5). Only this
-- module reads and writes the table (03 principle 12). A channel row is read together with the
-- default operator and the archive state of its space and the id of its main thread, because
-- the representation (operator, main_thread_id) and authorize (archived) need them. Orders are
-- sort_key (COLLATE "C" from the column), then id.

-- name: GetChannel :one
SELECT sqlc.embed(c), s.default_operator_agent_id AS space_operator_id,
       (s.archived_at IS NOT NULL)::boolean AS space_archived, t.id AS main_thread_id,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, coalesce(pb.ready, false)::boolean AS project_ready
FROM channels c
LEFT JOIN spaces s ON s.id = c.space_id
LEFT JOIN threads t ON t.channel_id = c.id AND t.kind = 'main'
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.id = @id;

-- name: LockChannel :one
-- The channel locked for a change (03 section 6.3: domain rows after permission_version).
SELECT sqlc.embed(c), s.default_operator_agent_id AS space_operator_id,
       (s.archived_at IS NOT NULL)::boolean AS space_archived, t.id AS main_thread_id,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, coalesce(pb.ready, false)::boolean AS project_ready
FROM channels c
LEFT JOIN spaces s ON s.id = c.space_id
LEFT JOIN threads t ON t.channel_id = c.id AND t.kind = 'main'
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.id = @id
FOR UPDATE OF c;

-- name: ListVisibleChannels :many
-- listSpaceChannels with the predicate of 07 section 7.4 and 04 section 7.2: the channels of the
-- space that are not archived, every one for an instance-wide reader or a fully visible space,
-- else the visible ones of Scope; authorize then checks each row.
SELECT sqlc.embed(c), s.default_operator_agent_id AS space_operator_id,
       (s.archived_at IS NOT NULL)::boolean AS space_archived, t.id AS main_thread_id,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, coalesce(pb.ready, false)::boolean AS project_ready
FROM channels c
LEFT JOIN spaces s ON s.id = c.space_id
LEFT JOIN threads t ON t.channel_id = c.id AND t.kind = 'main'
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.space_id = @space_id AND c.archived_at IS NULL
  AND (c.id = ANY (@visible::uuid[]) OR c.space_id = ANY (@fully_visible::uuid[]) OR @instance_wide::boolean)
ORDER BY c.sort_key, c.id;

-- name: ListAllChannels :many
-- Every channel that is not a dm channel, archived ones included, ascending by id, for the
-- structure source of authz.
SELECT sqlc.embed(c), s.default_operator_agent_id AS space_operator_id,
       (s.archived_at IS NOT NULL)::boolean AS space_archived, t.id AS main_thread_id,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, coalesce(pb.ready, false)::boolean AS project_ready
FROM channels c
LEFT JOIN spaces s ON s.id = c.space_id
LEFT JOIN threads t ON t.channel_id = c.id AND t.kind = 'main'
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.kind <> 'dm'
ORDER BY c.id;

-- name: ActiveChannelKeys :many
-- The order of the channels of a space that are not archived, the neighbours of create and move.
SELECT id, sort_key FROM channels WHERE space_id = @space_id AND archived_at IS NULL ORDER BY sort_key, id;

-- name: InsertChannel :exec
INSERT INTO channels (id, space_id, kind, name, topic, sort_key, operator_inherit, require_mention, thread_require_mention,
    free_response, auto_thread, allow_mentioned_agents, history_backfill_limit, forum_default_sort, read_only, created_by,
    created_at, version, settings_version)
VALUES (@id, @space_id, @kind, @name, @topic, @sort_key, @operator_inherit, @require_mention, @thread_require_mention,
    @free_response, @auto_thread, @allow_mentioned_agents, @history_backfill_limit, sqlc.narg(forum_default_sort), @read_only,
    @created_by, @created_at, @version, @settings_version);

-- name: SaveChannel :exec
-- Writes the columns of channel.update, channel.reorder and channel.archive and the version; a
-- move or a tag change writes exactly this one row.
UPDATE channels
SET name = @name, topic = @topic, forum_default_sort = sqlc.narg(forum_default_sort),
    default_notification_level = sqlc.narg(default_notification_level), sort_key = @sort_key,
    archived_at = sqlc.narg(archived_at), version = @version
WHERE id = @id;

-- name: ShareChannel :one
-- The channel held FOR SHARE while notify writes a preference on it or below it (M3-18); the
-- columns of GetChannel. The space row is not locked here: the caller shares it next.
SELECT sqlc.embed(c), s.default_operator_agent_id AS space_operator_id,
       (s.archived_at IS NOT NULL)::boolean AS space_archived, t.id AS main_thread_id,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, coalesce(pb.ready, false)::boolean AS project_ready
FROM channels c
LEFT JOIN spaces s ON s.id = c.space_id
LEFT JOIN threads t ON t.channel_id = c.id AND t.kind = 'main'
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.id = @id
FOR SHARE OF c;

-- name: ChannelIDsOfSpace :many
-- Every channel of a space, archived ones included, ascending: the targets of the cleanup of
-- notification preferences when the space is archived (M3-18).
SELECT id FROM channels WHERE space_id = @space_id ORDER BY id;
