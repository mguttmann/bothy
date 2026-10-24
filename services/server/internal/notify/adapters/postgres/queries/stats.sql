-- SPDX-License-Identifier: Apache-2.0
-- The statistics read of notify (M6-05, notify.StatsSource): push registrations have no channel,
-- so the stats use case calls it only for stats.read on the instance by an owner. Only counts per
-- provider leave, never a person, device or token.

-- name: PushStats :many
-- Per bucket (starts and ends are parallel) and provider: the registrations active at some time in
-- the bucket (created before its end, not disabled before its start) and those disabled in it for
-- a provider reason; logout and device_revoked are no failures.
SELECT b.start_at::timestamptz AS bucket, p.provider,
    count(*)::bigint AS registrations,
    (count(*) FILTER (WHERE p.disabled_at >= b.start_at AND p.disabled_at < b.end_at
        AND p.disabled_reason IN ('token_invalid', 'topic_mismatch', 'sender_mismatch', 'repeated_failures')))::bigint AS disabled
FROM (SELECT unnest(sqlc.arg(starts)::timestamptz[]) AS start_at, unnest(sqlc.arg(ends)::timestamptz[]) AS end_at) b
JOIN push_registrations p ON p.created_at < b.end_at AND (p.disabled_at IS NULL OR p.disabled_at >= b.start_at)
GROUP BY 1, 2
ORDER BY 1, 2;
