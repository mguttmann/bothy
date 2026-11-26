-- SPDX-License-Identifier: Apache-2.0
-- Queries of computer sessions (CU-04, docs/spec/03-data-model.md section 4.14 computer_sessions,
-- docs/spec/27-computer-use.md) and of the desktop enablement mirror of hosts. Locks follow 03
-- section 6.3: the host row before its sessions, sessions in ascending id. Metadata only.

-- name: InsertComputerSession :exec
INSERT INTO computer_sessions (id, host_id, user_id, agent_id, run_id, state, token_jti, generation, key_generation, opened_at)
VALUES (@id, @host_id, @user_id, @agent_id, @run_id, 'opening', @token_jti, @generation, @key_generation, @opened_at);

-- name: GetComputerSession :one
SELECT * FROM computer_sessions WHERE id = @id;

-- name: LockComputerSession :one
SELECT * FROM computer_sessions WHERE id = @id FOR UPDATE;

-- name: LockOpenComputerSessionsOfHost :many
-- The sessions of a host that are not closed, locked in ascending id (at most one by the unique
-- index; the list form keeps the end functions uniform).
SELECT * FROM computer_sessions WHERE host_id = @host_id AND state <> 'closed' ORDER BY id FOR UPDATE;

-- name: LockOpenComputerSessionsOfUser :many
SELECT * FROM computer_sessions WHERE user_id = @user_id AND state <> 'closed' ORDER BY id FOR UPDATE;

-- name: LockOpenComputerSessionsOfRun :many
SELECT * FROM computer_sessions WHERE run_id = @run_id AND state <> 'closed' ORDER BY id FOR UPDATE;

-- name: OpenComputerSessionOfRun :one
-- Whether the run holds a computer session that is not closed (the follow-up rule of SR13 item 5).
SELECT EXISTS (SELECT 1 FROM computer_sessions WHERE run_id = @run_id AND state <> 'closed');

-- name: CountOpenComputerSessions :one
-- The sessions that are not closed on a host and of a run (computer_busy, computer_run_busy).
SELECT
    (SELECT count(*) FROM computer_sessions c WHERE c.host_id = @host_id AND c.state <> 'closed')::bigint AS of_host,
    (SELECT count(*) FROM computer_sessions r WHERE r.run_id = @run_id AND r.state <> 'closed')::bigint AS of_run;

-- name: OpenComputerHosts :many
-- The hosts with a computer session that is not closed (busy in listComputerDesktops).
SELECT host_id FROM computer_sessions WHERE state <> 'closed' ORDER BY host_id;

-- name: ListComputerSessionsOfUser :many
-- The sessions of a person, newest first, optionally in one state, with the name of the host.
SELECT c.*, h.name AS desktop FROM computer_sessions c JOIN hosts h ON h.id = c.host_id
WHERE c.user_id = @user_id AND (@state::text = '' OR c.state = @state::text)
ORDER BY c.id DESC
LIMIT @max_rows;

-- name: HostName :one
SELECT name FROM hosts WHERE id = @id;

-- name: AttachComputerSession :execrows
-- computer.attach: attached_at once, of a session in opening (27 5.4).
UPDATE computer_sessions SET attached_at = @attached_at, version = version + 1
WHERE id = @id AND state = 'opening' AND attached_at IS NULL;

-- name: StartComputerSession :execrows
-- computer.started: an opening session becomes active or paused (locked).
UPDATE computer_sessions
SET state = @state, started_at = @started_at, pause_reason = sqlc.narg(pause_reason), paused_at = sqlc.narg(paused_at),
    version = version + 1
WHERE id = @id AND state = 'opening';

-- name: UpdateComputerState :execrows
-- computer.state: the state with its pause, the paused time added once at the end of a pause, the
-- action count and the frontmost app.
UPDATE computer_sessions
SET state = @state, pause_reason = sqlc.narg(pause_reason), paused_at = sqlc.narg(paused_at),
    paused_ms = paused_ms + @add_paused_ms, action_count = @action_count, front_app = sqlc.narg(front_app),
    version = version + 1
WHERE id = @id AND state IN ('active', 'paused');

-- name: EndComputerSession :execrows
-- The one closing statement of computer sessions: closed with reason, detail and the person of a
-- client stop; a pause in progress is added to paused_ms; the action count of computer.ended
-- replaces the last state when it is higher.
UPDATE computer_sessions
SET state = 'closed', closed_at = @closed_at, close_reason = @close_reason, close_detail = sqlc.narg(close_detail),
    stopped_by = sqlc.narg(stopped_by), pause_reason = NULL, paused_at = NULL,
    paused_ms = GREATEST(paused_ms + @add_paused_ms, @reported_paused_ms),
    action_count = GREATEST(action_count, @action_count), version = version + 1
WHERE id = @id AND state <> 'closed';

-- name: StaleOpeningComputerSessions :many
-- Sessions still in opening that were opened before the bound (the sweep of computer.open_timeout).
SELECT id FROM computer_sessions WHERE state = 'opening' AND opened_at < @before ORDER BY id;

-- name: SetComputerDisabledGeneration :execrows
-- computer.device.disable: the disabled generation and the next version of the host.
UPDATE hosts SET computer_disabled_generation = @generation, version = @version WHERE id = @id;

-- name: RunHadComputerSession :one
-- Whether the run opened a computer session at all (27 10.5: its agent session ends with
-- computer_use).
SELECT EXISTS (SELECT 1 FROM computer_sessions WHERE run_id = @run_id);

-- name: ComputerSessionsByIDs :many
-- The computer sessions among the ids, ascending (the live_sessions of a login, 24 6.2).
SELECT * FROM computer_sessions WHERE id = ANY (@ids::uuid[]) ORDER BY id;
