-- SPDX-License-Identifier: Apache-2.0
-- Queries on agent_sessions (agents module, M2-12; docs/spec/03-datenmodell.md agent_sessions).
-- Only this module reads and writes the table (03 Grundsatz 12). Several rows are locked in
-- ascending id (03 section 6.3).

-- name: ThreadSessions :many
-- The session history of one thread, newest first (listAgentSessions).
SELECT * FROM agent_sessions WHERE thread_id = @thread_id ORDER BY created_at DESC, id DESC;

-- name: CurrentSession :one
SELECT * FROM agent_sessions WHERE thread_id = @thread_id AND is_current;

-- name: LockCurrentSession :one
SELECT * FROM agent_sessions WHERE thread_id = @thread_id AND is_current FOR UPDATE;

-- name: LockSessions :many
SELECT * FROM agent_sessions WHERE id = ANY (@ids::uuid[]) ORDER BY id FOR UPDATE;

-- name: CurrentByRef :one
-- The row in which an external session is current, in any thread (the bind conflicts of 09 9.4).
SELECT * FROM agent_sessions WHERE agent_id = @agent_id AND external_session_ref = @external_session_ref AND is_current;

-- name: LatestByRef :one
-- The most recent row of an external session in any thread (the cost baseline of a bind).
SELECT * FROM agent_sessions
WHERE agent_id = @agent_id AND external_session_ref = @external_session_ref
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: LatestInThreadByRef :one
-- The most recent row of an external session in one thread (the attach rules of run.start).
SELECT * FROM agent_sessions
WHERE thread_id = @thread_id AND agent_id = @agent_id AND external_session_ref = @external_session_ref
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: InsertSession :one
-- No row comes back when one of the partial unique indexes refused the row (the thread has a
-- current row, or the external session is current in another thread). A NULL currency takes
-- 'USD', the column default of 03.
INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, started, model,
    cost_baseline_micros, currency, created_at, ended_at, end_reason, version)
VALUES (@id, @thread_id, @agent_id, @external_session_ref, @is_current, @started, sqlc.narg(model),
    @cost_baseline_micros, coalesce(sqlc.narg(currency)::text, 'USD'), @created_at, sqlc.narg(ended_at), sqlc.narg(end_reason),
    @version)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: EndSession :one
-- Ends a current row at its version; no row comes back when the row is not current or has
-- another version.
UPDATE agent_sessions
SET is_current = false, ended_at = @ended_at, end_reason = @end_reason, version = version + 1
WHERE id = @id AND version = @version AND is_current
RETURNING *;

-- name: UpdateSession :one
-- Writes reference, model and started at the row's version; no row comes back for another
-- version.
UPDATE agent_sessions
SET external_session_ref = @external_session_ref, model = sqlc.narg(model), started = @started, version = version + 1
WHERE id = @id AND version = @version
RETURNING *;
