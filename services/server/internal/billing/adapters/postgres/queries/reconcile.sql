-- SPDX-License-Identifier: Apache-2.0
-- The cost reconciliation of the billing module (M2-28, docs/spec/10-backend-policy.md section
-- 12.4). Every query is instance-level: reconcileCosts and submitCostReconciliation write by run ID
-- after authorize allowed the action on the instance, the job billing.reconcile reads the runs its
-- request named, and listCostEstimates (cost.reconcile.read, R-AZ-28) is evaluated on the instance
-- without a visible-channel predicate and names no channel, thread or content. cost_entries is no
-- content table (class cost). No query updates or deletes a row: a correction is a new row.

-- name: LockReconcileRun :exec
-- The exclusive transaction-scoped lock of one run in the key namespace billing.reconcile:<run_id>
-- (design note 3 c; 03 section 6.3 rule 2): two reconciliations of the same run serialize. No row
-- lock: bothy_app has only SELECT and INSERT on cost_entries, and FOR UPDATE needs UPDATE.
SELECT pg_advisory_xact_lock(hashtextextended('billing.reconcile:' || sqlc.arg(run_id)::uuid::text, 0));

-- name: LockReconcileRunShared :exec
-- The shared form of the same lock for the job (design note 6 a): a request that commits new rows
-- after the job read the totals had to wait for the job, and its own job runs afterwards.
SELECT pg_advisory_xact_lock_shared(hashtextextended('billing.reconcile:' || sqlc.arg(run_id)::uuid::text, 0));

-- name: ValidEntriesInPeriod :many
-- The valid entries (no entry supersedes them) of the runs with occurred_at in the half-open
-- period, every column, over the index (run_id).
SELECT e.* FROM cost_entries e
WHERE e.run_id = ANY (sqlc.arg(run_ids)::uuid[])
  AND e.occurred_at >= sqlc.arg(from_at)::timestamptz AND e.occurred_at < sqlc.arg(to_at)::timestamptz
  AND NOT EXISTS (SELECT 1 FROM cost_entries s WHERE s.supersedes_id = e.id)
ORDER BY e.id;

-- name: InsertReconciledEntry :exec
-- One reconciled entry: the copies of the entry it supersedes, the share of the actual amount,
-- is_estimate false, source reconciled and reconciled_at (design note 3 f). The partial unique
-- index on supersedes_id refuses a second successor.
INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id, billing_mode, provider,
    model, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, tokens_cache_write_1h, amount_micros, currency,
    is_estimate, source, occurred_at, reconciled_at, supersedes_id)
VALUES (sqlc.arg(id), sqlc.arg(run_id), sqlc.arg(agent_id), sqlc.arg(thread_id), sqlc.arg(channel_id), sqlc.narg(space_id),
    sqlc.narg(agent_session_id), sqlc.arg(billing_mode), sqlc.arg(provider), sqlc.arg(model), sqlc.arg(tokens_in),
    sqlc.arg(tokens_out), sqlc.arg(tokens_cache_read), sqlc.arg(tokens_cache_write), sqlc.arg(tokens_cache_write_1h),
    sqlc.arg(amount_micros), sqlc.arg(currency), false, 'reconciled', sqlc.arg(occurred_at), sqlc.arg(reconciled_at),
    sqlc.arg(supersedes_id));

-- name: ReconcileRunTotals :many
-- The valid sums of the runs over all their valid entries, per run and currency (design note 6 b).
SELECT e.run_id, e.currency,
    sum(e.amount_micros)::bigint AS amount_micros,
    bool_or(e.is_estimate)::boolean AS is_estimate
FROM cost_entries e
WHERE e.run_id = ANY (sqlc.arg(run_ids)::uuid[])
  AND NOT EXISTS (SELECT 1 FROM cost_entries s WHERE s.supersedes_id = e.id)
GROUP BY e.run_id, e.currency
ORDER BY e.run_id, e.currency;

-- name: ReconcileSessionTotals :many
-- The valid sums of the agent sessions of the runs over all valid entries of each session, per
-- session and currency (design note 6 c), over the index (thread_id): a session belongs to one
-- thread.
WITH sessions AS (
    SELECT DISTINCT c.thread_id, c.agent_session_id FROM cost_entries c
    WHERE c.run_id = ANY (sqlc.arg(run_ids)::uuid[]) AND c.agent_session_id IS NOT NULL
)
SELECT e.agent_session_id::uuid AS agent_session_id, e.currency,
    sum(e.amount_micros)::bigint AS amount_micros,
    bool_or(e.is_estimate)::boolean AS is_estimate
FROM cost_entries e
JOIN sessions x ON x.thread_id = e.thread_id AND x.agent_session_id = e.agent_session_id
WHERE NOT EXISTS (SELECT 1 FROM cost_entries s WHERE s.supersedes_id = e.id)
GROUP BY e.agent_session_id, e.currency
ORDER BY e.agent_session_id, e.currency;

-- name: CostEstimates :many
-- The estimates per run of listCostEstimates (cost.reconcile.read; design note 12): entries of mode
-- organization of the agents with occurred_at in the half-open range, valid or replaced, over the
-- index (agent_id, occurred_at). estimate_micros sums the estimate entries whose successor, if any,
-- is not an estimate (the estimate before any reconciliation); reconciled holds when every valid
-- entry is reconciled. Runs whose entries in the range mix currencies are left out. Keyset by
-- run_id. Instance-level (R-AZ-28): no visible-channel predicate, no channel, thread or content.
SELECT e.run_id, e.agent_id, min(e.provider)::text AS provider, min(e.currency)::text AS currency,
    coalesce(sum(e.amount_micros) FILTER (WHERE e.is_estimate AND (s.id IS NULL OR NOT s.is_estimate)), 0)::bigint AS estimate_micros,
    coalesce(bool_and(e.source = 'reconciled') FILTER (WHERE s.id IS NULL), false)::boolean AS reconciled
FROM cost_entries e
LEFT JOIN cost_entries s ON s.supersedes_id = e.id
WHERE e.agent_id = ANY (sqlc.arg(agent_ids)::uuid[])
  AND e.occurred_at >= sqlc.arg(from_at)::timestamptz AND e.occurred_at < sqlc.arg(to_at)::timestamptz
  AND e.billing_mode = 'organization'
  AND (sqlc.narg(after_run)::uuid IS NULL OR e.run_id > sqlc.narg(after_run)::uuid)
GROUP BY e.run_id, e.agent_id
HAVING count(DISTINCT e.currency) = 1
ORDER BY e.run_id
LIMIT sqlc.arg(row_limit);
