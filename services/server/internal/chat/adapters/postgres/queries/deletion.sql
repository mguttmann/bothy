-- SPDX-License-Identifier: Apache-2.0
-- Deleting spaces, channels and chats (FEAT-delete-spaces-chats; docs/design/delete.md,
-- docs/spec/03-data-model.md R-DM-57). The writes of space.delete, channel.delete, thread.delete
-- and their restores are in spaces.sql, channels.sql and threads.sql; here are the counts of
-- DeletionInfo, the reads and writes of the job messaging.purge_deleted and the folder requests of
-- project_bindings (09 section 18.15). None of these reads authorizes: the use case authorized
-- the item first, and the job and the heartbeat have no person (tools/lint/sql_visibility.yaml).

-- name: ScopeChannelsOfSpace :many
-- The channels a space deletion removes: the channels of the space that are not dm channels and
-- not deleted on their own.
SELECT id FROM channels WHERE space_id = @space_id AND kind <> 'dm' AND deleted_at IS NULL ORDER BY id;

-- name: ScopeThreads :many
-- The threads of the channels that are not deleted on their own, main threads included, with
-- their kind and message count (03 threads: message_count counts the messages that are not
-- deleted).
SELECT id, kind, message_count FROM threads WHERE channel_id = ANY (@channel_ids::uuid[]) AND deleted_at IS NULL ORDER BY id;

-- name: BindingTrash :one
SELECT workspace_ref, trash_check_ok, trash_check_reason, trash_checked_at, trash_state, trash_reason, trash_agent_id
FROM project_bindings WHERE channel_id = @channel_id;

-- name: LockBindingTrash :one
SELECT workspace_ref, trash_check_ok, trash_check_reason, trash_checked_at, trash_state, trash_reason, trash_agent_id
FROM project_bindings WHERE channel_id = @channel_id FOR UPDATE;

-- name: RequestTrash :exec
-- The folder move of a deleted forum, handed to agent once the deletion is final (D-DEL-11).
UPDATE project_bindings
SET trash_state = 'requested', trash_agent_id = @agent_id, trash_requested_at = @requested_at, trash_reason = NULL
WHERE channel_id = @channel_id;

-- name: DropTrashRequest :exec
-- A restore drops a folder request that no runner has reported on.
UPDATE project_bindings SET trash_state = NULL, trash_agent_id = NULL, trash_requested_at = NULL, trash_reason = NULL
WHERE channel_id = @channel_id AND trash_state = 'requested';

-- name: SetTrashCheck :exec
-- The folder check of the serving runner (D-DEL-10), runtime state without a binding version.
UPDATE project_bindings
SET trash_check_ok = @ok, trash_check_reason = sqlc.narg(reason), trash_checked_at = @checked_at
WHERE channel_id = @channel_id;

-- name: DuePurges :many
-- The deleted items whose undo window ended before @before and whose content is not purged yet,
-- oldest deletion first, over the partial indexes *_purge_pending_idx.
SELECT d.kind::text AS kind, d.id::uuid AS id
FROM (
    SELECT 'space' AS kind, sp.id, sp.deleted_at AS at FROM spaces sp
    WHERE sp.deleted_at IS NOT NULL AND sp.purged_at IS NULL AND sp.deleted_at < sqlc.arg(before)::timestamptz
    UNION ALL
    SELECT 'channel' AS kind, ch.id, ch.deleted_at AS at FROM channels ch
    WHERE ch.deleted_at IS NOT NULL AND ch.purged_at IS NULL AND ch.deleted_at < sqlc.arg(before)::timestamptz
    UNION ALL
    SELECT 'thread' AS kind, th.id, th.deleted_at AS at FROM threads th
    WHERE th.deleted_at IS NOT NULL AND th.purged_at IS NULL AND th.deleted_at < sqlc.arg(before)::timestamptz
) AS d
ORDER BY d.at, d.id
LIMIT sqlc.arg(max_rows)::integer;

-- name: PurgeThread :execrows
-- The end of the purge of a deleted thread: its title goes (title_source stays) and purged_at is
-- set; a purged thread is left as it is.
UPDATE threads
SET title = '', version = version + 1, purged_at = @purged_at
WHERE id = @id AND deleted_at IS NOT NULL AND purged_at IS NULL;

-- name: ClearThreadTitlesOfChannels :exec
-- The titles of every thread of the channels of a purged channel or space (title_source stays).
UPDATE threads SET title = '', version = version + 1 WHERE channel_id = ANY (@channel_ids::uuid[]) AND title <> '';

-- name: PurgeChannel :execrows
-- The end of the purge of a deleted channel: its topic goes and purged_at is set.
UPDATE channels
SET topic = '', version = version + 1, purged_at = @purged_at
WHERE id = @id AND deleted_at IS NOT NULL AND purged_at IS NULL;

-- name: ClearTopicsOfSpace :exec
-- The topics of every channel of a purged space.
UPDATE channels SET topic = '', version = version + 1 WHERE space_id = @space_id AND topic <> '';

-- name: PurgeSpace :execrows
-- The end of the purge of a deleted space: purged_at is set.
UPDATE spaces SET purged_at = @purged_at WHERE id = @id AND deleted_at IS NOT NULL AND purged_at IS NULL;

-- name: PendingTrash :many
-- The folder requests of a runner whose forum is purged, the final deletion (D-DEL-11).
SELECT pb.channel_id, pb.workspace_ref
FROM project_bindings pb
JOIN channels c ON c.id = pb.channel_id
WHERE pb.trash_state = 'requested' AND pb.trash_agent_id = @agent_id AND c.purged_at IS NOT NULL
ORDER BY pb.channel_id;

-- name: LockTrashRequests :many
-- The requested folder moves of a runner among channel_ids, locked in ascending channel id.
SELECT channel_id
FROM project_bindings
WHERE channel_id = ANY (@channel_ids::uuid[]) AND trash_state = 'requested' AND trash_agent_id = @agent_id
ORDER BY channel_id
FOR UPDATE;

-- name: SetTrashResult :exec
-- The outcome a runner reported for its requested folder move; stored once.
UPDATE project_bindings SET trash_state = @result, trash_reason = sqlc.narg(reason)
WHERE channel_id = @channel_id AND trash_state = 'requested';
