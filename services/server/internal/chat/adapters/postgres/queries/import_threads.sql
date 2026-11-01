-- SPDX-License-Identifier: Apache-2.0
-- Queries of the imported threads (M8-11; docs/spec/22-import-migration.md 5.1.1, 6.4, 12.2 and
-- docs/spec/04-sync-protocol.md 15.8). Only the port chat.ImportThreads runs them, inside the unit
-- of work of one import batch.

-- name: InsertImportedThread :one
-- A thread of kind thread with the times, the state and the planned operator of the source. The
-- insert names search_language like every insert of threads (14 3.1: the instance language).
INSERT INTO threads (id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, operator_agent_id,
    created_by, created_at, last_activity_at, search_language)
VALUES (@id, @channel_id, 'thread', sqlc.narg(parent_message_id), @title, @title_source, @state, sqlc.narg(closed_at),
    sqlc.narg(operator_agent_id), @created_by, @created_at, @last_activity_at, sqlc.arg(search_language)::text::regconfig)
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at;

-- name: SaveImportedThread :exec
-- The mirror changes of a posted thread (22 12.2 step 3) without the version: the batch raises it
-- once at its end.
UPDATE threads
SET title = @title, state = @state, closed_at = sqlc.narg(closed_at), closed_by = sqlc.narg(closed_by),
    last_activity_at = @last_activity_at
WHERE id = @id;

-- name: ReserveImportedSeqs :one
-- n thread numbers for imported messages under the row lock (03 section 6.2), without a reopen
-- and without the version; last_activity_at only moves forward.
UPDATE threads
SET last_message_seq = last_message_seq + sqlc.arg(n)::bigint, message_count = message_count + sqlc.arg(n)::bigint,
    last_activity_at = GREATEST(last_activity_at, @last_activity_at)
WHERE id = @id
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at;

-- name: FinishImportBatch :one
-- The end of a batch that changed the thread (04 15.8): imported content is unread for nobody, and
-- the version moves once.
UPDATE threads
SET unread_floor_seq = last_message_seq, version = version + 1
WHERE id = @id
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at;
