-- SPDX-License-Identifier: Apache-2.0
-- Queries on threads (chat module, M1-55; docs/spec/03-data-model.md section 4.5). Only this
-- module reads and writes the table (03 principle 12). The main thread of a text channel is
-- inserted by threads_main.sql with its channel (M1-53).
--
-- The columns are listed instead of "*": no read carries search_language or the generated
-- title_search (docs/DECISIONS.md D-M3-13-10), so the rows stay without regconfig and tsvector.
-- Every INSERT names search_language, the language of the creator (14 3.1, M3-13); the static
-- test TestThreadInsertsSetSearchLanguage holds it.

-- name: GetThread :one
SELECT id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version
FROM threads WHERE id = @id;

-- name: LockThread :one
-- The thread locked for a change (03 section 6.3: domain rows before event_seq).
SELECT id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version
FROM threads WHERE id = @id FOR UPDATE;

-- name: ShareThread :one
-- The thread held against a concurrent move while sync copies its channel (M1-61).
SELECT id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version
FROM threads WHERE id = @id FOR SHARE;

-- name: InsertThread :one
-- search_language goes in as text and is cast here, so pgx needs no codec for regconfig.
INSERT INTO threads (id, channel_id, kind, parent_message_id, title, title_source, operator_agent_id, created_by, created_at,
    last_activity_at, search_language)
VALUES (@id, @channel_id, 'thread', sqlc.narg(parent_message_id), @title, @title_source, sqlc.narg(operator_agent_id), @created_by,
    @created_at, @created_at, sqlc.arg(search_language)::text::regconfig)
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version;

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
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version;

-- name: ListThreadsByActivity :many
-- listChannelThreads sorted by last_activity (brief D6): the threads of kind thread of a
-- channel, keyset (last_activity_at DESC, id DESC) after the position of the cursor.
SELECT t.id, t.channel_id, t.kind, t.parent_message_id, t.title, t.title_source, t.state, t.closed_at, t.closed_by,
    t.operator_agent_id, t.last_message_seq, t.unread_floor_seq, t.last_activity_at, t.message_count, t.created_by, t.created_at,
    t.moved_at, t.version
FROM threads t
WHERE t.channel_id = @channel_id AND t.kind = 'thread'
  AND (sqlc.narg(state)::text IS NULL OR t.state = sqlc.narg(state)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM thread_tags tt WHERE tt.thread_id = t.id AND tt.tag_id = sqlc.narg(tag_id)::uuid))
  AND (NOT @has_after::boolean OR (t.last_activity_at, t.id) < (@after_at::timestamptz, @after_id::uuid))
ORDER BY t.last_activity_at DESC, t.id DESC
LIMIT @max_rows;

-- name: ListThreadsByCreated :many
-- listChannelThreads sorted by created: keyset (created_at DESC, id DESC).
SELECT t.id, t.channel_id, t.kind, t.parent_message_id, t.title, t.title_source, t.state, t.closed_at, t.closed_by,
    t.operator_agent_id, t.last_message_seq, t.unread_floor_seq, t.last_activity_at, t.message_count, t.created_by, t.created_at,
    t.moved_at, t.version
FROM threads t
WHERE t.channel_id = @channel_id AND t.kind = 'thread'
  AND (sqlc.narg(state)::text IS NULL OR t.state = sqlc.narg(state)::text)
  AND (sqlc.narg(tag_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM thread_tags tt WHERE tt.thread_id = t.id AND tt.tag_id = sqlc.narg(tag_id)::uuid))
  AND (NOT @has_after::boolean OR (t.created_at, t.id) < (@after_at::timestamptz, @after_id::uuid))
ORDER BY t.created_at DESC, t.id DESC
LIMIT @max_rows;

-- name: DecrementMessageCount :execrows
-- One message of the locked thread was deleted (M1-58, 03 threads: message_count counts messages
-- without deleted ones); a count of zero is left alone, so the caller sees no row and fails.
UPDATE threads SET message_count = message_count - 1, version = version + 1
WHERE id = @id AND message_count > 0;
