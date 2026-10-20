-- SPDX-License-Identifier: Apache-2.0
-- Queries on auth_sessions (identity, docs/spec/03-datenmodell.md section 4.2, M1-34). Rows of
-- several sessions are locked in ascending id (03 "Sperrreihenfolge").

-- name: InsertSession :exec
INSERT INTO auth_sessions (id, user_id, device_id, client_component, parent_session_id, identity_id, auth_method,
                           created_at, expires_at, last_ip)
VALUES (@id, @user_id, @device_id, @client_component, sqlc.narg(parent_session_id), sqlc.narg(identity_id),
        @auth_method, @created_at, @expires_at, sqlc.narg(last_ip)::inet);

-- name: GetSession :one
SELECT * FROM auth_sessions WHERE id = @id;

-- name: LockSession :one
-- The session row, locked first: a transaction that adds a child holds this lock, so the children
-- read afterwards (LockChildren, a new statement with a new snapshot) include its child.
SELECT * FROM auth_sessions WHERE id = @id FOR UPDATE;

-- name: LockChildren :many
-- The children of a session, locked in ascending id after their parent.
SELECT * FROM auth_sessions WHERE parent_session_id = @parent_id ORDER BY id FOR UPDATE;

-- name: RevokeSessions :many
-- Revokes the sessions among ids that are not revoked yet and returns their ids.
UPDATE auth_sessions SET revoked_at = @revoked_at, revoke_reason = @revoke_reason
WHERE id = ANY(@ids::uuid[]) AND revoked_at IS NULL
RETURNING id;

-- name: ListActiveSessionsOfUser :many
-- Keyset on id DESC (UUIDv7, newest first); before is the id of the last row of the previous page.
SELECT * FROM auth_sessions
WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > @now
  AND (sqlc.narg(before)::uuid IS NULL OR id < sqlc.narg(before)::uuid)
ORDER BY id DESC
LIMIT @row_limit;

-- name: GetAuthState :one
-- The authentication lookup of one request: the session with its device and the device's active
-- dpop key, in one indexed query (brief M1-34 Scope 2).
SELECT sqlc.embed(s),
       d.state        AS device_state,
       d.last_seen_at AS device_last_seen_at,
       d.approved_at  AS device_approved_at,
       k.jkt          AS dpop_jkt,
       d.created_at   AS device_created_at
FROM auth_sessions s
JOIN devices d ON d.id = s.device_id
LEFT JOIN device_keys k ON k.device_id = d.id AND k.purpose = 'dpop' AND k.revoked_at IS NULL
WHERE s.id = @id;

-- name: TouchSession :exec
-- Records activity of a session and its device; each row only when its last_seen_at is older than
-- the threshold (identity.last_active_granularity) or unset. One statement.
WITH sess AS (
    UPDATE auth_sessions SET last_seen_at = @seen_at, last_ip = coalesce(sqlc.narg(client_ip)::inet, auth_sessions.last_ip)
    WHERE auth_sessions.id = @session_id
      AND (auth_sessions.last_seen_at IS NULL OR auth_sessions.last_seen_at < @threshold)
    RETURNING auth_sessions.id
)
UPDATE devices SET last_seen_at = @seen_at
WHERE devices.id = @device_id AND (devices.last_seen_at IS NULL OR devices.last_seen_at < @threshold);

-- name: ListOpenSessionsOfDevice :many
-- The sessions a device revocation ends (M1-38), children included.
SELECT id FROM auth_sessions WHERE device_id = @device_id AND revoked_at IS NULL ORDER BY id;

-- name: ClearSessionStepUp :exec
-- A pairing code without an open pairing ends the step-up of the approving session (M1-38).
UPDATE auth_sessions SET step_up_at = NULL, step_up_method = NULL WHERE id = @id;

-- name: EndedAuthSessions :many
-- The sessions among ids that ended by at: revoked, or expires_at not after at (M2-46, the job
-- terminal.sweep). Unknown ids give no row.
SELECT id FROM auth_sessions
WHERE id = ANY(@ids::uuid[]) AND (revoked_at IS NOT NULL OR expires_at <= @at)
ORDER BY id;
