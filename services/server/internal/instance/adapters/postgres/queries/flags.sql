-- SPDX-License-Identifier: Apache-2.0

-- name: GetFlag :many
-- The row of one flag by its primary key; no row for a flag without a stored value.
SELECT key, enabled, rollout, version, updated_by, updated_at
FROM feature_flags
WHERE key = sqlc.arg(key);

-- name: ListFlags :many
-- Every stored flag in key order, registered or orphaned.
SELECT key, enabled, rollout, version, updated_by, updated_at
FROM feature_flags
ORDER BY key;

-- name: LockFlag :many
-- The row of one flag under its row lock until the commit; no row for a flag without one.
SELECT key, enabled, rollout, version, updated_by, updated_at
FROM feature_flags
WHERE key = sqlc.arg(key)
FOR UPDATE;

-- name: InsertFlag :many
-- The first write of a flag (version 1). A concurrent first write of the same key makes this
-- insert wait for it and then insert nothing (no row returned); the writer then locks the row and
-- starts again.
INSERT INTO feature_flags (key, enabled, rollout, version, updated_by, updated_at)
VALUES (sqlc.arg(key), sqlc.arg(enabled), sqlc.arg(rollout), 1, sqlc.narg(updated_by), sqlc.arg(updated_at))
ON CONFLICT (key) DO NOTHING
RETURNING key, enabled, rollout, version, updated_by, updated_at;

-- name: UpdateFlag :one
-- A later write of a locked row: version + 1.
UPDATE feature_flags
SET enabled = sqlc.arg(enabled), rollout = sqlc.arg(rollout), version = version + 1, updated_by = sqlc.narg(updated_by),
    updated_at = sqlc.arg(updated_at)
WHERE key = sqlc.arg(key)
RETURNING key, enabled, rollout, version, updated_by, updated_at;
