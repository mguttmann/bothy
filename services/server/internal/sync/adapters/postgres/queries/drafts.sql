-- SPDX-License-Identifier: Apache-2.0
-- Drafts (M1-61; 03 drafts, 04 section 14). Every statement names the person (user_id). The
-- writes upsert on the partial unique indexes, so two concurrent first writes do not fail: the
-- second updates the row the first inserted. A returned version 1 is a new row; a larger one is
-- the version after the update (the row had version - 1 under its lock).

-- name: UpsertThreadDraft :one
INSERT INTO drafts (id, user_id, thread_id, channel_id, content_md, reply_to_message_id, file_ids, version,
                    updated_by_device_id, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(thread_id), sqlc.arg(channel_id), sqlc.arg(content_md),
        sqlc.narg(reply_to_message_id), sqlc.arg(file_ids)::uuid[], 1, sqlc.narg(updated_by_device_id), sqlc.arg(updated_at))
ON CONFLICT (user_id, thread_id) WHERE thread_id IS NOT NULL DO UPDATE
SET channel_id = EXCLUDED.channel_id, content_md = EXCLUDED.content_md, reply_to_message_id = EXCLUDED.reply_to_message_id,
    file_ids = EXCLUDED.file_ids, version = drafts.version + 1, updated_by_device_id = EXCLUDED.updated_by_device_id,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: UpsertChannelDraft :one
INSERT INTO drafts (id, user_id, thread_id, channel_id, content_md, reply_to_message_id, file_ids, version,
                    updated_by_device_id, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), NULL, sqlc.arg(channel_id), sqlc.arg(content_md), NULL, sqlc.arg(file_ids)::uuid[], 1,
        sqlc.narg(updated_by_device_id), sqlc.arg(updated_at))
ON CONFLICT (user_id, channel_id) WHERE thread_id IS NULL DO UPDATE
SET content_md = EXCLUDED.content_md, file_ids = EXCLUDED.file_ids, version = drafts.version + 1,
    updated_by_device_id = EXCLUDED.updated_by_device_id, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteThreadDraft :one
DELETE FROM drafts WHERE user_id = sqlc.arg(user_id) AND thread_id = sqlc.arg(thread_id) RETURNING *;

-- name: DeleteChannelDraft :one
DELETE FROM drafts WHERE user_id = sqlc.arg(user_id) AND channel_id = sqlc.arg(channel_id) AND thread_id IS NULL RETURNING *;

-- name: ListDraftsInChannels :many
-- The drafts of the person in the visible channels; drafts of lost channels stay stored.
SELECT * FROM drafts WHERE user_id = sqlc.arg(user_id) AND channel_id = ANY(sqlc.arg(channel_ids)::uuid[]) ORDER BY id;

-- name: HeldByUploaderDrafts :many
-- files.DraftFileSource (M3-08, docs/spec/12-files.md section 12 row "Never attached"): the files
-- of the list that a draft of their own uploader holds. The two arrays have one entry per file in
-- the same order (set-returning functions of equal length in the select list advance together);
-- the partial unique indexes of drafts lead with user_id, so no index is added. A file ID in the
-- draft of another principal never matches.
SELECT DISTINCT u.file_id::uuid AS file_id
FROM (SELECT unnest(sqlc.arg(file_ids)::uuid[]) AS file_id, unnest(sqlc.arg(uploader_ids)::uuid[]) AS uploader_id) AS u
JOIN drafts d ON d.user_id = u.uploader_id
WHERE u.file_id = ANY(d.file_ids);
