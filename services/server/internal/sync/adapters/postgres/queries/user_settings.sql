-- SPDX-License-Identifier: Apache-2.0
-- Personal settings (M1-61; 03 user_settings, 04 section 14). Every statement names the person
-- (user_id).

-- name: PutSetting :one
-- Inserts the key with version 1 or replaces a different value with version + 1. jsonb compares
-- values semantically (member order and spacing do not count), so no row is returned for the
-- stored value: no change, no version.
INSERT INTO user_settings (user_id, key, value, version, updated_by_device_id, updated_at)
VALUES (sqlc.arg(user_id), sqlc.arg(key), sqlc.arg(value)::jsonb, 1, sqlc.narg(updated_by_device_id), sqlc.arg(updated_at))
ON CONFLICT (user_id, key) DO UPDATE
SET value = EXCLUDED.value, version = user_settings.version + 1, updated_by_device_id = EXCLUDED.updated_by_device_id,
    updated_at = EXCLUDED.updated_at
WHERE user_settings.value IS DISTINCT FROM EXCLUDED.value
RETURNING *;

-- name: GetSetting :one
SELECT * FROM user_settings WHERE user_id = sqlc.arg(user_id) AND key = sqlc.arg(key);

-- name: DeleteSetting :one
DELETE FROM user_settings WHERE user_id = sqlc.arg(user_id) AND key = sqlc.arg(key) RETURNING *;

-- name: ListSettings :many
SELECT * FROM user_settings WHERE user_id = sqlc.arg(user_id) ORDER BY key;

-- name: SettingValues :many
-- The stored values of the keys of many people in one read (M3-18: notify reads
-- notify.default_level of every candidate of a thread). No defaults: absent rows stay absent.
SELECT user_id, key, value FROM user_settings WHERE user_id = ANY (@user_ids::uuid[]) AND key = ANY (@keys::text[]);
