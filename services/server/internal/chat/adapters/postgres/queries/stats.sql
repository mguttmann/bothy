-- SPDX-License-Identifier: Apache-2.0
-- The statistics reads of chat (M6-05, chat.StatsSource): the structure the statistics group by
-- and the thread counts. Each read takes the channel set the stats use case built from Scope of the
-- caller (07 section 7.4) and never widens it; nothing but IDs and counts leaves.

-- name: StatsChannels :many
-- The channels of the set that belong to a space, with the space; DM channels have none.
SELECT c.id, c.space_id::uuid AS space_id
FROM channels c
WHERE c.id = ANY (sqlc.arg(channel_ids)::uuid[]) AND c.space_id IS NOT NULL
ORDER BY c.id;

-- name: StatsThreads :many
-- The threads of the channel set created before the end of the range, with their current channel:
-- the runs of these threads count for the statistics (runs has no channel_id, D-M2-05-9).
SELECT t.id, t.channel_id
FROM threads t
WHERE t.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND t.created_at < sqlc.arg(before_at)
ORDER BY t.id;

-- name: StatsRunChats :many
-- The chats of the threads of runs in the channel set: the channel name and the thread title the
-- sidebar shows (run labels of the live topology and the timeline, 25 4 rule 6); the main thread
-- has no title of its own. The stats use cases pass the names only to a caller who may read the
-- thread.
SELECT t.id, t.channel_id, c.name AS channel_name, (CASE WHEN t.kind = 'main' THEN '' ELSE t.title END)::text AS thread_title
FROM threads t
JOIN channels c ON c.id = t.channel_id
WHERE t.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND t.id = ANY (sqlc.arg(thread_ids)::uuid[])
ORDER BY t.id;

-- name: ThreadStats :many
-- threads_created (threads of kind thread by created_at) and threads_done (threads still closed, by
-- closed_at) per bucket and group; group_ids is parallel to channel_ids. date_trunc with a time
-- zone starts day, week (ISO, Monday) and month buckets at local midnight (admin-statistics.md 5).
SELECT s.bucket::timestamptz AS bucket, s.group_id::uuid AS group_id, s.metric::text AS metric, count(*)::bigint AS value
FROM (
    SELECT date_trunc(sqlc.arg(granularity)::text, t.created_at, sqlc.arg(time_zone)::text) AS bucket, g.group_id,
        'threads_created' AS metric
    FROM threads t
    JOIN (SELECT unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
        ON g.channel_id = t.channel_id
    WHERE t.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND t.kind = 'thread'
      AND t.created_at >= sqlc.arg(from_at) AND t.created_at < sqlc.arg(to_at)
    UNION ALL
    SELECT date_trunc(sqlc.arg(granularity)::text, t.closed_at, sqlc.arg(time_zone)::text) AS bucket, g.group_id,
        'threads_done' AS metric
    FROM threads t
    JOIN (SELECT unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
        ON g.channel_id = t.channel_id
    WHERE t.channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND t.state = 'closed'
      AND t.closed_at >= sqlc.arg(from_at) AND t.closed_at < sqlc.arg(to_at)
) s
GROUP BY s.bucket, s.group_id, s.metric
ORDER BY s.bucket, s.group_id, s.metric;
