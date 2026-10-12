-- SPDX-License-Identifier: Apache-2.0
-- Queries on owners (identity, docs/spec/03-datenmodell.md section 4.2).

-- name: InsertOwner :one
INSERT INTO owners (id, user_id, granted_by, reason)
VALUES (@id, @user_id, @granted_by, @reason)
RETURNING granted_at;

-- name: ListActiveOwners :many
SELECT sqlc.embed(o), sqlc.embed(p)
FROM owners o
JOIN principals p ON p.id = o.user_id
WHERE o.revoked_at IS NULL
ORDER BY o.granted_at, o.id;

-- name: ListActiveOwnerIDs :many
SELECT o.user_id
FROM owners o
JOIN principals p ON p.id = o.user_id
WHERE o.revoked_at IS NULL AND p.status = 'active'
ORDER BY o.user_id;

-- name: LockActiveOwners :many
-- The active owners rows, locked in ascending id (03 "Sperrreihenfolge", M1-31).
SELECT id, user_id FROM owners WHERE revoked_at IS NULL ORDER BY id FOR UPDATE;

-- name: GetActiveOwner :one
SELECT sqlc.embed(o), sqlc.embed(p)
FROM owners o
JOIN principals p ON p.id = o.user_id
WHERE o.user_id = @user_id AND o.revoked_at IS NULL;

-- name: RevokeOwner :execrows
UPDATE owners SET revoked_at = now(), revoked_by = @revoked_by
WHERE user_id = @user_id AND revoked_at IS NULL;
