-- SPDX-License-Identifier: Apache-2.0
-- The history of a thread over thread_seq (messaging, M1-57; docs/spec/06-api.md section 6 item 5,
-- docs/spec/04-sync-protokoll.md section 4). Each statement is one snapshot under READ COMMITTED:
-- the page rows of each side with LIMIT @row_limit (the page size plus one, so a full side shows
-- that more exist) and EXISTS for the opposite side (brief note 4). The one-row flags relation f
-- is left joined with the rows, so an empty page still returns its flag in one row whose message
-- columns are NULL. before_side marks the rows below the anchor (read descending). Every read of
-- messages carries the visible-channel predicate (07 section 7.4 rule 1) and the thread, so
-- PostgreSQL walks messages_thread_seq_key.

-- name: ListThreadNewest :many
-- The newest page: the @row_limit highest thread_seq.
WITH page AS (
    SELECT m.* FROM messages m
    WHERE m.thread_id = @thread_id AND m.channel_id = ANY(@visible_channel_ids::uuid[])
    ORDER BY m.thread_seq DESC
    LIMIT @row_limit
)
SELECT p.id, p.thread_id, p.channel_id, p.thread_seq, p.author_principal_id, p.client_msg_id, p.origin, p.kind,
       p.system_code, p.content_md, p.content_format, p.reply_to_message_id, p.silent, p.version, p.edited_at,
       p.deleted_at, p.deleted_by, p.created_at, true AS before_side, false AS opposite
FROM (SELECT 1) f LEFT JOIN page p ON true
ORDER BY p.thread_seq;

-- name: ListThreadBefore :many
-- before_seq = @anchor: the @row_limit highest thread_seq below the anchor; opposite is whether a
-- message at or above the anchor exists.
WITH page AS (
    SELECT m.* FROM messages m
    WHERE m.thread_id = @thread_id AND m.channel_id = ANY(@visible_channel_ids::uuid[]) AND m.thread_seq < @anchor
    ORDER BY m.thread_seq DESC
    LIMIT @row_limit
), f AS (
    SELECT EXISTS (
        SELECT 1 FROM messages o
        WHERE o.thread_id = @thread_id AND o.channel_id = ANY(@visible_channel_ids::uuid[]) AND o.thread_seq >= @anchor
    ) AS opposite
)
SELECT p.id, p.thread_id, p.channel_id, p.thread_seq, p.author_principal_id, p.client_msg_id, p.origin, p.kind,
       p.system_code, p.content_md, p.content_format, p.reply_to_message_id, p.silent, p.version, p.edited_at,
       p.deleted_at, p.deleted_by, p.created_at, true AS before_side, f.opposite
FROM f LEFT JOIN page p ON true
ORDER BY p.thread_seq;

-- name: ListThreadAfter :many
-- after_seq = @anchor: the @row_limit lowest thread_seq above the anchor; opposite is whether a
-- message at or below the anchor exists.
WITH page AS (
    SELECT m.* FROM messages m
    WHERE m.thread_id = @thread_id AND m.channel_id = ANY(@visible_channel_ids::uuid[]) AND m.thread_seq > @anchor
    ORDER BY m.thread_seq
    LIMIT @row_limit
), f AS (
    SELECT EXISTS (
        SELECT 1 FROM messages o
        WHERE o.thread_id = @thread_id AND o.channel_id = ANY(@visible_channel_ids::uuid[]) AND o.thread_seq <= @anchor
    ) AS opposite
)
SELECT p.id, p.thread_id, p.channel_id, p.thread_seq, p.author_principal_id, p.client_msg_id, p.origin, p.kind,
       p.system_code, p.content_md, p.content_format, p.reply_to_message_id, p.silent, p.version, p.edited_at,
       p.deleted_at, p.deleted_by, p.created_at, false AS before_side, f.opposite
FROM f LEFT JOIN page p ON true
ORDER BY p.thread_seq;

-- name: ListThreadAround :many
-- around_seq = @anchor: up to @row_limit rows below the anchor (descending) and up to @row_limit
-- from the anchor on (ascending); the use case picks the window (domain.PickAround).
WITH below AS (
    SELECT m.* FROM messages m
    WHERE m.thread_id = @thread_id AND m.channel_id = ANY(@visible_channel_ids::uuid[]) AND m.thread_seq < @anchor
    ORDER BY m.thread_seq DESC
    LIMIT @row_limit
), above AS (
    SELECT m.* FROM messages m
    WHERE m.thread_id = @thread_id AND m.channel_id = ANY(@visible_channel_ids::uuid[]) AND m.thread_seq >= @anchor
    ORDER BY m.thread_seq
    LIMIT @row_limit
), page AS (
    SELECT b.*, true AS before_side FROM below b
    UNION ALL
    SELECT a.*, false AS before_side FROM above a
)
SELECT p.id, p.thread_id, p.channel_id, p.thread_seq, p.author_principal_id, p.client_msg_id, p.origin, p.kind,
       p.system_code, p.content_md, p.content_format, p.reply_to_message_id, p.silent, p.version, p.edited_at,
       p.deleted_at, p.deleted_by, p.created_at, coalesce(p.before_side, false)::boolean AS before_side, false AS opposite
FROM (SELECT 1) f LEFT JOIN page p ON true
ORDER BY p.thread_seq;

-- name: GetVisibleMessage :one
-- getMessage: the message @id when its channel is visible (07 section 7.4 rule 1); a foreign and
-- an unknown id both give no row.
SELECT * FROM messages m WHERE m.id = @id AND m.channel_id = ANY(@visible_channel_ids::uuid[]);
