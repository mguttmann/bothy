-- SPDX-License-Identifier: Apache-2.0
-- Queries of the directory use cases on principals and users (identity, M1-31; docs/spec/03-
-- data-model.md section 4.2). Only this module reads these tables (03 "Principles" rule 12).

-- name: ListUsers :many
-- One page of people, keyset on (lower(handle), id) ascending. q is a case-insensitive prefix
-- of the handle or the display name, compared with starts_with, so "%" and "_" are literal.
SELECT sqlc.embed(p), sqlc.embed(u),
       EXISTS (SELECT 1 FROM owners o WHERE o.user_id = p.id AND o.revoked_at IS NULL) AS is_owner
FROM principals p
JOIN users u ON u.principal_id = p.id
WHERE p.kind = 'user'
  AND (sqlc.narg(status)::text IS NULL OR p.status = sqlc.narg(status)::text)
  AND (sqlc.narg(q)::text IS NULL
       OR starts_with(lower(p.handle), lower(sqlc.narg(q)::text))
       OR starts_with(lower(p.display_name), lower(sqlc.narg(q)::text)))
  AND (sqlc.narg(after_handle)::text IS NULL
       OR (lower(p.handle), p.id) > (sqlc.narg(after_handle)::text, sqlc.narg(after_id)::uuid))
ORDER BY lower(p.handle), p.id
LIMIT @row_limit;

-- name: LockUser :one
-- The person id with its row of principals locked for the change (03 "Lock order").
SELECT sqlc.embed(p), sqlc.embed(u),
       EXISTS (SELECT 1 FROM owners o WHERE o.user_id = p.id AND o.revoked_at IS NULL) AS is_owner
FROM principals p
JOIN users u ON u.principal_id = p.id
WHERE p.id = @id
FOR UPDATE OF p;

-- name: UpdatePrincipalDisplayName :exec
UPDATE principals SET display_name = @display_name WHERE id = @id;

-- name: UpdateUserProfile :exec
UPDATE users SET email = sqlc.narg(email), locale = @locale, time_zone = @time_zone
WHERE principal_id = @principal_id;

-- name: EnablePrincipal :exec
-- user.enable: pending or disabled to active; disabled_at is cleared (08 section 11).
UPDATE principals SET status = 'active', disabled_at = NULL
WHERE id = @id AND status IN ('pending', 'disabled');
