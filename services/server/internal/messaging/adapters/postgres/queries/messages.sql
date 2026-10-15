-- SPDX-License-Identifier: Apache-2.0
-- Queries on messages (messaging, docs/spec/03-data-model.md section 4.6).

-- name: InsertMessage :one
-- A new message; thread_seq comes from threads.last_message_seq under the row lock (03 section
-- 6.2). A second (author_principal_id, client_msg_id) violates messages_author_client_msg_id_key.
INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256,
                      origin, kind, system_code, content_md, content_format, reply_to_message_id, silent, version, created_at)
VALUES (@id, @thread_id, @channel_id, @thread_seq, @author_principal_id, @client_msg_id, @client_payload_sha256,
        @origin, @kind, @system_code, @content_md, @content_format, @reply_to_message_id, @silent, 1, @created_at)
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = @id;

-- name: GetMessageByClientID :one
-- The idempotency lookup of 04 section 5.1 point 4.
SELECT * FROM messages WHERE author_principal_id = @author_principal_id AND client_msg_id = @client_msg_id;
