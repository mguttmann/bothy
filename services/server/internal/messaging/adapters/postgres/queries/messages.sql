-- SPDX-License-Identifier: Apache-2.0
-- Queries on messages (messaging, docs/spec/03-data-model.md section 4.6).
--
-- The columns are listed instead of "*": no read carries search_language or the generated
-- search_vector (docs/DECISIONS.md D-M3-13-10). Every INSERT names search_language, the language
-- of 14 3.1 (M3-13); the static test TestMessageInsertsSetSearchLanguage holds it.

-- name: InsertMessage :one
-- A new message; thread_seq comes from threads.last_message_seq under the row lock (03 section
-- 6.2). A second (author_principal_id, client_msg_id) violates messages_author_client_msg_id_key.
-- search_language goes in as text and is cast here, so pgx needs no codec for regconfig.
INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256,
                      origin, kind, system_code, content_md, content_format, reply_to_message_id, silent, version, created_at,
                      search_language)
VALUES (@id, @thread_id, @channel_id, @thread_seq, @author_principal_id, @client_msg_id, @client_payload_sha256,
        @origin, @kind, @system_code, @content_md, @content_format, @reply_to_message_id, @silent, 1, @created_at,
        sqlc.arg(search_language)::text::regconfig)
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at;

-- name: GetMessage :one
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at
FROM messages WHERE id = @id;

-- name: GetMessageByClientID :one
-- The idempotency lookup of 04 section 5.1 point 4.
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at
FROM messages WHERE author_principal_id = @author_principal_id AND client_msg_id = @client_msg_id;

-- name: LockVisibleMessage :one
-- The visible message locked for an edit or a deletion (M1-58 lock order: thread row first,
-- then the message row).
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at
FROM messages WHERE id = @id AND channel_id = ANY(@visible::uuid[]) FOR UPDATE;

-- name: UpdateMessageContent :one
-- An edit of the locked message: the new content, edited_at and the next version.
UPDATE messages SET content_md = @content_md, edited_at = @edited_at, version = version + 1
WHERE id = @id AND channel_id = ANY(@visible::uuid[]) AND deleted_at IS NULL
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at;

-- name: TombstoneMessage :one
-- The deletion of the locked message (03 section 6.5): the row and thread_seq stay, the content
-- goes, deleted_at, deleted_by and the next version are set.
UPDATE messages SET content_md = '', deleted_at = @deleted_at, deleted_by = @deleted_by, version = version + 1
WHERE id = @id AND channel_id = ANY(@visible::uuid[]) AND deleted_at IS NULL
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at;
