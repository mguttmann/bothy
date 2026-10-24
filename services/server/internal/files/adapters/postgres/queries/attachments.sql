-- SPDX-License-Identifier: Apache-2.0
-- Attachments of messages (M3-07, docs/spec/03-data-model.md 4.7 message_attachments, 12 section
-- 10). files owns the table and never reads messages (03 principle 12): thread_id and channel_id
-- are the copies of the message's place (R-DM-43). The reads are keyed by the listed files, by
-- messages the caller already read or by one file id; the exceptions of AZ-P12 in
-- tools/lint/sql_visibility.yaml name their categories. The lock order is files rows in ascending
-- id before message_attachments (03 6.3).

-- name: LockAttachTargets :many
-- Attach (brief M3-07 design note 4 a): the files rows of the listed ids, deleted or not, with the
-- state of their upload session, locked FOR UPDATE OF files in ascending id. The session row is
-- not locked, because completed is final for a session (M3-03). A concurrent attach of the same
-- file and the outcome write of the scan wait for this transaction.
SELECT f.id, f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted, s.state AS session_state
FROM files f
LEFT JOIN file_upload_sessions s ON s.file_id = f.id
WHERE f.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY f.id
FOR UPDATE OF f;

-- name: AttachedFileIDs :many
-- Attach, after the lock: the listed files that hang on a message already (12 10 point 3).
SELECT file_id
FROM message_attachments
WHERE file_id = ANY(sqlc.arg(file_ids)::uuid[]);

-- name: InsertAttachments :exec
-- Attach (design note 4 d): one row per file of one message, position = index + 1, with the copies
-- of the message's thread and channel. A unique violation on (file_id) means a concurrent attach
-- committed first. The two arrays have one entry per row; set-returning functions of equal length
-- in the select list advance together.
INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id)
SELECT sqlc.arg(message_id)::uuid, unnest(sqlc.arg(file_ids)::uuid[]), unnest(sqlc.arg(positions)::smallint[]),
       sqlc.arg(thread_id)::uuid, sqlc.arg(channel_id)::uuid;

-- name: ListMessageAttachments :many
-- ForMessages (design note 6): the attachments of messages the caller already read, with their
-- files, by message and position; transcript_text for the run context (M3-09 design note 12 d).
SELECT a.message_id, a.position, a.thread_id, a.channel_id,
       f.id, f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted, f.transcript_text
FROM message_attachments a
JOIN files f ON f.id = a.file_id
WHERE a.message_id = ANY(sqlc.arg(message_ids)::uuid[])
ORDER BY a.message_id, a.position;

-- name: ListThumbnailsOfFiles :many
-- The thumbnails of the files of one Attach or ForMessages call in one statement (12 7.3); the
-- caller orders them by variant.
SELECT file_id, variant, object_key, mime_type, width, height, size_bytes
FROM file_thumbnails
WHERE file_id = ANY(sqlc.arg(file_ids)::uuid[]);

-- name: GetAttachmentByFile :one
-- Where one file is attached, by the unique key (file_id): the resource of file.read (getFile,
-- downloads), MessageOf and the target of file.updated (design notes 9 and 10).
SELECT message_id, file_id, position, thread_id, channel_id
FROM message_attachments
WHERE file_id = sqlc.arg(file_id);

-- name: LockMessageAttachmentFiles :many
-- DeleteForMessage (design note 7 b): the files of the attachments of one message FOR UPDATE OF
-- files in ascending id, before their rows go.
SELECT f.id
FROM files f
JOIN message_attachments a ON a.file_id = f.id
WHERE a.message_id = sqlc.arg(message_id)
ORDER BY f.id
FOR UPDATE OF f;

-- name: DeleteMessageAttachments :execrows
-- DeleteForMessage: the rows of the deleted message.
DELETE FROM message_attachments WHERE message_id = sqlc.arg(message_id);

-- name: ListAttachmentStates :many
-- Pending (design note 11 a; M3-09 design note 3): the state and the transcript status of every
-- attached file of one message.
SELECT f.upload_state, f.scan_status, (f.deleted_at IS NOT NULL)::boolean AS deleted, f.transcript_status
FROM message_attachments a
JOIN files f ON f.id = a.file_id
WHERE a.message_id = sqlc.arg(message_id);

-- name: MoveThreadAttachments :exec
-- The subscriber of chat.ThreadMoved (design note 8): the copies of one thread follow it to the
-- target channel, over the index (thread_id); no version, no event.
UPDATE message_attachments SET channel_id = sqlc.arg(channel_id) WHERE thread_id = sqlc.arg(thread_id);
