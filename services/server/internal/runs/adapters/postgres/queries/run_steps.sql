-- SPDX-License-Identifier: Apache-2.0
-- run_steps (docs/spec/03-datenmodell.md section 4.8, M2-05). Every read names one run whose
-- visibility the use case checked first (docs/DECISIONS.md D-M2-05-9); tools/lint/sql_visibility.yaml
-- lists each read.

-- name: AppendStep :one
-- The next ordinal of the run: one more than the highest, 1 for the first. The caller holds the
-- row lock of the run (TransitionRun), so two appends of one run never read the same maximum.
INSERT INTO run_steps (id, run_id, ordinal, parent_step_id, kind, name, title, status, detail, started_at, finished_at)
SELECT sqlc.arg(id)::uuid, sqlc.arg(run_id)::uuid, COALESCE(MAX(ordinal), 0) + 1, sqlc.narg(parent_step_id)::uuid,
    sqlc.arg(kind)::text, sqlc.arg(name)::text, sqlc.arg(title)::text, sqlc.arg(status)::text, sqlc.arg(detail)::jsonb,
    COALESCE(sqlc.narg(started_at)::timestamptz, now()), sqlc.narg(finished_at)::timestamptz
FROM run_steps
WHERE run_id = sqlc.arg(run_id)::uuid
RETURNING *;

-- name: StepCounts :many
-- step_count of a page of summaries in one statement; a run without steps has no row.
SELECT run_id, count(*)::bigint AS steps
FROM run_steps
WHERE run_id = ANY (sqlc.arg(run_ids)::uuid[])
GROUP BY run_id;

-- name: Steps :many
-- One page by ordinal over the unique index (run_id, ordinal).
SELECT * FROM run_steps
WHERE run_id = sqlc.arg(run_id) AND ordinal > sqlc.arg(after_ordinal)
ORDER BY ordinal
LIMIT sqlc.arg(row_limit);

-- name: AllSteps :many
SELECT * FROM run_steps
WHERE run_id = sqlc.arg(run_id)
ORDER BY ordinal;
