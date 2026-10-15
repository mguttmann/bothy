-- SPDX-License-Identifier: Apache-2.0
-- Notification levels (M3-18; 03 notification_preferences, 13 section 3.1). Only the module
-- notify reads and writes the table (03 principle 12). Every statement names the person
-- (user_id) or the explicit list of people.

-- name: UpsertPreference :one
-- The single write of a level: inserts the row with version 1 or replaces a different level or
-- muted_until with version + 1. No row is returned for equal values: no change, no version.
INSERT INTO notification_preferences (user_id, target_kind, target_id, level, muted_until, version, updated_at)
VALUES (@user_id, @target_kind, @target_id, @level, sqlc.narg(muted_until), 1, @updated_at)
ON CONFLICT (user_id, target_kind, target_id) DO UPDATE
SET level = EXCLUDED.level, muted_until = EXCLUDED.muted_until, version = notification_preferences.version + 1,
    updated_at = EXCLUDED.updated_at
WHERE (notification_preferences.level, notification_preferences.muted_until) IS DISTINCT FROM (EXCLUDED.level, EXCLUDED.muted_until)
RETURNING *;

-- name: GetPreference :one
SELECT * FROM notification_preferences WHERE user_id = @user_id AND target_kind = @target_kind AND target_id = @target_id;

-- name: DeletePreference :one
DELETE FROM notification_preferences WHERE user_id = @user_id AND target_kind = @target_kind AND target_id = @target_id
RETURNING *;

-- name: ListPreferences :many
SELECT * FROM notification_preferences WHERE user_id = @user_id ORDER BY target_kind, target_id;

-- name: DeletePreferencesOfTargets :many
-- The cleanup on archiving (R-DM-16): every row of the space, of the channels and of the threads,
-- of every person. A NULL space matches nothing.
DELETE FROM notification_preferences
WHERE (target_kind = 'space' AND target_id = sqlc.narg(space_id))
   OR (target_kind = 'channel' AND target_id = ANY (@channel_ids::uuid[]))
   OR (target_kind = 'thread' AND target_id = ANY (@thread_ids::uuid[]))
RETURNING *;

-- name: LevelRows :many
-- The own rows of the people for one thread, its channel and its space in one query (13 section
-- 12 point 2). A NULL space (a DM channel) matches nothing.
SELECT * FROM notification_preferences
WHERE user_id = ANY (@user_ids::uuid[])
  AND ((target_kind = 'thread' AND target_id = @thread_id)
    OR (target_kind = 'channel' AND target_id = @channel_id)
    OR (target_kind = 'space' AND target_id = sqlc.narg(space_id)));
