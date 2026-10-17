-- SPDX-License-Identifier: Apache-2.0
-- Queries on groups (authz, docs/spec/03-data-model.md section 4.3).

-- name: InsertGroup :exec
INSERT INTO groups (id, name, description, source, provider_id, external_ref, created_by, created_at)
VALUES (@id, @name, @description, @source, @provider_id, @external_ref, @created_by, @created_at);

-- name: GetGroup :one
SELECT sqlc.embed(g)
FROM groups g
WHERE g.id = @id;

-- name: SoftDeleteGroup :execrows
UPDATE groups
SET deleted_at = @deleted_at
WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateGroup :execrows
UPDATE groups
SET name = @name, description = @description
WHERE id = @id AND deleted_at IS NULL;

-- name: ListLiveGroups :many
-- One keyset page of live groups ascending by id (listGroups, M1-48). With only_member the page
-- holds the groups in which member_id has a membership in effect at @at (not removed, not
-- expired; 07 section 2.5: at expires_at = @at it is already over).
SELECT sqlc.embed(g)
FROM groups g
WHERE g.deleted_at IS NULL AND g.id > @after_id
  AND (NOT @only_member::boolean OR EXISTS (
        SELECT 1 FROM group_memberships m
        WHERE m.group_id = g.id AND m.user_id = @member_id AND m.removed_at IS NULL
          AND (m.expires_at IS NULL OR m.expires_at > @at)))
ORDER BY g.id
LIMIT @max_rows;

-- name: CountMembersInEffect :many
-- member_count of the groups (M1-48): the memberships that are not removed and not expired at @at.
SELECT m.group_id, count(*)::int AS members
FROM group_memberships m
WHERE m.group_id = ANY (@group_ids::uuid[]) AND m.removed_at IS NULL
  AND (m.expires_at IS NULL OR m.expires_at > @at)
GROUP BY m.group_id;
