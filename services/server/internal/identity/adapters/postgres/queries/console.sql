-- SPDX-License-Identifier: Apache-2.0
-- Queries of the web console session (identity, docs/spec/08-identitaet.md section 7.5, M6-01):
-- the cookie lives only as HMAC in auth_sessions.cookie_hash (unique partial index), "View as
-- person" in view_as_user_id and view_as_started_at (R-DM-49).

-- name: InsertConsoleSession :exec
-- A console session with its cookie hash; last_seen_at starts at the login, so the idle rule has a
-- start.
INSERT INTO auth_sessions (id, user_id, device_id, client_component, identity_id, auth_method, cookie_hash,
                           created_at, expires_at, last_seen_at, last_ip)
VALUES (@id, @user_id, @device_id, 'console', sqlc.narg(identity_id), @auth_method, @cookie_hash,
        @created_at, @expires_at, @created_at, sqlc.narg(last_ip)::inet);

-- name: GetConsoleAuthState :one
-- The authentication lookup of a console request by the HMAC of its cookie: the session with the
-- state of its device and the status of its person, in one indexed query.
SELECT sqlc.embed(s),
       d.state  AS device_state,
       p.status AS user_status
FROM auth_sessions s
JOIN devices d ON d.id = s.device_id
JOIN principals p ON p.id = s.user_id
WHERE s.cookie_hash = @cookie_hash;

-- name: SetConsoleCookieHash :execrows
-- Rotates the cookie of an active console session; the old value stops working at commit.
UPDATE auth_sessions SET cookie_hash = @cookie_hash
WHERE id = @id AND client_component = 'console' AND revoked_at IS NULL;

-- name: StoreConsoleView :execrows
-- Stores "View as person" on an active console session (access.view_as from the console).
UPDATE auth_sessions SET view_as_user_id = @view_as_user_id, view_as_started_at = @started_at
WHERE id = @id AND client_component = 'console' AND revoked_at IS NULL;

-- name: ClearConsoleView :one
-- Ends the stored view of a session and returns what it was. With a start given, only that view
-- ends, so two requests that both found it expired write one end entry.
WITH old AS (
    SELECT o.id, o.view_as_user_id, o.view_as_started_at FROM auth_sessions o WHERE o.id = @session_id FOR UPDATE
)
UPDATE auth_sessions s SET view_as_user_id = NULL, view_as_started_at = NULL
FROM old
WHERE s.id = old.id AND old.view_as_user_id IS NOT NULL
  AND (sqlc.narg(started_at)::timestamptz IS NULL OR old.view_as_started_at = sqlc.narg(started_at)::timestamptz)
RETURNING old.view_as_user_id, old.view_as_started_at;

-- name: TouchConsoleSession :exec
-- Records activity of a console session and its device, each only where last_seen_at is older
-- than the write bound (one hundredth of auth.console_session_idle_timeout).
WITH sess AS (
    UPDATE auth_sessions SET last_seen_at = @seen_at, last_ip = coalesce(sqlc.narg(client_ip)::inet, auth_sessions.last_ip)
    WHERE auth_sessions.id = @session_id AND auth_sessions.revoked_at IS NULL
      AND (auth_sessions.last_seen_at IS NULL OR auth_sessions.last_seen_at < @threshold)
    RETURNING auth_sessions.id
)
UPDATE devices SET last_seen_at = @seen_at
WHERE devices.id = @device_id AND (devices.last_seen_at IS NULL OR devices.last_seen_at < @threshold);

-- name: ListDueConsoleSessions :many
-- The open console sessions past their absolute end or idle since before idle_cutoff, for the job
-- identity.auth_cleanup, ascending after the last id of the previous page.
SELECT id FROM auth_sessions
WHERE client_component = 'console' AND revoked_at IS NULL
  AND (expires_at <= @now OR last_seen_at IS NULL OR last_seen_at <= @idle_cutoff)
  AND (sqlc.narg(after)::uuid IS NULL OR id > sqlc.narg(after)::uuid)
ORDER BY id
LIMIT @row_limit;
