-- SPDX-License-Identifier: Apache-2.0
-- Imported messages (M8-11; docs/spec/22-import-migration.md 6.2 and 6.4, 03 "What the import sets
-- differently"). Only the port messaging.ImportMessages runs it, inside the unit of work of one
-- import batch.

-- name: InsertImportedMessage :one
-- An imported message: origin import, silent, no trigger_state, created_at and edited_at of the
-- source, import_item_id of its item. thread_seq comes from chat.ImportThreads.Reserve under the
-- row lock. search_language goes in as text and is cast here like every insert of messages
-- (14 3.1: the instance language for an import).
INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256,
                      origin, kind, content_md, content_format, reply_to_message_id, silent, import_item_id, version,
                      edited_at, created_at, search_language, has_attachments)
VALUES (@id, @thread_id, @channel_id, @thread_seq, @author_principal_id, @client_msg_id, @client_payload_sha256,
        'import', 'text', @content_md, @content_format, sqlc.narg(reply_to_message_id), TRUE, @import_item_id, 1,
        sqlc.narg(edited_at), @created_at, sqlc.arg(search_language)::text::regconfig, @has_attachments)
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments;
