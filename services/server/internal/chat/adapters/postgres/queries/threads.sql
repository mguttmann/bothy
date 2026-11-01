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
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at
FROM threads WHERE id = @id;

-- name: LockThread :one
-- The thread locked for a change (03 section 6.3: domain rows before event_seq).
SELECT id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at
FROM threads WHERE id = @id FOR UPDATE;

-- name: ShareThread :one
-- The thread held against a concurrent move while sync copies its channel (M1-61).
SELECT id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at
FROM threads WHERE id = @id FOR SHARE;

-- name: GetThreadByParentMessage :one
-- The thread that started from a message of a main thread (03 threads: parent_message_id is unique;
-- the replay of an incoming webhook post with a title in a text channel, M2-27).
SELECT id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at
FROM threads WHERE parent_message_id = @parent_message_id;

-- name: InsertThread :one
-- search_language goes in as text and is cast here, so pgx needs no codec for regconfig.
INSERT INTO threads (id, channel_id, kind, parent_message_id, title, title_source, operator_agent_id, created_by, created_at,
    last_activity_at, search_language)
VALUES (@id, @channel_id, 'thread', sqlc.narg(parent_message_id), @title, @title_source, sqlc.narg(operator_agent_id), @created_by,
    @created_at, @created_at, sqlc.arg(search_language)::text::regconfig)
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at;

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
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version, deleted_at, deleted_by, purged_at;

-- name: ListThreadsByActivity :many
-- listChannelThreads sorted by last_activity (brief D6): the threads of kind thread of a
-- channel that are not deleted (D-DEL-3), keyset (last_activity_at DESC, id DESC) after the position of the cursor.
SELECT t.id, t.channel_id, t.kind, t.parent_message_id, t.title, t.title_source, t.state, t.closed_at, t.closed_by,
    t.operator_agent_id, t.last_message_seq, t.unread_floor_seq, t.last_activity_at, t.message_count, t.created_by, t.created_at,
    t.moved_at, t.version, t.deleted_at, t.deleted_by, t.purged_at
FROM threads t
WHERE t.channel_id = @channel_id AND t.kind = 'thread' AND t.deleted_at IS NULL
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
    t.moved_at, t.version, t.deleted_at, t.deleted_by, t.purged_at
FROM threads t
WHERE t.channel_id = @channel_id AND t.kind = 'thread' AND t.deleted_at IS NULL
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

-- name: ThreadCounters :many
-- The counters of the snapshot of sync (M1-63, 07 section 7.4 "unread counter"): the threads of
-- the given channels (the visible set of the caller) with a message above unread_floor_seq, so
-- somebody may have something unread there. The read positions live in sync, which combines both
-- in Go (07 7.4 rule 6, no join across modules).
SELECT id, channel_id, last_message_seq, unread_floor_seq
FROM threads
WHERE channel_id = ANY(sqlc.arg(channel_ids)::uuid[]) AND last_message_seq > unread_floor_seq AND deleted_at IS NULL
ORDER BY id;

-- name: ThreadIDsOfChannels :many
-- Every thread of the channels, ascending: the targets of the cleanup of notification
-- preferences when a channel or space is archived (M3-18).
SELECT id FROM threads WHERE channel_id = ANY (@channel_ids::uuid[]) ORDER BY id;

-- name: LocateThreads :many
-- The routing data of a batch of threads (M2-05, chat.Threads.Locate: "Running now" resolves the
-- threads of the active runs) in one statement, the fields of Threads.Info: the channel kind and
-- space, and the archived flag of the channel or its space. A dm channel has no space, hence the
-- LEFT JOIN. The caller filters by its Authorizer.Scope, so there is no channel predicate here.
SELECT t.id, t.channel_id, c.space_id, c.kind AS channel_kind, t.kind, t.state, t.operator_agent_id, t.created_by, t.version,
    (c.archived_at IS NOT NULL OR s.archived_at IS NOT NULL)::boolean AS channel_archived, t.last_message_seq, t.unread_floor_seq
FROM threads t
JOIN channels c ON c.id = t.channel_id
LEFT JOIN spaces s ON s.id = c.space_id
WHERE t.id = ANY(sqlc.arg(thread_ids)::uuid[])
ORDER BY t.id;

-- name: ActiveThreads :many
-- The active threads of the snapshot of sync (M2-52, 04 section 6.3 item 4, R-SY-28) in one
-- statement: the open threads of kind thread among the pinned ids (the caller's active runs and
-- unread threads, never left out; read through the primary key) in the given channels, plus per
-- channel a LATERAL read over threads_channel_state_activity_idx (channel_id, state,
-- last_activity_at DESC) of the open threads active since the given time, newest first, limited
-- to the bound (sync.snapshot_threads_per_channel) minus the pinned threads of that channel. Both
-- parts carry the whole row, so no third read of threads joins them (a join back by id planned a
-- sequential scan, TestActiveThreads_QueryPlan). The result follows the order of the channel list,
-- then descending activity.
WITH pinned AS (
    SELECT p.id, p.channel_id, p.kind, p.parent_message_id, p.title, p.title_source, p.state, p.closed_at, p.closed_by,
        p.operator_agent_id, p.last_message_seq, p.unread_floor_seq, p.last_activity_at, p.message_count, p.created_by,
        p.created_at, p.moved_at, p.version, p.deleted_at, p.deleted_by, p.purged_at
    FROM threads p
    WHERE p.id = ANY (sqlc.arg(pinned_ids)::uuid[]) AND p.channel_id = ANY (sqlc.arg(channel_ids)::uuid[])
      AND p.kind = 'thread' AND p.state = 'open' AND p.deleted_at IS NULL
), recent AS (
    SELECT r.id, r.channel_id, r.kind, r.parent_message_id, r.title, r.title_source, r.state, r.closed_at, r.closed_by,
        r.operator_agent_id, r.last_message_seq, r.unread_floor_seq, r.last_activity_at, r.message_count, r.created_by,
        r.created_at, r.moved_at, r.version, r.deleted_at, r.deleted_by, r.purged_at
    FROM unnest(sqlc.arg(channel_ids)::uuid[]) AS ch(id)
    CROSS JOIN LATERAL (
        SELECT w.id, w.channel_id, w.kind, w.parent_message_id, w.title, w.title_source, w.state, w.closed_at, w.closed_by,
            w.operator_agent_id, w.last_message_seq, w.unread_floor_seq, w.last_activity_at, w.message_count, w.created_by,
            w.created_at, w.moved_at, w.version, w.deleted_at, w.deleted_by, w.purged_at
        FROM threads w
        WHERE w.channel_id = ch.id AND w.state = 'open' AND w.kind = 'thread' AND w.deleted_at IS NULL
          AND w.last_activity_at >= sqlc.arg(since)::timestamptz
          AND NOT (w.id = ANY (sqlc.arg(pinned_ids)::uuid[]))
        ORDER BY w.last_activity_at DESC, w.id DESC
        LIMIT GREATEST(sqlc.arg(per_channel)::bigint - (SELECT count(*) FROM pinned pc WHERE pc.channel_id = ch.id), 0)
    ) r
)
SELECT t.id, t.channel_id, t.kind, t.parent_message_id, t.title, t.title_source, t.state, t.closed_at, t.closed_by,
    t.operator_agent_id, t.last_message_seq, t.unread_floor_seq, t.last_activity_at, t.message_count, t.created_by, t.created_at,
    t.moved_at, t.version, t.deleted_at, t.deleted_by, t.purged_at
FROM (
    SELECT pinned.id, pinned.channel_id, pinned.kind, pinned.parent_message_id, pinned.title, pinned.title_source, pinned.state,
        pinned.closed_at, pinned.closed_by, pinned.operator_agent_id, pinned.last_message_seq, pinned.unread_floor_seq,
        pinned.last_activity_at, pinned.message_count, pinned.created_by, pinned.created_at, pinned.moved_at, pinned.version, pinned.deleted_at, pinned.deleted_by, pinned.purged_at
    FROM pinned
    UNION ALL
    SELECT recent.id, recent.channel_id, recent.kind, recent.parent_message_id, recent.title, recent.title_source, recent.state,
        recent.closed_at, recent.closed_by, recent.operator_agent_id, recent.last_message_seq, recent.unread_floor_seq,
        recent.last_activity_at, recent.message_count, recent.created_by, recent.created_at, recent.moved_at, recent.version, recent.deleted_at, recent.deleted_by, recent.purged_at
    FROM recent
) AS t
ORDER BY array_position(sqlc.arg(channel_ids)::uuid[], t.channel_id), t.last_activity_at DESC, t.id DESC;

-- name: SaveThreadDeletion :exec
-- Writes a delete or restore of thread.delete and thread.restore (D-DEL-3) and the version.
UPDATE threads SET deleted_at = sqlc.narg(deleted_at), deleted_by = sqlc.narg(deleted_by), version = @version WHERE id = @id;
