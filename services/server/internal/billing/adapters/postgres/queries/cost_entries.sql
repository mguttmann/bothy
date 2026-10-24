-- SPDX-License-Identifier: Apache-2.0
-- The cost entries of the billing module (M2-22, 03 section 4.9). cost_entries is no content table
-- (class cost); every read is named by a thread whose cost.read.thread authorize allowed, by a
-- price row of an owner's closePrice, or by a level and the visible channels the caller of
-- Totals passes (channel_id = ANY).

-- name: InsertCostEntry :exec
-- One entry of a booking: an estimate without reconciliation (10 section 12.1).
INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id, billing_mode, provider,
    model, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, tokens_cache_write_1h, amount_micros, currency,
    is_estimate, source, occurred_at)
VALUES (sqlc.arg(id), sqlc.arg(run_id), sqlc.arg(agent_id), sqlc.arg(thread_id), sqlc.arg(channel_id), sqlc.narg(space_id),
    sqlc.narg(agent_session_id), sqlc.arg(billing_mode), sqlc.arg(provider), sqlc.arg(model), sqlc.arg(tokens_in),
    sqlc.arg(tokens_out), sqlc.arg(tokens_cache_read), sqlc.arg(tokens_cache_write), sqlc.arg(tokens_cache_write_1h),
    sqlc.arg(amount_micros), sqlc.arg(currency), true, sqlc.arg(source), sqlc.arg(occurred_at));

-- name: ThreadEntries :many
-- Every entry of one thread, superseded ones included, over the index (thread_id).
SELECT * FROM cost_entries WHERE thread_id = sqlc.arg(thread_id) ORDER BY id;

-- name: Totals :many
-- The valid sums of one level per billing mode and currency (M2-22 design note 7): entries no
-- other entry supersedes, in the half-open period on occurred_at and in the visible channels.
SELECT e.billing_mode, e.currency,
    sum(e.amount_micros)::bigint AS amount_micros,
    bool_or(e.is_estimate)::boolean AS is_estimate,
    sum(e.tokens_in)::bigint AS tokens_in,
    sum(e.tokens_out)::bigint AS tokens_out,
    sum(e.tokens_cache_read)::bigint AS tokens_cache_read,
    sum(e.tokens_cache_write)::bigint AS tokens_cache_write,
    sum(e.tokens_cache_write_1h)::bigint AS tokens_cache_write_1h,
    count(*)::bigint AS entries
FROM cost_entries e
WHERE CASE sqlc.arg(level)::text
        WHEN 'run' THEN e.run_id = sqlc.arg(id)::uuid
        WHEN 'thread' THEN e.thread_id = sqlc.arg(id)::uuid
        WHEN 'session' THEN e.agent_session_id = sqlc.arg(id)::uuid
        WHEN 'agent' THEN e.agent_id = sqlc.arg(id)::uuid
        WHEN 'space' THEN e.space_id = sqlc.arg(id)::uuid
        WHEN 'instance' THEN true
        ELSE false
      END
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR e.occurred_at >= sqlc.narg(from_at)::timestamptz)
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR e.occurred_at < sqlc.narg(to_at)::timestamptz)
  AND (sqlc.arg(all_channels)::boolean OR e.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]))
  AND NOT EXISTS (SELECT 1 FROM cost_entries s WHERE s.supersedes_id = e.id)
GROUP BY e.billing_mode, e.currency
ORDER BY e.billing_mode, e.currency;

-- name: PriceInUse :one
-- A computed entry, valid or superseded, of the row's provider and model at or after the row's
-- valid_from and at or after the new end with a count above 0 for the row's unit (M2-22 design
-- note 12 c). A computed entry is never updated, so it stays is_estimate and the partial index
-- (is_estimate, occurred_at) WHERE is_estimate serves this query.
SELECT EXISTS (
    SELECT 1 FROM cost_entries
    WHERE is_estimate AND source = 'computed'
      AND provider = sqlc.arg(provider) AND model = sqlc.arg(model)
      AND occurred_at >= sqlc.arg(valid_from)::timestamptz AND occurred_at >= sqlc.arg(valid_to)::timestamptz
      AND CASE sqlc.arg(unit)::text
            WHEN 'input_mtok' THEN tokens_in
            WHEN 'output_mtok' THEN tokens_out
            WHEN 'cache_read_mtok' THEN tokens_cache_read
            WHEN 'cache_write_mtok' THEN tokens_cache_write
            WHEN 'cache_write_1h_mtok' THEN tokens_cache_write_1h
            ELSE 0
          END > 0
)::boolean AS in_use;
