-- SPDX-License-Identifier: Apache-2.0
-- Queries on principals and users (identity, docs/spec/03-datenmodell.md section 4.2).

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
