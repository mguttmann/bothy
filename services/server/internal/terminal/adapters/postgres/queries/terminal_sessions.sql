-- SPDX-License-Identifier: Apache-2.0
-- Queries of the table terminal_sessions (M2-39): the open sessions of a host; the end itself is
-- EndTerminalSession of ending.sql (M2-46).

-- name: LockOpenSessions :many
-- The sessions of a host that are not closed, of one person or of everyone (zero user), locked in
-- ascending id (03 section 6.3).
SELECT * FROM terminal_sessions
WHERE host_id = @host_id AND state <> 'closed'
  AND (@user_id::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR user_id = @user_id)
ORDER BY id
FOR UPDATE;
