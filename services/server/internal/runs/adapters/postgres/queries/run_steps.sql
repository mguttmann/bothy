-- SPDX-License-Identifier: Apache-2.0
-- run_steps (docs/spec/03-data-model.md section 4.8, M2-05). Every read names one run whose
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

-- name: LiveFacts :many
-- The live frame of stats (M6-15, docs/spec/25-visualization.md 5.2): per active run of the
-- batch the kind and name of its newest running step and whether an active approval component
-- (required_capability approve_actions, 09 11.5) waits. Never title, detail or values; the live
-- builder drops every run outside the channel set of each receiver before anything is sent.
SELECT r.id::uuid AS run_id,
       coalesce(s.kind, '')::text AS step_kind,
       coalesce(s.name, '')::text AS step_name,
       EXISTS (
           SELECT 1 FROM message_components mc
           WHERE mc.run_id = r.id AND mc.state = 'active' AND mc.required_capability = 'approve_actions'
       )::boolean AS approval_pending
FROM unnest(sqlc.arg(run_ids)::uuid[]) AS r(id)
LEFT JOIN LATERAL (
    SELECT kind, name FROM run_steps
    WHERE run_id = r.id AND status = 'running'
    ORDER BY ordinal DESC
    LIMIT 1
) s ON true;
