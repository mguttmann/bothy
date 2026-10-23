-- SPDX-License-Identifier: Apache-2.0
-- Queries on messages (messaging, docs/spec/03-data-model.md section 4.6).
--
-- The columns are listed instead of "*": no read carries search_language or the generated
-- search_vector (docs/DECISIONS.md D-M3-13-10). has_attachments ends every list (M3-07), so the
-- generated row types keep converting to messageRow of repository.go. Every INSERT names search_language, the language
-- of 14 3.1 (M3-13); the static test TestMessageInsertsSetSearchLanguage holds it.

-- name: InsertMessage :one
-- A new message; thread_seq comes from threads.last_message_seq under the row lock (03 section
-- 6.2). A second (author_principal_id, client_msg_id) violates messages_author_client_msg_id_key.
-- search_language goes in as text and is cast here, so pgx needs no codec for regconfig. run_id is
-- set exactly for the live box anchor of a run (kind run, M2-05); the deferred foreign key lets
-- the anchor and its run be written in either order. has_attachments is true exactly for a message
-- sent with at least one attachment (M3-07, 03 messages); files inserts the rows of
-- message_attachments right after it in the same transaction.
INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256,
                      origin, kind, system_code, content_md, content_format, reply_to_message_id, run_id, silent, version,
                      created_at, search_language, has_attachments)
VALUES (@id, @thread_id, @channel_id, @thread_seq, @author_principal_id, @client_msg_id, @client_payload_sha256,
        @origin, @kind, @system_code, @content_md, @content_format, @reply_to_message_id, sqlc.narg(run_id), @silent, 1,
        @created_at, sqlc.arg(search_language)::text::regconfig, @has_attachments)
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments;

-- name: GetMessage :one
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments
FROM messages WHERE id = @id;

-- name: GetMessageByClientID :one
-- The idempotency lookup of 04 section 5.1 point 4.
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments
FROM messages WHERE author_principal_id = @author_principal_id AND client_msg_id = @client_msg_id;

-- name: LockVisibleMessage :one
-- The visible message locked for an edit or a deletion (M1-58 lock order: thread row first,
-- then the message row).
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments
FROM messages WHERE id = @id AND channel_id = ANY(@visible::uuid[]) FOR UPDATE;

-- name: UpdateMessageContent :one
-- An edit of the locked message: the new content, edited_at and the next version.
UPDATE messages SET content_md = @content_md, edited_at = @edited_at, version = version + 1
WHERE id = @id AND channel_id = ANY(@visible::uuid[]) AND deleted_at IS NULL
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments;

-- name: TombstoneMessage :one
-- The deletion of the locked message (03 section 6.5): the row and thread_seq stay, the content
-- goes, has_attachments goes back to false (its attachments go with it in the same transaction,
-- M3-07; the CHECK messages_has_attachments_deleted_check refuses a deleted message with the
-- flag), deleted_at, deleted_by and the next version are set. The caller read the flag from the
-- locked row before.
UPDATE messages SET content_md = '', has_attachments = false, deleted_at = @deleted_at, deleted_by = @deleted_by,
    version = version + 1
WHERE id = @id AND channel_id = ANY(@visible::uuid[]) AND deleted_at IS NULL
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments;

-- name: SetTriggerState :one
-- The trigger state of a message of a person (M2-05, 09 section 6): the three columns and the
-- next version, only when state or reason differ from the stored ones (D-M2-05-17). No row means
-- an unknown message, another origin or no change; the caller reads the message to tell them
-- apart. The row lock of this UPDATE comes after the runs row of the caller (lock order of the
-- README: runs before messages).
UPDATE messages SET trigger_state = @trigger_state, trigger_state_reason = sqlc.narg(trigger_state_reason),
    trigger_state_at = @trigger_state_at, version = version + 1
WHERE id = @id AND origin = 'human'
  AND (trigger_state IS DISTINCT FROM @trigger_state::text OR trigger_state_reason IS DISTINCT FROM sqlc.narg(trigger_state_reason)::text)
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments;

-- name: LockCardMessage :one
-- The card message of a component change locked by runs (M2-15, Messages.LockCard): runs
-- authorized the interaction or the component change on its path before, or a job or the end of a
-- run names it; the lock order is threads, runs, messages, then message_components.
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments
FROM messages WHERE id = @id FOR UPDATE;

-- name: RaiseMessageVersion :one
-- A change of the components of the locked card (M2-15, Messages.ComponentsChanged): the next
-- version, so message.updated carries a new resource version (03 messages.version "state
-- change").
UPDATE messages SET version = version + 1
WHERE id = @id
RETURNING id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments;
