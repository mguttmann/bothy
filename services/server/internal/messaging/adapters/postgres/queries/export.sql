-- SPDX-License-Identifier: Apache-2.0
-- The channel export (messaging, M3-11; docs/api/openapi.yaml exportChannel, brief note 5): the
-- messages of one channel with created_at in the half-open range [@from_at, @to_at), threads
-- ascending by their UUIDv7 id and the messages of a thread ascending by thread_seq, read as a
-- keyset over (thread_id, thread_seq) in pages of @row_limit (api.page_size_max). Every statement
-- carries the channel and the visible-channel predicate (07 section 7.4 rule 1), so a channel the
-- caller cannot see yields no row. No new index: the thread page walks messages by
-- (channel_id, created_at DESC), the message page the unique (thread_id, thread_seq).

-- name: ListExportThreadIDs :many
-- The next @row_limit threads of the channel after @after_thread_id that have a message in the
-- range.
SELECT DISTINCT m.thread_id
FROM messages m
WHERE m.channel_id = @channel_id AND m.channel_id = ANY(@visible_channel_ids::uuid[])
  AND m.created_at >= @from_at AND m.created_at < @to_at AND m.thread_id > @after_thread_id
ORDER BY m.thread_id
LIMIT @row_limit;

-- name: ListExportThreadMessages :many
-- The next @row_limit messages of the thread in the range after @after_seq, tombstones included
-- (they keep their thread_seq). The columns are those of GetVisibleMessage, without
-- search_language and search_vector.
SELECT m.id, m.thread_id, m.channel_id, m.thread_seq, m.author_principal_id, m.client_msg_id, m.client_payload_sha256, m.origin,
    m.kind, m.system_code, m.content_md, m.content_format, m.reply_to_message_id, m.run_id, m.trigger_state,
    m.trigger_state_reason, m.trigger_state_at, m.silent, m.import_item_id, m.version, m.edited_at, m.deleted_at, m.deleted_by,
    m.created_at, m.has_attachments
FROM messages m
WHERE m.thread_id = @thread_id AND m.channel_id = @channel_id AND m.channel_id = ANY(@visible_channel_ids::uuid[])
  AND m.created_at >= @from_at AND m.created_at < @to_at AND m.thread_seq > @after_seq
ORDER BY m.thread_seq
LIMIT @row_limit;
