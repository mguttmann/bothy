-- SPDX-License-Identifier: Apache-2.0
-- Queries of the table host_session_users (M2-39, ruling T12).

-- name: LockSessionUsersOfHost :many
-- Every row of a host, valid and ended, locked in ascending id (03 section 6.3).
SELECT * FROM host_session_users WHERE host_id = @host_id ORDER BY id FOR UPDATE;

-- name: SessionUsersOfHost :many
-- Every row of a host, valid and ended, ascending id, without a lock (the check before the
-- rights change; the rights change locks them again, 03 section 6.3).
SELECT * FROM host_session_users WHERE host_id = @host_id ORDER BY id;

-- name: ValidSessionUsersOfHosts :many
-- The valid assignments of the hosts, ordered by host, creation, then id (HostView.session_users).
SELECT * FROM host_session_users
WHERE host_id = ANY (@host_ids::uuid[]) AND revoked_at IS NULL
ORDER BY host_id, created_at, id;

-- name: InsertSessionUser :exec
INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by, created_at, owner_account)
VALUES (@id, @host_id, @user_id, @os_user, @created_by, @created_at, @owner_account);

-- name: RevokeSessionUser :execrows
-- Ends a valid assignment; the row stays and its name stays taken.
UPDATE host_session_users SET revoked_at = @revoked_at, revoked_by = @revoked_by
WHERE id = @id AND revoked_at IS NULL;
