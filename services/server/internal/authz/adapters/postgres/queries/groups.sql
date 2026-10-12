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
