-- SPDX-License-Identifier: Apache-2.0
-- The session column of runs (M2-12): runs.agent_session_id names the agent_sessions row a run
-- attached at its start or at the first agent_session.report (03 runs, 09 9.1). agents owns
-- agent_sessions and never reads runs (02 4.4); these queries are the side of runs: the column,
-- the question whether a run used a row (the mode of 09 9.1) and the declared source
-- agents.RunSource. runs has no channel_id; each query names one thread or run that the caller
-- decided first (tools/lint/sql_visibility.yaml).

-- name: SetRunSession :execrows
-- Writes only the column: no version change and no event, because the run document shows the
-- column at the next read and the session row announces itself with agent_session.changed.
UPDATE runs SET agent_session_id = sqlc.arg(session_id) WHERE id = sqlc.arg(id);

-- name: SessionUsed :one
-- Whether some run of the thread already used the session row (resume rather than bind, 09 9.1):
-- a run that started with it. The acceptance records the row a run is invoked with before the run
-- starts, so a queued run that never started has not used it. A row belongs to one thread, and
-- only runs of that thread attach it, so the thread prefix of runs_thread_queued_idx serves the
-- lookup.
SELECT EXISTS (
    SELECT 1 FROM runs
    WHERE thread_id = sqlc.arg(thread_id) AND agent_session_id = sqlc.arg(session_id) AND started_at IS NOT NULL
) AS used;

-- name: LockActiveRunForReport :one
-- The run of the thread in queued, running or waiting_input with its row lock for the rest of the
-- unit of work (agents.RunSource, R-DM-14): a report locks the run before its session row (03
-- 6.3). The partial unique index runs_thread_active_key serves it. The same statement as
-- LockActiveRunOfThread of the follow-ups, kept apart because it runs before authorize.
SELECT * FROM runs
WHERE thread_id = sqlc.arg(thread_id) AND state IN ('queued', 'running', 'waiting_input')
FOR UPDATE;
