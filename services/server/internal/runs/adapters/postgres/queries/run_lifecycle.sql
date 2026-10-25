-- SPDX-License-Identifier: Apache-2.0
-- The lifecycle of a run after its delivery (M2-09): start, renewal, wait, end, cancellation,
-- revocation and orphans. Every change locks the run row first (LockRun of delivery.sql) and
-- swaps through TransitionRun or RecordDelivery; several runs are locked in ascending id (03
-- section 6.3). runs has no channel_id; the runs are named by the request, the revoking change,
-- the disabled agent or the job (tools/lint/sql_visibility.yaml).

-- name: LockActiveRunsByStarter :many
-- The active runs started by one of the principals whose access changed, locked in ascending id
-- (R-SY-11); the subscriber keeps those whose channel the principal lost.
SELECT * FROM runs
WHERE state IN ('queued', 'running', 'waiting_input') AND triggered_by = ANY (sqlc.arg(principal_ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: LockActiveRunsByThreads :many
-- The active runs of the threads of a channel that became read-only (M8-03, 05
-- channel.read_only.set), locked in ascending id; the partial unique index runs_thread_active_key
-- serves it. chat names the threads, because runs has no channel_id.
SELECT * FROM runs
WHERE state IN ('queued', 'running', 'waiting_input') AND thread_id = ANY (sqlc.arg(thread_ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: LockActiveRunsByAgent :many
-- The active runs of a disabled agent, locked in ascending id (09 2.1); the index
-- runs_state_agent_active_idx serves it.
SELECT * FROM runs
WHERE state IN ('queued', 'running', 'waiting_input') AND agent_id = sqlc.arg(agent_id)
ORDER BY id
FOR UPDATE;

-- name: OrphanCandidates :many
-- The started active runs for the job runs.stall_check, ascending by id; at most one per thread
-- (R-DM-14). No lock, because each run is decided again under its own lock (LockRun).
SELECT * FROM runs
WHERE state IN ('running', 'waiting_input')
ORDER BY id;

-- name: MarkRunStalled :one
-- The stall marking of runs.stall_check (09 5.5, M2-51 design note 5): the locked run is still
-- running at the version the job read, made no progress since stalled_before and has no notice
-- yet; it gets the notice and one version step. No row: the run changed in between and is skipped.
UPDATE runs
SET stall_warned_at = sqlc.arg(at)::timestamptz,
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) AND state = 'running'
    AND stall_warned_at IS NULL AND last_progress_at < sqlc.arg(stalled_before)::timestamptz
RETURNING *;
