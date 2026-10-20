-- SPDX-License-Identifier: Apache-2.0
-- Queries of thread.move (chat module, M1-62; docs/spec/03-data-model.md threads, "I1 check" and
-- "Lock order"). The thread row is locked FOR UPDATE by LockThread of threads.sql first; then the
-- target channel and, when it inherits its operator, the target's space are held FOR SHARE
-- (ShareChannel of channels.sql and ShareSpace of spaces.sql, shared with notify), so
-- channel.operator.set and space.operator.set, which update these rows, are serialized with the
-- operator check of the move. The copies in messages, read_states and drafts are changed by the
-- subscribers of messaging and sync in the same transaction.

-- name: MoveThread :one
-- The move of the locked thread: the new channel, the time of the move and version + 1; the id
-- never changes (03 threads.id).
UPDATE threads
SET channel_id = @channel_id, moved_at = @moved_at, version = version + 1
WHERE id = @id
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version;

-- name: DeleteThreadTagsOfThread :exec
-- A tag belongs to the channel of its thread (03 thread_tags), so a move removes every tag.
DELETE FROM thread_tags WHERE thread_id = @thread_id;
