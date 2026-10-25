-- SPDX-License-Identifier: Apache-2.0
-- The statistics read of apps (M6-05, apps.StatsSource): webhook deliveries have no channel, so
-- the stats use case calls it only for stats.read on the instance by an owner. The table is
-- transient: only deliveries still within their retention count.

-- name: WebhookStats :many
-- The finished deliveries (succeeded, failed, dead) and the failed ones (failed, dead) per bucket
-- of created_at.
SELECT date_trunc(sqlc.arg(granularity)::text, d.created_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    (count(*) FILTER (WHERE d.status IN ('succeeded', 'failed', 'dead')))::bigint AS finished,
    (count(*) FILTER (WHERE d.status IN ('failed', 'dead')))::bigint AS failed
FROM webhook_deliveries d
WHERE d.created_at >= sqlc.arg(from_at) AND d.created_at < sqlc.arg(to_at)
GROUP BY 1
ORDER BY 1;
