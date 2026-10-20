-- SPDX-License-Identifier: Apache-2.0
-- Queries on mentions (messaging, docs/spec/03-data-model.md section 4.6).

-- name: InsertMentions :exec
-- One row per resolved principal of a message, in one statement (brief M1-56 D9).
INSERT INTO mentions (message_id, principal_id, thread_id, created_at)
SELECT @message_id, p, @thread_id, @created_at
FROM unnest(@principal_ids::uuid[]) AS p;

-- name: ListMentions :many
-- The resolved principals of a message, sorted (Message.mention_ids).
SELECT principal_id FROM mentions WHERE message_id = @message_id ORDER BY principal_id;

-- name: ListMyMentions :many
-- One page of the mentions of a principal in visible channels without tombstones, newest first
-- (brief M1-60 note 2): keyset (mn.created_at DESC, mn.message_id DESC) after the cursor, index
-- mentions_principal_created_idx. The caller asks for one row more than the page. The message
-- columns are listed, so the read carries neither search_language nor search_vector (D-M3-13-10).
SELECT m.id, m.thread_id, m.channel_id, m.thread_seq, m.author_principal_id, m.client_msg_id,
    m.client_payload_sha256, m.origin, m.kind, m.system_code, m.content_md, m.content_format,
    m.reply_to_message_id, m.run_id, m.trigger_state, m.trigger_state_reason, m.trigger_state_at, m.silent,
    m.import_item_id, m.version, m.edited_at, m.deleted_at, m.deleted_by, m.created_at,
    mn.created_at AS mentioned_at
FROM mentions mn
JOIN messages m ON m.id = mn.message_id
WHERE mn.principal_id = @principal_id
  AND m.channel_id = ANY (@visible_channel_ids::uuid[])
  AND m.deleted_at IS NULL
  AND (NOT @has_after::boolean OR (mn.created_at, mn.message_id) < (@after_at::timestamptz, @after_id::uuid))
ORDER BY mn.created_at DESC, mn.message_id DESC
LIMIT @row_limit;

-- name: CountUnreadMentions :many
-- The unread mentions of a principal per thread (brief M1-60 note 4): messages above the read
-- sequence of their thread (0 for a thread outside the arrays), without tombstones, only in
-- visible channels. The read sequences come as two parallel arrays of equal length, zipped by
-- the two unnest calls of one select list. Threads with no unread
-- mention are absent.
SELECT m.thread_id, m.channel_id, count(*)::int AS mention_count
FROM mentions mn
JOIN messages m ON m.id = mn.message_id
LEFT JOIN (SELECT unnest(@read_thread_ids::uuid[]) AS thread_id, unnest(@read_seqs::bigint[]) AS last_read_seq) AS r
    ON r.thread_id = m.thread_id
WHERE mn.principal_id = @principal_id
  AND m.channel_id = ANY (@visible_channel_ids::uuid[])
  AND m.deleted_at IS NULL
  AND m.thread_seq > coalesce(r.last_read_seq, 0)
GROUP BY m.thread_id, m.channel_id
ORDER BY m.thread_id;
