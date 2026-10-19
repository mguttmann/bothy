-- SPDX-License-Identifier: Apache-2.0
-- Queries of the reported session users as rights state (M2-40, 07 SR12).

-- name: UpdateReportedSessionUsers :execrows
-- Writes the report of the host (schema host_reported_session_users.v1) and the next version; the
-- caller holds the lock of the row.
UPDATE hosts SET reported_session_users = @reported_session_users::jsonb, version = @version WHERE id = @id;

-- name: ValidSessionUsersOfUser :many
-- The valid assignments of one person with the names their hosts report, ascending by host
-- (SR12 in the rights state).
SELECT su.id, su.host_id, su.os_user, h.reported_session_users
FROM host_session_users su
JOIN hosts h ON h.id = su.host_id
WHERE su.user_id = @user_id AND su.revoked_at IS NULL
ORDER BY su.host_id, su.id;
