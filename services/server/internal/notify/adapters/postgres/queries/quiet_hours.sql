-- SPDX-License-Identifier: Apache-2.0
-- Quiet hours (M3-18; 03 quiet_hours, 13 section 3.4). Only the module notify reads and writes
-- the table (03 principle 12). Every statement names the person (user_id) or the explicit list
-- of people.

-- name: LockRulesOfUser :exec
-- Serializes the writes of one person until the end of the transaction, so every
-- quiet_hours.updated carries the complete list as of its own commit. A hash collision only
-- serializes two people.
SELECT pg_advisory_xact_lock(hashtextextended('notify.quiet_hours:' || sqlc.arg(user_id)::text, 0));

-- name: ListRules :many
SELECT * FROM quiet_hours WHERE user_id = @user_id ORDER BY id;

-- name: GetRule :one
SELECT * FROM quiet_hours WHERE id = @id AND user_id = @user_id;

-- name: InsertRule :one
INSERT INTO quiet_hours (id, user_id, weekdays, start_local, end_local, time_zone, enabled, version)
VALUES (@id, @user_id, @weekdays, CAST(sqlc.arg(start_local)::text AS time), CAST(sqlc.arg(end_local)::text AS time), sqlc.narg(time_zone), @enabled, 1)
RETURNING *;

-- name: UpdateRule :one
-- Replaces the whole rule (an absent time_zone writes NULL) with version + 1.
UPDATE quiet_hours
SET weekdays = @weekdays, start_local = CAST(sqlc.arg(start_local)::text AS time), end_local = CAST(sqlc.arg(end_local)::text AS time),
    time_zone = sqlc.narg(time_zone), enabled = @enabled, version = version + 1
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteRule :one
DELETE FROM quiet_hours WHERE id = @id AND user_id = @user_id RETURNING *;

-- name: RulesOfUsers :many
-- The rules of many people for the push path (M3-19), enabled or not.
SELECT * FROM quiet_hours WHERE user_id = ANY (@user_ids::uuid[]) ORDER BY user_id, id;
