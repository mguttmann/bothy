-- SPDX-License-Identifier: Apache-2.0
-- Queries of the expiry jobs authz.expiry_sweep and authz.expiry_reminder (M1-30, 07 sections 8.3
-- and 9.4). The selections run after the rights begin hook has locked permission_version (sweep)
-- and lock their rows in ascending id (03 section 6.3), so two runs never report a row twice.
-- They hang on the stored markers, not on the time of the last run, so a row that expired while
-- the worker was down is found on the next run.

-- name: ListDueExpiredGrants :many
SELECT sqlc.embed(g)
FROM grants g
WHERE g.revoked_at IS NULL AND g.expires_at IS NOT NULL AND g.expires_at <= @at
  AND g.expiry_reported_at IS NULL
ORDER BY g.id
LIMIT @max_rows
FOR UPDATE;

-- name: MarkGrantsExpiryReported :execrows
UPDATE grants
SET expiry_reported_at = @at
WHERE id = ANY (@ids::uuid[]) AND expiry_reported_at IS NULL;

-- name: ListDueExpiredMemberships :many
-- Memberships of deleted groups are reported too; their difference is empty.
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.removed_at IS NULL AND m.expires_at IS NOT NULL AND m.expires_at <= @at
  AND m.expiry_reported_at IS NULL
ORDER BY m.id
LIMIT @max_rows
FOR UPDATE OF m;

-- name: MarkMembershipsExpiryReported :execrows
UPDATE group_memberships
SET expiry_reported_at = @at
WHERE id = ANY (@ids::uuid[]) AND expiry_reported_at IS NULL;

-- name: ListDueReminderGrants :many
-- Grants of deleted groups are skipped in the selection itself, so they never fill a batch.
SELECT sqlc.embed(g)
FROM grants g
WHERE g.revoked_at IS NULL AND g.expires_at IS NOT NULL AND g.expires_at > @at AND g.expires_at <= @until
  AND g.expiry_reminded_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM groups d WHERE d.id = g.subject_group_id AND d.deleted_at IS NOT NULL)
ORDER BY g.id
LIMIT @max_rows
FOR UPDATE OF g;

-- name: MarkGrantsExpiryReminded :execrows
UPDATE grants
SET expiry_reminded_at = @at
WHERE id = ANY (@ids::uuid[]) AND expiry_reminded_at IS NULL;

-- name: ListDueReminderMemberships :many
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.removed_at IS NULL AND m.expires_at IS NOT NULL AND m.expires_at > @at AND m.expires_at <= @until
  AND m.expiry_reminded_at IS NULL AND g.deleted_at IS NULL
ORDER BY m.id
LIMIT @max_rows
FOR UPDATE OF m;

-- name: MarkMembershipsExpiryReminded :execrows
UPDATE group_memberships
SET expiry_reminded_at = @at
WHERE id = ANY (@ids::uuid[]) AND expiry_reminded_at IS NULL;

-- name: ListExpiringGrants :many
-- The list "Expiring soon" (07 section 9.4, M6-07): grants not revoked, not of a deleted group,
-- expiring in (at, until]; a read without lock, ordered by expiry, then id.
SELECT sqlc.embed(g)
FROM grants g
WHERE g.revoked_at IS NULL AND g.expires_at IS NOT NULL AND g.expires_at > @at AND g.expires_at <= @until
  AND NOT EXISTS (SELECT 1 FROM groups d WHERE d.id = g.subject_group_id AND d.deleted_at IS NOT NULL)
ORDER BY g.expires_at, g.id;

-- name: ListExpiringMemberships :many
-- The list "Expiring soon" (M6-07): memberships not removed of live groups expiring in
-- (at, until]; a read without lock, ordered by expiry, then id.
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.removed_at IS NULL AND m.expires_at IS NOT NULL AND m.expires_at > @at AND m.expires_at <= @until
  AND g.deleted_at IS NULL
ORDER BY m.expires_at, m.id;
