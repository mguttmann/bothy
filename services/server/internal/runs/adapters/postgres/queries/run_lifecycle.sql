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
