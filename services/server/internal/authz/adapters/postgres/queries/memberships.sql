-- SPDX-License-Identifier: Apache-2.0
-- Queries on group_memberships (authz, docs/spec/03-datenmodell.md section 4.3).

-- name: InsertMembership :exec
INSERT INTO group_memberships (id, group_id, user_id, added_by, added_at, expires_at)
VALUES (@id, @group_id, @user_id, @added_by, @added_at, @expires_at);

-- name: RemoveMembership :execrows
UPDATE group_memberships
SET removed_at = @removed_at, removed_by = @removed_by
WHERE id = @id AND removed_at IS NULL;

-- name: ListActiveMembershipsOfUser :many
-- The group row comes along, deleted or not, so the domain decides validity (07 section 2.5).
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.user_id = @user_id AND m.removed_at IS NULL
ORDER BY m.id;
