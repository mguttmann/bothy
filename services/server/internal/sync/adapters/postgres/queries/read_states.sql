-- SPDX-License-Identifier: Apache-2.0
-- Read states (M1-61; 03 read_states, 04 section 14). Every statement names the person
-- (user_id); unread and mention counters are never stored (R-DM-32).

-- name: GetReadState :one
SELECT * FROM read_states WHERE user_id = sqlc.arg(user_id) AND thread_id = sqlc.arg(thread_id);

-- name: LockReadState :one
-- The row locked for markThreadUnread, which compares its version with If-Match.
SELECT * FROM read_states WHERE user_id = sqlc.arg(user_id) AND thread_id = sqlc.arg(thread_id) FOR UPDATE;

-- name: AdvanceReadState :one
-- Inserts the row with version 1 or moves an existing one forward: the position only grows
-- (GREATEST). No row is returned when the stored position is already at least the new one, so
-- nothing changes and the version stays.
INSERT INTO read_states (user_id, thread_id, channel_id, last_read_seq, version, updated_by_device_id, updated_at)
VALUES (sqlc.arg(user_id), sqlc.arg(thread_id), sqlc.arg(channel_id), sqlc.arg(last_read_seq), 1,
        sqlc.narg(updated_by_device_id), sqlc.arg(updated_at))
ON CONFLICT (user_id, thread_id) DO UPDATE
SET last_read_seq = EXCLUDED.last_read_seq, channel_id = EXCLUDED.channel_id, version = read_states.version + 1,
    updated_by_device_id = EXCLUDED.updated_by_device_id, updated_at = EXCLUDED.updated_at
WHERE read_states.last_read_seq < EXCLUDED.last_read_seq
RETURNING *;

-- name: SetReadState :one
-- Moves the locked row back (markThreadUnread) and raises its version.
UPDATE read_states
SET last_read_seq = sqlc.arg(last_read_seq), version = version + 1, updated_by_device_id = sqlc.narg(updated_by_device_id),
    updated_at = sqlc.arg(updated_at)
WHERE user_id = sqlc.arg(user_id) AND thread_id = sqlc.arg(thread_id)
RETURNING *;

-- name: ListReadStatesOfChannel :many
-- The rows of the person in one channel (index read_states_user_channel_idx).
SELECT * FROM read_states WHERE user_id = sqlc.arg(user_id) AND channel_id = sqlc.arg(channel_id) ORDER BY thread_id;

-- name: ListReadStatesInChannels :many
-- The rows of the person in the given channels (the visible set), for the unread counters of the
-- snapshot (M1-63).
SELECT * FROM read_states WHERE user_id = sqlc.arg(user_id) AND channel_id = ANY(sqlc.arg(channel_ids)::uuid[]) ORDER BY thread_id;
