-- SPDX-License-Identifier: Apache-2.0
-- Queries of the own measurement on stats_minutes (M6-13, docs/spec/03-data-model.md 4.15). Only
-- the stats module runs SQL on the table (03 principle 12).

-- name: UpsertMinutes :exec
-- Adds the counters of one flush chunk (docs/spec/25-visualization.md 3.4): one row per key, the
-- additive upsert on the unique key over all dimensions (NULLS NOT DISTINCT), so several
-- processes and nodes add into the same row without a coordinator. The uuid arrays carry NULL for
-- an absent channel or agent; the text array of the currencies carries the empty string for NULL. Set-returning
-- functions in one select list run in lock step, so element i of every array forms row i.
INSERT INTO stats_minutes (bucket_start, metric, channel_id, agent_id, dimension, currency, count, sum, good)
SELECT r.bucket_start, r.metric, r.channel_id, r.agent_id, r.dimension, NULLIF(r.currency, ''), r.count, r.sum, r.good
FROM (
    SELECT
           unnest(sqlc.arg(bucket_starts)::timestamptz[]) AS bucket_start,
           unnest(sqlc.arg(metrics)::text[]) AS metric,
           unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id,
           unnest(sqlc.arg(agent_ids)::uuid[]) AS agent_id,
           unnest(sqlc.arg(dimensions)::text[]) AS dimension,
           unnest(sqlc.arg(currencies)::text[]) AS currency,
           unnest(sqlc.arg(counts)::bigint[]) AS count,
           unnest(sqlc.arg(sums)::bigint[]) AS sum,
           unnest(sqlc.arg(goods)::bigint[]) AS good
) r
ON CONFLICT (bucket_start, metric, channel_id, agent_id, dimension, currency) DO UPDATE
SET count = stats_minutes.count + excluded.count,
    sum = stats_minutes.sum + excluded.sum,
    good = stats_minutes.good + excluded.good,
    updated_at = now();

-- name: EnsurePartition :one
-- Creates the partition of one UTC day through the SECURITY DEFINER function of bothy_owner.
SELECT stats_minutes_ensure_partition(sqlc.arg(day)::date)::int;

-- name: DropExpiredPartitions :one
-- Detaches and drops the expired partitions through the SECURITY DEFINER function of bothy_owner.
SELECT stats_minutes_drop_expired(sqlc.arg(cutoff)::timestamptz, sqlc.arg(lock_timeout)::interval)::int;

-- name: LiveRates :many
-- The rates of the live frame (M6-15, docs/spec/25-visualization.md 5.2): the counters of the last
-- full and the current minute without the sli.* metrics, summed over agents and nodes, per
-- channel (NULL for the rows without a channel), dimension and currency. The live builder filters
-- them per receiver; the table holds no person and no content.
SELECT bucket_start, metric, channel_id, dimension, coalesce(currency, '')::text AS currency,
       sum(count)::bigint AS count, sum(sum)::bigint AS sum
FROM stats_minutes
WHERE bucket_start IN (sqlc.arg(last_minute)::timestamptz, sqlc.arg(current_minute)::timestamptz)
  AND metric NOT LIKE 'sli.%'
GROUP BY bucket_start, metric, channel_id, dimension, currency;

-- name: LiveSLI :many
-- The SLI totals of the live frame (M6-15): count and good of every sli.* metric over the minutes
-- from the start of stats.live_sli_window up to the current minute, summed over all keys.
SELECT metric, sum(count)::bigint AS total, sum(good)::bigint AS good
FROM stats_minutes
WHERE metric LIKE 'sli.%'
  AND bucket_start >= sqlc.arg(from_minute)::timestamptz AND bucket_start <= sqlc.arg(to_minute)::timestamptz
GROUP BY metric;

-- name: SLITotals :many
-- The sums of one metric over a window of minute buckets per dimension, over every channel and
-- agent: the automatic checks of the status page (M7-09). The index (metric, bucket_start)
-- serves it, and the partitions by day prune the scan to the window.
SELECT dimension, sum(count)::bigint AS count, sum(sum)::bigint AS sum, sum(good)::bigint AS good
FROM stats_minutes
WHERE metric = sqlc.arg(metric) AND bucket_start >= sqlc.arg(window_from) AND bucket_start < sqlc.arg(window_to)
GROUP BY dimension
ORDER BY dimension;

-- name: TimelineRows :many
-- The minute rows of the timeline (M6-14, docs/spec/25-visualization.md 6.5): every row of the
-- requested metrics with bucket_start in [from_at, to_at) in the channel set the use case built
-- from the caller's Scope, plus the rows without a channel only when with_instance is set (an
-- owner on the instance, 25 section 4 rule 1). The use case leaves cost out of metrics without
-- cost.read.aggregate. The table holds no person and no content.
SELECT bucket_start, metric, channel_id, agent_id, dimension, coalesce(currency, '')::text AS currency, count, sum, good
FROM stats_minutes
WHERE bucket_start >= sqlc.arg(from_at)::timestamptz AND bucket_start < sqlc.arg(to_at)::timestamptz
  AND metric = ANY (sqlc.arg(metrics)::text[])
  AND (channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) OR (channel_id IS NULL AND sqlc.arg(with_instance)::boolean))
ORDER BY bucket_start, metric, channel_id NULLS FIRST, agent_id NULLS FIRST, dimension, currency NULLS FIRST;

-- name: BackendRows :many
-- The sums of the backend view per agent (M6-14, 25 6.2) over bucket_start in [from_at, to_at):
-- the billing mode switches of the agents (no channel; the use case passes only agents of the
-- visible channels), the denials of I2 and I3 in the channel set and, only with with_cost, the
-- cost rows in the channel set, per metric, dimension and currency.
SELECT agent_id, metric, dimension, coalesce(currency, '')::text AS currency, sum(count)::bigint AS count, sum(sum)::bigint AS sum
FROM stats_minutes
WHERE bucket_start >= sqlc.arg(from_at)::timestamptz AND bucket_start < sqlc.arg(to_at)::timestamptz
  AND agent_id = ANY (sqlc.arg(agent_ids)::uuid[])
  AND (metric = 'billing_mode_switches'
    OR (metric = 'invocations_denied' AND channel_id = ANY (sqlc.arg(channel_ids)::uuid[]))
    OR (metric = 'cost' AND sqlc.arg(with_cost)::boolean AND channel_id = ANY (sqlc.arg(channel_ids)::uuid[])))
GROUP BY agent_id, metric, dimension, currency
ORDER BY agent_id, metric, dimension, currency;
