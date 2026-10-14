-- SPDX-License-Identifier: Apache-2.0
-- Queries of the table terminal_sessions (M2-39: the end of sessions by the server).

-- name: LockOpenSessions :many
-- The sessions of a host that are not closed, of one person or of everyone (zero user), locked in
-- ascending id (03 section 6.3).
SELECT * FROM terminal_sessions
WHERE host_id = @host_id AND state <> 'closed'
  AND (@user_id::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR user_id = @user_id)
ORDER BY id
FOR UPDATE;

-- name: CloseSession :execrows
-- Closes a session as the server: closed_at, close_reason, no detail and no closer, detached_at
-- cleared (03 terminal_sessions_detached_check), version raised.
UPDATE terminal_sessions
SET state = 'closed', closed_at = @closed_at, close_reason = @close_reason, detached_at = NULL, version = version + 1
WHERE id = @id AND state <> 'closed';
