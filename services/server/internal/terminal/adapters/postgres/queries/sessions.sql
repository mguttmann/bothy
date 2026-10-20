-- SPDX-License-Identifier: Apache-2.0
-- Queries of the terminal sessions of M2-44: opening, attach, reads, the
-- counters of the relay and the reconciliation at the start.

-- name: InsertTerminalSession :exec
-- A new session in opening (24 section 6.1 step 3): the person's own session user, the device and
-- login session of the grant, the jti of its token and the key generation of the host.
INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id, auth_session_id,
    profile, state, token_jti, key_generation, opened_at)
VALUES (@id, @host_id, @user_id, @host_session_user_id, @device_id, @device_id, @auth_session_id, @profile, 'opening',
    @token_jti, @key_generation, @opened_at);

-- name: GetTerminalSession :one
-- One session without a lock (the descriptor before authorize, reads).
SELECT * FROM terminal_sessions WHERE id = @id;

-- name: AttachTerminalSession :execrows
-- A new grant of a session that is not closed (24 section 6.2): device, login session, token jti
-- and version; opened_at, host_session_user_id and key_generation never change.
UPDATE terminal_sessions
SET device_id = @device_id, auth_session_id = @auth_session_id, token_jti = @token_jti, version = version + 1
WHERE id = @id AND state <> 'closed';

-- name: ActivateTerminalSession :execrows
-- detached -> active when a started session pairs again (Design note 7 of M2-44).
UPDATE terminal_sessions SET state = 'active', detached_at = NULL, version = version + 1
WHERE id = @id AND state = 'detached';

-- name: AddTerminalSessionCounters :execrows
-- The counts of one pairing of the relay, added; last_input_at only forward. Metadata without an
-- event and without a new version (ruling D-M2-44-5).
UPDATE terminal_sessions
SET bytes_in = bytes_in + @bytes_in, bytes_out = bytes_out + @bytes_out,
    last_input_at = CASE WHEN sqlc.narg(last_input_at)::timestamptz IS NULL THEN last_input_at
        ELSE greatest(coalesce(last_input_at, sqlc.narg(last_input_at)::timestamptz), sqlc.narg(last_input_at)::timestamptz) END
WHERE id = @id;

-- name: ListTerminalSessions :many
-- One page of sessions in descending id after the keyset position: those of one person (zero
-- user: everyone), optionally of one host and in one state (ruling D-M2-44-10).
SELECT * FROM terminal_sessions
WHERE (@user_id::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR user_id = @user_id)
  AND (@host_id::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR host_id = @host_id)
  AND (@state::text = '' OR state = @state)
  AND (@before_id::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR id < @before_id)
ORDER BY id DESC
LIMIT @page_limit;

-- name: UnfinishedTerminalSessions :many
-- The sessions in opening or active, ascending id: the reconciliation at the start of a single
-- node (Design note 14 of M2-44).
SELECT id FROM terminal_sessions WHERE state IN ('opening', 'active') ORDER BY id;
