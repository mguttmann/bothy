-- SPDX-License-Identifier: Apache-2.0

-- name: ListSettings :many
-- Every stored setting, in key order; the use case drops keys the registry does not know.
SELECT key, value, version, updated_by, updated_at
FROM instance_settings
ORDER BY key;

-- name: LockSettingsWriters :exec
-- Serializes every writer of instance_settings until the commit, also while the rows it will
-- write do not exist yet: a row lock alone would let two writers of different missing keys pass
-- each other and break the branding version as optimistic lock (red team M1-46).
SELECT pg_advisory_xact_lock(hashtext('bothy.instance_settings'));

-- name: LockSettings :many
-- The rows of the given keys under their row lock until the commit, in key order, so two writers
-- of overlapping keys lock in the same order. A key without a row returns nothing; the writer
-- inserts it with InsertSetting.
SELECT key, value, version, updated_by, updated_at
FROM instance_settings
WHERE key = ANY (sqlc.arg(keys)::text[])
ORDER BY key
FOR UPDATE;

-- name: InsertSetting :many
-- The first write of a key (version 1). A concurrent first write of the same key makes this
-- insert wait for it and then insert nothing (no row returned); the writer then locks the row and
-- starts again.
INSERT INTO instance_settings (key, value, version, updated_by, updated_at)
VALUES (sqlc.arg(key), sqlc.arg(value), 1, sqlc.narg(updated_by), sqlc.arg(updated_at))
ON CONFLICT (key) DO NOTHING
RETURNING key, value, version, updated_by, updated_at;

-- name: UpdateSetting :one
-- A later write of a locked row: version + 1.
UPDATE instance_settings
SET value = sqlc.arg(value), version = version + 1, updated_by = sqlc.narg(updated_by), updated_at = sqlc.arg(updated_at)
WHERE key = sqlc.arg(key)
RETURNING key, value, version, updated_by, updated_at;
