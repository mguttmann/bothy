-- SPDX-License-Identifier: Apache-2.0
-- Queries on threads (chat module, M1-55; docs/spec/03-data-model.md section 4.5). Only this
-- module reads and writes the table (03 principle 12). The main thread of a text channel is
-- inserted by threads_main.sql with its channel (M1-53).

-- name: GetThread :one
SELECT * FROM threads WHERE id = @id;

-- name: LockThread :one
-- The thread locked for a change (03 section 6.3: domain rows before event_seq).
SELECT * FROM threads WHERE id = @id FOR UPDATE;

-- name: InsertThread :one
INSERT INTO threads (id, channel_id, kind, parent_message_id, title, title_source, operator_agent_id, created_by, created_at,
    last_activity_at)
VALUES (@id, @channel_id, 'thread', sqlc.narg(parent_message_id), @title, @title_source, sqlc.narg(operator_agent_id), @created_by,
    @created_at, @created_at)
RETURNING *;

-- name: SaveThread :exec
-- Writes the columns of thread.title.update, thread.close and thread.reopen and the version.
UPDATE threads
SET title = @title, title_source = @title_source, state = @state, closed_at = sqlc.narg(closed_at),
    closed_by = sqlc.narg(closed_by), version = @version
WHERE id = @id;

-- name: AppendToThread :one
-- The thread sequence (03 section 6.2): the caller holds the row lock; a rolled back transaction
-- returns the number with it, so there are no gaps. With reopen a closed thread opens again.
UPDATE threads
SET last_message_seq = last_message_seq + 1, last_activity_at = @now, message_count = message_count + 1,
    version = version + 1,
    state = CASE WHEN @reopen::boolean THEN 'open' ELSE state END,
    closed_at = CASE WHEN @reopen::boolean THEN NULL ELSE closed_at END,
    closed_by = CASE WHEN @reopen::boolean THEN NULL ELSE closed_by END
WHERE id = @id
RETURNING *;

-- name: ListThreadsByActivity :many
-- listChannelThreads sorted by last_activity (brief D6): the threads of kind thread of a
-- channel, keyset (last_activity_at DESC, id DESC) after the position of the cursor.
SELECT t.* FROM threads t
WHERE t.channel_id = @channel_id AND t.kind = 'thread'
  AND (sqlc.narg(state)::text IS NULL OR t.state = sqlc.narg(state)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM thread_tags tt WHERE tt.thread_id = t.id AND tt.tag_id = sqlc.narg(tag_id)::uuid))
  AND (NOT @has_after::boolean OR (t.last_activity_at, t.id) < (@after_at::timestamptz, @after_id::uuid))
ORDER BY t.last_activity_at DESC, t.id DESC
LIMIT @max_rows;

-- name: ListThreadsByCreated :many
-- listChannelThreads sorted by created: keyset (created_at DESC, id DESC).
SELECT t.* FROM threads t
WHERE t.channel_id = @channel_id AND t.kind = 'thread'
  AND (sqlc.narg(state)::text IS NULL OR t.state = sqlc.narg(state)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM thread_tags tt WHERE tt.thread_id = t.id AND tt.tag_id = sqlc.narg(tag_id)::uuid))
  AND (NOT @has_after::boolean OR (t.created_at, t.id) < (@after_at::timestamptz, @after_id::uuid))
ORDER BY t.created_at DESC, t.id DESC
LIMIT @max_rows;
