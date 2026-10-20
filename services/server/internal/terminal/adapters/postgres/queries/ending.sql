-- SPDX-License-Identifier: Apache-2.0
-- Queries of the end of terminal sessions (M2-46): the one closing statement of EndSessions, the
-- open sessions by person, device, login session and host, the sweep candidates, the counts of
-- terminal.open with the person lock. EndTerminalSession is the only statement of the module that
-- sets close_reason (TestEndingSQLOnlyInEndSessions).

-- name: EndTerminalSession :execrows
-- Closes a session through the one end function (Design note 1 of M2-46): reason and detail of the
-- closed lists (checked by domain.ValidateEnd before), exit code only from the host, closed_by only
-- for closeTerminalSession, the counts the relay had not written yet added once, last_input_at only
-- forward, detached_at cleared (03 terminal_sessions_detached_check), version raised.
UPDATE terminal_sessions
SET state = 'closed', closed_at = @closed_at, close_reason = @close_reason, close_detail = sqlc.narg(close_detail),
    exit_code = sqlc.narg(exit_code), closed_by = sqlc.narg(closed_by), detached_at = NULL,
    bytes_in = bytes_in + @bytes_in, bytes_out = bytes_out + @bytes_out,
    last_input_at = CASE WHEN sqlc.narg(last_input_at)::timestamptz IS NULL THEN last_input_at
        ELSE greatest(coalesce(last_input_at, sqlc.narg(last_input_at)::timestamptz), sqlc.narg(last_input_at)::timestamptz) END,
    version = version + 1
WHERE id = @id AND state <> 'closed';

-- name: LockOpenSessionsOfUsers :many
-- The open sessions of the people, locked in ascending id (03 section 6.3): the rights change of a
-- person outside the module (OnAccessChanged).
SELECT * FROM terminal_sessions
WHERE user_id = ANY(@user_ids::uuid[]) AND state <> 'closed'
ORDER BY id
FOR UPDATE;

-- name: LockOpenSessionsOfDevice :many
-- The open sessions whose current device is the device, locked in ascending id (OnDeviceRevoked;
-- the index terminal_sessions_device_open_idx).
SELECT * FROM terminal_sessions
WHERE device_id = @device_id AND state <> 'closed'
ORDER BY id
FOR UPDATE;

-- name: LockOpenSessionsOfAuthSessions :many
-- The open sessions of the login sessions, locked in ascending id (OnSessionEnded; the index
-- terminal_sessions_auth_session_open_idx).
SELECT * FROM terminal_sessions
WHERE auth_session_id = ANY(@auth_session_ids::uuid[]) AND state <> 'closed'
ORDER BY id
FOR UPDATE;

-- name: SweepCandidates :many
-- Every session that is not closed with the state of its host and the host's last
-- require_local_confirm, ascending id (terminal.sweep, Design note 8 of M2-46). No lock: the job
-- locks each session in its own unit of work.
SELECT s.id, s.auth_session_id, h.state AS host_state, h.reported_require_local_confirm
FROM terminal_sessions s
JOIN hosts h ON h.id = s.host_id
WHERE s.state <> 'closed'
ORDER BY s.id;

-- name: LockPersonForOpen :exec
-- The person lock of terminal.open (Design note 9 of M2-46): an advisory lock for the person,
-- taken first in the unit of work, because no table of the module has one row per person.
SELECT pg_advisory_xact_lock(hashtextextended('terminal.open:' || sqlc.arg(user_id)::uuid::text, 0));

-- name: CountOpenSessions :one
-- The sessions that are not closed of the person and of the host (terminal.max_sessions_per_person
-- and terminal.max_sessions_per_host; the indexes terminal_sessions_user_open_idx and
-- terminal_sessions_host_open_idx).
SELECT
    (SELECT count(*) FROM terminal_sessions p WHERE p.user_id = sqlc.arg(person_id)::uuid AND p.state <> 'closed')::bigint AS of_person,
    (SELECT count(*) FROM terminal_sessions h WHERE h.host_id = sqlc.arg(of_host_id)::uuid AND h.state <> 'closed')::bigint AS of_host;
