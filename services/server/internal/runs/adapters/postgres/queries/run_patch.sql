-- SPDX-License-Identifier: Apache-2.0
-- Live-box patches and the delivery reads of the live box (M2-10, docs/spec/09-agents.md section
-- 5.3). The run row is locked with LockRun first; every statement here names that one run, whose
-- run.update the use case authorized, or the active runs of threads the gateway checked
-- (docs/DECISIONS.md D-M2-05-9). tools/lint/sql_visibility.yaml lists each read.

-- name: PatchSteps :many
-- Every step of the locked run in ordinal order. The detail is read only for the summary step
-- (kind status, name steps_summarized, member summarized), so a patch never reads every large
-- detail of a run; NULL stands for "not loaded".
SELECT id, run_id, ordinal, parent_step_id, kind, name, title, status,
    CASE WHEN kind = 'status' AND name = 'steps_summarized' AND detail ? 'summarized' THEN detail END::jsonb AS detail,
    started_at, finished_at
FROM run_steps
WHERE run_id = sqlc.arg(run_id)
ORDER BY ordinal;

-- name: PatchRun :one
-- One version step per stored patch, whatever the number of ops (03 runs.version); a patch is
-- movement, so the stall hint goes (ruling D-M2-10-2) and last_progress_at becomes the server
-- time of the patch (09 5.5, M2-51). A false set_* flag keeps the column; a set
-- text column with an empty value becomes NULL (remove).
UPDATE runs
SET status_line = CASE WHEN sqlc.arg(set_status_line)::boolean
        THEN NULLIF(sqlc.arg(status_line)::text, '') ELSE status_line END,
    last_text_excerpt = CASE WHEN sqlc.arg(set_last_text_excerpt)::boolean
        THEN NULLIF(sqlc.arg(last_text_excerpt)::text, '') ELSE last_text_excerpt END,
    model = CASE WHEN sqlc.arg(set_model)::boolean THEN NULLIF(sqlc.arg(model)::text, '') ELSE model END,
    metrics = COALESCE(sqlc.narg(metrics)::jsonb, metrics),
    plan = COALESCE(sqlc.narg(plan)::jsonb, plan),
    cost_micros = CASE WHEN sqlc.arg(set_cost)::boolean THEN sqlc.arg(cost_micros)::bigint ELSE cost_micros END,
    currency = CASE WHEN sqlc.arg(set_cost)::boolean THEN sqlc.arg(currency)::text ELSE currency END,
    cost_is_estimate = CASE WHEN sqlc.arg(set_cost)::boolean THEN sqlc.arg(cost_is_estimate)::boolean ELSE cost_is_estimate END,
    version = version + 1,
    stall_warned_at = NULL,
    last_progress_at = sqlc.arg(progress_at)::timestamptz
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeletePatchSteps :exec
-- The steps the summarization merged (03 run_steps); a deleted step has no child, so the cascade
-- of parent_step_id removes nothing else.
DELETE FROM run_steps
WHERE run_id = sqlc.arg(run_id) AND id = ANY (sqlc.arg(ids)::uuid[]);

-- name: UpdatePatchStep :exec
-- The members of one step a patch replaced; NULL keeps the column.
UPDATE run_steps
SET status = COALESCE(sqlc.narg(status)::text, status),
    title = COALESCE(sqlc.narg(title)::text, title),
    detail = COALESCE(sqlc.narg(detail)::jsonb, detail),
    finished_at = COALESCE(sqlc.narg(finished_at)::timestamptz, finished_at),
    ordinal = COALESCE(sqlc.narg(ordinal)::integer, ordinal)
WHERE run_id = sqlc.arg(run_id) AND id = sqlc.arg(id);

-- name: InsertPatchStep :exec
-- A new step with the ordinal the use case took under the run row lock (unique (run_id, ordinal)).
INSERT INTO run_steps (id, run_id, ordinal, parent_step_id, kind, name, title, status, detail, started_at, finished_at)
VALUES (sqlc.arg(id), sqlc.arg(run_id), sqlc.arg(ordinal), sqlc.narg(parent_step_id), sqlc.arg(kind), sqlc.arg(name),
    sqlc.arg(title), sqlc.arg(status), sqlc.arg(detail), sqlc.arg(started_at), sqlc.narg(finished_at));

-- name: ActiveRunsOfThreads :many
-- The delivery reads of runs.LiveBox: the active runs of the threads the gateway checked with
-- gateway.focus, over the partial unique index runs_thread_active_key.
SELECT * FROM runs
WHERE thread_id = ANY (sqlc.arg(thread_ids)::uuid[]) AND state IN ('queued', 'running', 'waiting_input');
