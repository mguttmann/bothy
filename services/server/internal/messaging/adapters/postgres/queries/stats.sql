-- SPDX-License-Identifier: Apache-2.0
-- The statistics reads of messaging (M6-05, messaging.StatsSource). Each read takes the channel set
-- the stats use case built from Scope of the caller (07 section 7.4) and never widens it; group_ids
-- is parallel to channel_ids (the space of each channel). Only counts leave: active people are
-- counted with count(DISTINCT author_principal_id) here, so no person ID leaves the module.

-- name: MessageStats :many
-- The messages not deleted by origin (system lines are not counted) per bucket and group, and the
-- distinct people with a message of origin human when count_people is set. date_trunc with a time
-- zone starts day, week (ISO, Monday) and month buckets at local midnight (admin-statistics.md 5).
SELECT date_trunc(sqlc.arg(granularity)::text, m.created_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    g.group_id::uuid AS group_id,
    (count(*) FILTER (WHERE m.origin = 'human'))::bigint AS human,
    (count(*) FILTER (WHERE m.origin = 'agent'))::bigint AS agent,
    (count(*) FILTER (WHERE m.origin = 'app'))::bigint AS app,
    (count(*) FILTER (WHERE m.origin = 'import'))::bigint AS import,
    (CASE WHEN sqlc.arg(count_people)::boolean
        THEN count(DISTINCT m.author_principal_id) FILTER (WHERE m.origin = 'human') ELSE 0 END)::bigint AS people
FROM messages m
JOIN (SELECT unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
    ON g.channel_id = m.channel_id
WHERE m.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND m.deleted_at IS NULL AND m.origin <> 'system'
  AND m.created_at >= sqlc.arg(from_at) AND m.created_at < sqlc.arg(to_at)
GROUP BY 1, 2
ORDER BY 1, 2;

-- name: InvocationDenialStats :many
-- The messages whose trigger was refused with billing_mode_personal_not_holder (10 section 7) per
-- bucket of their creation and group.
SELECT date_trunc(sqlc.arg(granularity)::text, m.created_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    g.group_id::uuid AS group_id, count(*)::bigint AS value
FROM messages m
JOIN (SELECT unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
    ON g.channel_id = m.channel_id
WHERE m.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND m.trigger_state_reason = 'billing_mode_personal_not_holder'
  AND m.created_at >= sqlc.arg(from_at) AND m.created_at < sqlc.arg(to_at)
GROUP BY 1, 2
ORDER BY 1, 2;
