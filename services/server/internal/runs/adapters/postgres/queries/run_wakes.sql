-- SPDX-License-Identifier: Apache-2.0
-- run_wakes and run_autonomy (docs/spec/03-data-model.md section 4.8, AR-04, docs/spec/09-agents.md
-- 5.8). Neither table has a channel_id: every read names the rows of one wake-up, run, thread or
-- agent that the use case authorized in the same unit of work, or is the job runs.wake_due without a
-- principal (tools/lint/sql_visibility.yaml lists each read). Lock order (03 section 6.3): the runs
-- row first, then the wake-up, then the counts.

-- name: InsertWake :one
-- A new wake-up; the conflict targets are the idempotency keys of run.wake.recover and of an
-- outage, so a repeated request stores nothing and no row comes back. ended_at is set exactly for a
-- wake-up stored in an end state (a refused deferred request, 09 5.8.2).
INSERT INTO run_wakes (id, agent_id, thread_id, source_run_id, triggered_by, kind, condition, label, reason, outage,
    not_before, deadline, state, state_reason, created_at, ended_at)
VALUES (sqlc.arg(id), sqlc.arg(agent_id), sqlc.arg(thread_id), sqlc.arg(source_run_id), sqlc.arg(triggered_by), sqlc.arg(kind),
    sqlc.arg(condition), sqlc.arg(label), sqlc.arg(reason), sqlc.narg(outage), sqlc.arg(not_before), sqlc.arg(deadline),
    sqlc.arg(state), sqlc.narg(state_reason), sqlc.arg(created_at),
    CASE WHEN sqlc.arg(state)::text = 'pending' THEN NULL ELSE sqlc.arg(created_at)::timestamptz END)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: LockWake :one
-- One wake-up with its row lock.
SELECT * FROM run_wakes WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: WakeByOutage :one
-- The wake-up of one outage of a source run, locked (run_wakes_outage_key).
SELECT * FROM run_wakes WHERE source_run_id = sqlc.arg(source_run_id) AND outage = sqlc.arg(outage) FOR UPDATE;

-- name: WakeBySourceReason :one
-- The wake-up of run.wake.recover of a source run and reason, locked (run_wakes_recover_key).
SELECT * FROM run_wakes
WHERE source_run_id = sqlc.arg(source_run_id) AND reason = sqlc.arg(reason) AND outage IS NULL
    AND (reason = 'recovery' OR reason = 'usage_limit')
FOR UPDATE;

-- name: UpdateWake :one
-- The compare-and-swap of a state change: ended_at follows the state, the window changes only
-- with a repeated usage_limit request, the fired columns only with a fire or a skipped input.
UPDATE run_wakes SET
    state = sqlc.arg(state),
    state_reason = sqlc.narg(state_reason),
    ended_at = CASE WHEN sqlc.arg(state)::text = 'pending' THEN NULL ELSE sqlc.arg(at)::timestamptz END,
    not_before = COALESCE(sqlc.narg(not_before), not_before),
    deadline = COALESCE(sqlc.narg(deadline), deadline),
    result_text = COALESCE(sqlc.narg(result_text), result_text),
    fired_run_id = CASE WHEN sqlc.arg(clear_fired)::boolean THEN NULL ELSE COALESCE(sqlc.narg(fired_run_id), fired_run_id) END,
    fired_into_run_id = CASE WHEN sqlc.arg(clear_fired)::boolean THEN NULL
        ELSE COALESCE(sqlc.narg(fired_into_run_id), fired_into_run_id) END,
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: CountPendingWakes :one
-- The pending wake-ups of one thread (run.wake_max_pending).
SELECT count(*) FROM run_wakes WHERE thread_id = sqlc.arg(thread_id) AND state = 'pending';

-- name: CountUsageWakes :one
-- The usage_limit wake-ups of one source run in every state (run.usage_wake_max).
SELECT count(*) FROM run_wakes WHERE source_run_id = sqlc.arg(source_run_id) AND reason = 'usage_limit';

-- name: PendingWakesOfAgent :many
-- The pending wake-ups of one agent (run.wake.list_pending), ascending by id.
SELECT * FROM run_wakes WHERE agent_id = sqlc.arg(agent_id) AND state = 'pending' ORDER BY id;

-- name: ThreadWakes :many
-- One page of the wake-ups of one thread, newest first (run.wake.read).
SELECT * FROM run_wakes
WHERE thread_id = sqlc.arg(thread_id)
    AND (NOT sqlc.arg(has_after)::boolean OR (created_at, id) < (sqlc.arg(after_created_at)::timestamptz, sqlc.arg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ReadyWakesOfThread :many
-- The pending wake-ups of one thread that could fire now: a due at wake-up, or a wake-up whose
-- condition the runner reported or whose wake input was skipped (09 5.8.3).
SELECT id FROM run_wakes
WHERE thread_id = sqlc.arg(thread_id) AND state = 'pending'
    AND ((kind = 'at' AND not_before <= sqlc.arg(now)::timestamptz)
        OR state_reason IN ('condition_met', 'wake_input_skipped', 'run_not_running'))
ORDER BY not_before, id;

-- name: PendingWakeCounts :many
-- The pending wake-ups by kind for the gauge bothy.run.wake.pending.
SELECT kind, count(*) AS n FROM run_wakes WHERE state = 'pending' GROUP BY kind ORDER BY kind;

-- name: LockAutonomy :one
-- The counts of one thread and person with the row lock.
SELECT * FROM run_autonomy WHERE thread_id = sqlc.arg(thread_id) AND person_id = sqlc.arg(person_id) FOR UPDATE;

-- name: SaveAutonomy :exec
-- Inserts or replaces the counts of one thread and person.
INSERT INTO run_autonomy (thread_id, person_id, chain_count, idle_count, ended_reason, updated_at)
VALUES (sqlc.arg(thread_id), sqlc.arg(person_id), sqlc.arg(chain_count), sqlc.arg(idle_count), sqlc.narg(ended_reason), sqlc.arg(updated_at))
ON CONFLICT (thread_id, person_id) DO UPDATE SET
    chain_count = EXCLUDED.chain_count, idle_count = EXCLUDED.idle_count, ended_reason = EXCLUDED.ended_reason,
    updated_at = EXCLUDED.updated_at;

-- name: ResetAutonomy :exec
-- A run or follow-up of the person ends the chain: both counts 0, no ended reason (03 run_autonomy).
UPDATE run_autonomy SET chain_count = 0, idle_count = 0, ended_reason = NULL, updated_at = sqlc.arg(updated_at)
WHERE thread_id = sqlc.arg(thread_id) AND person_id = sqlc.arg(person_id)
    AND (chain_count <> 0 OR idle_count <> 0 OR ended_reason IS NOT NULL);
