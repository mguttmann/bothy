-- SPDX-License-Identifier: Apache-2.0
-- Queries of the control connection of a host service (M2-43, 24 section 3.5). The login reads the
-- host with GetHost (state, auth_public_jwk, auth_jkt, key_generation); the caller holds the lock
-- of the host row (LockHost) for every write of a host and locks sessions after it (03 section
-- 6.3).

-- name: UpdateHostLogin :execrows
-- The login: the reported host version and last_seen_at; the version only when the host version
-- changed (the caller passes the next or the current version).
UPDATE hosts
SET reported_host_version = @reported_host_version, last_seen_at = @last_seen_at, version = @version
WHERE id = @id;

-- name: TouchHostLastSeen :execrows
-- A heartbeat: last_seen_at when older than identity.last_active_granularity; no version.
UPDATE hosts SET last_seen_at = @last_seen_at
WHERE id = @id AND (last_seen_at IS NULL OR last_seen_at < @older_than);

-- name: UpdateHostReport :execrows
-- The reported columns of a report besides the session users (M2-40 writes those), and the
-- version.
UPDATE hosts
SET reported_profiles = @reported_profiles::jsonb, reported_doctor = @reported_doctor::jsonb,
    reported_is_server_host = @reported_is_server_host, reported_require_local_confirm = @reported_require_local_confirm,
    reported_os_version = sqlc.narg(reported_os_version), version = @version
WHERE id = @id;

-- name: LockSession :one
-- One session, locked (session.started and session.ended of the host service).
SELECT * FROM terminal_sessions WHERE id = @id FOR UPDATE;

-- name: SessionsByIDs :many
-- The sessions listed in live_sessions, without a lock, ascending id.
SELECT * FROM terminal_sessions WHERE id = ANY (@ids::uuid[]) ORDER BY id;

-- name: StartSession :execrows
-- opening -> active with started_at (session.started).
UPDATE terminal_sessions SET state = 'active', started_at = @started_at, version = version + 1
WHERE id = @id AND state = 'opening';

-- name: DetachSession :execrows
-- active -> detached with detached_at (the loss of the control connection, 24 section 6.2).
UPDATE terminal_sessions SET state = 'detached', detached_at = @detached_at, version = version + 1
WHERE id = @id AND state = 'active';

-- name: CloseSessionReported :execrows
-- Closes a session the host service ended (session.ended): reason, detail and exit code as
-- reported, detached_at cleared, version raised.
UPDATE terminal_sessions
SET state = 'closed', closed_at = @closed_at, close_reason = @close_reason, close_detail = sqlc.narg(close_detail),
    exit_code = sqlc.narg(exit_code), detached_at = NULL, version = version + 1
WHERE id = @id AND state <> 'closed';
