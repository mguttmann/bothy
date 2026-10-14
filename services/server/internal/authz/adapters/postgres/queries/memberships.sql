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

-- name: ListActiveMembershipsOfGroups :many
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.group_id = ANY (@group_ids::uuid[]) AND m.removed_at IS NULL
ORDER BY m.id;

-- name: ListMembershipsOfUserInGroup :many
-- Removed rows included: group.member.remove tells "never a member" from "already removed".
SELECT sqlc.embed(m)
FROM group_memberships m
WHERE m.group_id = @group_id AND m.user_id = @user_id
ORDER BY m.id;

-- name: ListMembersInEffect :many
-- One keyset page of the memberships of a group in effect at @at, ascending by membership id
-- (listGroupMembers, M1-48).
SELECT sqlc.embed(m)
FROM group_memberships m
WHERE m.group_id = @group_id AND m.removed_at IS NULL
  AND (m.expires_at IS NULL OR m.expires_at > @at) AND m.id > @after_id
ORDER BY m.id
LIMIT @max_rows;

-- name: LockClaimMembershipsOfUser :many
-- The claim sync of an OIDC login (M1-42, 08 section 3.4): the memberships of the person that
-- are not removed, expired ones included, in the live oidc_claim groups of one provider, each
-- with its group, locked in ascending id (03 section 6.3 item 2).
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.user_id = @user_id AND m.removed_at IS NULL
  AND g.source = 'oidc_claim' AND g.provider_id = @provider_id AND g.deleted_at IS NULL
ORDER BY m.id
FOR UPDATE OF m;

-- name: ExtendMembership :execrows
-- The claim sync extends a membership that is not removed (M1-42): the new expiry starts a new
-- reminder and a new expiry report, so both markers are cleared.
UPDATE group_memberships
SET expires_at = @expires_at, expiry_reminded_at = NULL, expiry_reported_at = NULL
WHERE id = @id AND removed_at IS NULL;
