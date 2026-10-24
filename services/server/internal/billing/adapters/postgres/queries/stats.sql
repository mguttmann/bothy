-- SPDX-License-Identifier: Apache-2.0
-- The statistics reads of billing (M6-05, billing.StatsSource): valid entries (no other entry
-- supersedes them) in the channel set the stats use case built from Scope of the caller; the set is
-- never widened. show_personal is billing.show_personal_estimates (10 section 12.2).

-- name: CostStats :many
-- The valid sums per bucket of occurred_at, group (space, agent or none), billing mode and
-- currency; personal and organization are separate rows and never summed together.
SELECT date_trunc(sqlc.arg(granularity)::text, e.occurred_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    (CASE sqlc.arg(group_by)::text
        WHEN 'space' THEN coalesce(e.space_id::text, '')
        WHEN 'agent' THEN e.agent_id::text
        ELSE '' END)::text AS group_key,
    e.billing_mode, e.currency, sum(e.amount_micros)::bigint AS amount_micros, bool_or(e.is_estimate)::boolean AS is_estimate
FROM cost_entries e
WHERE e.channel_id = ANY (sqlc.arg(channel_ids)::uuid[])
  AND e.occurred_at >= sqlc.arg(from_at) AND e.occurred_at < sqlc.arg(to_at)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR e.agent_id = sqlc.narg(agent_id)::uuid)
  AND (sqlc.arg(currency)::text = '' OR e.currency = sqlc.arg(currency)::text)
  AND (sqlc.arg(show_personal)::boolean OR e.billing_mode <> 'personal')
  AND NOT EXISTS (SELECT 1 FROM cost_entries s WHERE s.supersedes_id = e.id)
GROUP BY 1, 2, 3, 4
ORDER BY 1, 2, 3, 4;

-- name: CostCurrencies :many
-- The currencies of the valid entries CostStats sums without a currency filter.
SELECT DISTINCT e.currency
FROM cost_entries e
WHERE e.channel_id = ANY (sqlc.arg(channel_ids)::uuid[])
  AND e.occurred_at >= sqlc.arg(from_at) AND e.occurred_at < sqlc.arg(to_at)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR e.agent_id = sqlc.narg(agent_id)::uuid)
  AND (sqlc.arg(show_personal)::boolean OR e.billing_mode <> 'personal')
  AND NOT EXISTS (SELECT 1 FROM cost_entries s WHERE s.supersedes_id = e.id)
ORDER BY 1;
