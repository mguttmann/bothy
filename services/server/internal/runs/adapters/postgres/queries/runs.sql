-- SPDX-License-Identifier: Apache-2.0
-- runs (docs/spec/03-datenmodell.md section 4.8, M2-05). runs has no channel_id: every read
-- follows the visibility rule of docs/DECISIONS.md D-M2-05-9 (07 section 7.4 rule 1). It reads the
-- rows of one thread or run whose visibility the use case checked first, the active runs whose
-- threads the use case resolves through the chat port and filters by Scope, or (the gauge) all
-- active runs without a principal. tools/lint/sql_visibility.yaml lists each read.

-- name: InsertRun :one
-- The conflict target names the predicate of runs_thread_active_key: a thread with an active run
-- gets no second one (R-DM-14), and no row comes back.
INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, state, version,
    billing_mode_expected, invocation_jti, queued_at)
VALUES (sqlc.arg(id), sqlc.arg(thread_id), sqlc.arg(agent_id), sqlc.arg(trigger_kind), sqlc.narg(triggering_message_id),
    sqlc.arg(triggered_by), 'queued', 1, sqlc.arg(billing_mode_expected), sqlc.arg(invocation_jti), sqlc.arg(queued_at))
ON CONFLICT (thread_id) WHERE state IN ('queued', 'running', 'waiting_input') DO NOTHING
RETURNING *;

-- name: GetRun :one
SELECT * FROM runs WHERE id = sqlc.arg(id);

-- name: ActiveRunOfThread :one
-- The partial unique index runs_thread_active_key serves this; at most one row exists.
SELECT * FROM runs
WHERE thread_id = sqlc.arg(thread_id) AND state IN ('queued', 'running', 'waiting_input');

-- name: TransitionRun :one
-- The compare-and-swap of 03 runs: only the row with the expected state and version moves, and
-- the UPDATE holds its row lock until the commit. started_at keeps its first value (waiting_input
-- -> running starts nothing new); an end state stamps finished_at and the milliseconds since
-- started_at, NULL for a run that never started.
UPDATE runs
SET state = sqlc.arg(to_state)::text,
    state_reason = NULLIF(sqlc.arg(reason)::text, ''),
    version = version + 1,
    started_at = CASE WHEN sqlc.arg(to_state)::text = 'running'
        THEN COALESCE(started_at, sqlc.arg(at)::timestamptz) ELSE started_at END,
    finished_at = CASE WHEN sqlc.arg(to_state)::text IN ('completed', 'failed', 'cancelled')
        THEN sqlc.arg(at)::timestamptz ELSE finished_at END,
    duration_ms = CASE WHEN sqlc.arg(to_state)::text IN ('completed', 'failed', 'cancelled') AND started_at IS NOT NULL
        THEN floor(extract(epoch FROM (sqlc.arg(at)::timestamptz - started_at)) * 1000)::bigint ELSE duration_ms END,
    -- The columns a start, a wait or an end of M2-09 store with the same swap; NULL keeps the
    -- column. A start stores started_jti and clears renewed_from_jti (09 4.5.1).
    started_jti = COALESCE(sqlc.narg(started_jti)::uuid, started_jti),
    renewed_from_jti = CASE WHEN sqlc.narg(started_jti)::uuid IS NOT NULL THEN NULL ELSE renewed_from_jti END,
    billing_mode_used = COALESCE(sqlc.narg(billing_mode_used)::text, billing_mode_used),
    billing_evidence = COALESCE(sqlc.narg(billing_evidence)::jsonb, billing_evidence),
    model = COALESCE(sqlc.narg(model)::text, model),
    status_line = COALESCE(sqlc.narg(status_line)::text, status_line),
    metrics = COALESCE(sqlc.narg(metrics)::jsonb, metrics),
    summary = COALESCE(sqlc.narg(summary)::text, summary)
WHERE id = sqlc.arg(id) AND state = sqlc.arg(from_state)::text AND version = sqlc.arg(version)
RETURNING *;

-- name: ThreadRuns :many
-- Keyset (queued_at DESC, id DESC) over the index runs_thread_queued_idx; after_at and after_id
-- are both NULL for the first page.
SELECT * FROM runs
WHERE thread_id = sqlc.arg(thread_id)
  AND (sqlc.narg(after_at)::timestamptz IS NULL
       OR (queued_at, id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY queued_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ActiveRuns :many
-- The active runs of all threads, optionally of one agent, in batches for "Running now"; the use
-- case keeps only rows whose thread is in its Scope and authorizes each (D-M2-05-9).
SELECT * FROM runs
WHERE state IN ('queued', 'running', 'waiting_input')
  AND (sqlc.narg(agent_id)::uuid IS NULL OR agent_id = sqlc.narg(agent_id)::uuid)
  AND (sqlc.narg(after_at)::timestamptz IS NULL
       OR (queued_at, id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY queued_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ActiveCounts :many
-- The gauge bothy.run.active: one grouped count over runs_state_agent_active_idx, no content.
SELECT agent_id, state, count(*)::bigint AS active
FROM runs
WHERE state IN ('queued', 'running', 'waiting_input')
GROUP BY agent_id, state;
