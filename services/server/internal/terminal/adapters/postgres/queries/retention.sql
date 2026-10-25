-- SPDX-License-Identifier: Apache-2.0
-- The participant of class security (M7-05, docs/spec/20-privacy.md 2 and 4.1): terminal sessions
-- after closed_at, the session users of a host and the host itself after retired_at of the host,
-- oldest first; an open session and a host that is not retired have no end time.

-- name: ExpiredTerminalSessions :many
SELECT id FROM terminal_sessions WHERE closed_at < @cutoff ORDER BY closed_at, id LIMIT @row_limit;

-- name: DeleteTerminalSession :execrows
DELETE FROM terminal_sessions WHERE id = @id AND closed_at < @cutoff;

-- name: ExpiredHostSessionUsers :many
-- 03 host_session_users: never deleted before the host is retired, so no name becomes free.
SELECT u.id FROM host_session_users u JOIN hosts h ON h.id = u.host_id
WHERE h.retired_at < @cutoff ORDER BY h.retired_at, u.id LIMIT @row_limit;

-- name: DeleteHostSessionUser :execrows
DELETE FROM host_session_users u USING hosts h
WHERE u.id = @id AND h.id = u.host_id AND h.retired_at < @cutoff;

-- name: ExpiredHosts :many
SELECT id FROM hosts WHERE retired_at < @cutoff ORDER BY retired_at, id LIMIT @row_limit;

-- name: DeleteHost :execrows
DELETE FROM hosts WHERE id = @id AND retired_at < @cutoff;
