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
