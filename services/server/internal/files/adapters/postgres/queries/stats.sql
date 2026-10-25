-- SPDX-License-Identifier: Apache-2.0
-- The statistics reads of files (M6-05, files.StatsSource). Only sums and counts leave, never a
-- file name, uploader or content.

-- name: StorageStats :many
-- The current size of the complete attachments not deleted per group: scope_channel_id is one of
-- the channel set the stats use case built from Scope of the caller, group_ids parallel to it.
SELECT g.group_id::uuid AS group_id, sum(f.size_bytes)::bigint AS value
FROM files f
JOIN (SELECT unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
    ON g.channel_id = f.scope_channel_id
WHERE f.scope_channel_id = ANY (sqlc.arg(channel_ids)::uuid[]) AND f.deleted_at IS NULL AND f.upload_state = 'complete'
GROUP BY 1
ORDER BY 1;

-- name: ScanStats :many
-- The scans that ended with error or infected per bucket of scanned_at, over every file: the stats
-- use case calls it only for stats.read on the instance by an owner (values without a channel).
SELECT date_trunc(sqlc.arg(granularity)::text, f.scanned_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    (count(*) FILTER (WHERE f.scan_status = 'error'))::bigint AS errors,
    (count(*) FILTER (WHERE f.scan_status = 'infected'))::bigint AS infected
FROM files f
WHERE f.scan_status IN ('error', 'infected') AND f.scanned_at >= sqlc.arg(from_at) AND f.scanned_at < sqlc.arg(to_at)
GROUP BY 1
ORDER BY 1;
