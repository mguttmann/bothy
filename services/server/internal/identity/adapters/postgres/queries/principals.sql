-- SPDX-License-Identifier: Apache-2.0
-- Queries on principals and users (identity, docs/spec/03-data-model.md section 4.2).

-- name: InsertPrincipal :one
INSERT INTO principals (id, kind, status, handle, display_name, avatar_file_id)
VALUES (@id, @kind, @status, @handle, @display_name, @avatar_file_id)
RETURNING created_at;

-- name: InsertUser :exec
INSERT INTO users (principal_id, email, locale, time_zone)
VALUES (@principal_id, @email, @locale, @time_zone);

-- name: GetPrincipal :one
SELECT sqlc.embed(p),
       EXISTS (SELECT 1 FROM owners o WHERE o.user_id = p.id AND o.revoked_at IS NULL) AS is_owner
FROM principals p
WHERE p.id = @id;

-- name: ListPrincipalsByIDs :many
SELECT sqlc.embed(p)
FROM principals p
WHERE p.id = ANY (@ids::uuid[])
ORDER BY p.id;

-- name: GetPrincipalByHandle :one
SELECT sqlc.embed(p)
FROM principals p
WHERE lower(p.handle) = lower(@handle);

-- name: GetUser :one
SELECT sqlc.embed(p), sqlc.embed(u),
       EXISTS (SELECT 1 FROM owners o WHERE o.user_id = p.id AND o.revoked_at IS NULL) AS is_owner
FROM principals p
JOIN users u ON u.principal_id = p.id
WHERE p.id = @id;

-- name: LockPrincipal :one
-- The principal id locked for a change of its status or display name (M1-51; 03 section 6.3).
SELECT sqlc.embed(p)
FROM principals p
WHERE p.id = @id
FOR UPDATE;

-- name: SetPrincipalStatus :one
-- active clears disabled_at, disabled sets it to the transaction time (03 principals, M1-51).
UPDATE principals
SET status = @status,
    disabled_at = CASE WHEN @status::text = 'disabled' THEN now() ELSE NULL END
WHERE id = @id
RETURNING *;

-- name: SetBotPrincipalStatus :execrows
-- The status of an agent or app principal (M2-24, M1-51): active clears disabled_at, disabled
-- sets it; people change status through their own queries (08 section 11).
UPDATE principals SET status = @status, disabled_at = sqlc.narg(disabled_at)
WHERE id = @id AND kind IN ('agent', 'app');

-- name: UpdateBotDisplayName :execrows
-- The display name of an agent or app principal (M2-24: an upgraded manifest renames the app).
UPDATE principals SET display_name = @display_name
WHERE id = @id AND kind IN ('agent', 'app');
