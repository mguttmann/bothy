-- SPDX-License-Identifier: Apache-2.0
-- Queries on grants (authz, docs/spec/03-data-model.md section 4.3). There is no delete and
-- no general update: a grant is only revoked (R-DM-09); the trigger authz_grants_immutable holds
-- that in the database.

-- name: InsertGrant :exec
INSERT INTO grants (id, subject_kind, subject_principal_id, subject_group_id, target_kind,
                    target_space_id, target_channel_id, target_host_id, level, capabilities,
                    expires_at, reason, granted_by, granted_at)
VALUES (@id, @subject_kind, @subject_principal_id, @subject_group_id, @target_kind,
        @target_space_id, @target_channel_id, @target_host_id, @level, @capabilities,
        @expires_at, @reason, @granted_by, @granted_at);

-- name: GetGrant :one
SELECT sqlc.embed(g)
FROM grants g
WHERE g.id = @id;

-- name: RevokeGrant :one
-- Only a grant that is not revoked; no row means unknown or already revoked.
UPDATE grants
SET revoked_at = @revoked_at, revoked_by = @revoked_by, revoke_reason = @revoke_reason
WHERE id = @id AND revoked_at IS NULL
RETURNING *;

-- name: ListActiveGrantsOfPrincipal :many
SELECT sqlc.embed(g)
FROM grants g
WHERE g.subject_principal_id = @principal_id AND g.revoked_at IS NULL
ORDER BY g.id;

-- name: ListActiveGrantsOfGroups :many
SELECT sqlc.embed(g)
FROM grants g
WHERE g.subject_group_id = ANY (@group_ids::uuid[]) AND g.revoked_at IS NULL
ORDER BY g.id;

-- name: ListActiveTriggerGrants :many
-- The I1 check (03 section 6.4, 10 section 5.3) filters expiry and served targets in the domain,
-- so one clock decides; hosts carry no level.
SELECT sqlc.embed(g)
FROM grants g
WHERE g.level = 'trigger' AND g.revoked_at IS NULL
ORDER BY g.id;

-- name: ListActiveGrantsOnTargets :many
-- Grants on the instance and on the given spaces and channels: the subjects a rights change on a
-- target affects (02 section 8.1).
SELECT sqlc.embed(g)
FROM grants g
WHERE g.revoked_at IS NULL
  AND (g.target_kind = 'instance'
       OR g.target_space_id = ANY (@space_ids::uuid[])
       OR g.target_channel_id = ANY (@channel_ids::uuid[]))
ORDER BY g.id;
