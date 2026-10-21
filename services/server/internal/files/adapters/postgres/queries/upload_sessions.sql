-- SPDX-License-Identifier: Apache-2.0
-- file_upload_sessions (M3-03, 03 4.7). Reads are keyed by the file id and the uploader.

-- name: InsertSession :exec
INSERT INTO file_upload_sessions (file_id, mode, state, declared_size_bytes, declared_sha256, part_size_bytes,
                                  part_count, created_at, expires_at)
VALUES (sqlc.arg(file_id), sqlc.arg(mode), sqlc.arg(state), sqlc.arg(declared_size_bytes), sqlc.arg(declared_sha256),
        sqlc.arg(part_size_bytes), sqlc.arg(part_count), sqlc.arg(created_at), sqlc.arg(expires_at));

-- name: GetSession :one
-- The session of one file of the uploader, with the file, when the file is not deleted.
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at AS file_created_at
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id) AND f.uploaded_by = sqlc.arg(uploaded_by) AND f.deleted_at IS NULL;

-- name: LockSessionForUpdate :one
-- GetSession with the session row locked FOR UPDATE (abort and completion, design notes 12 and 13).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at AS file_created_at
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id) AND f.uploaded_by = sqlc.arg(uploaded_by) AND f.deleted_at IS NULL
FOR UPDATE OF s;

-- name: LockSessionForShare :one
-- GetSession with the session row locked FOR SHARE (the confirmation of a part, design note 9 h).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at AS file_created_at
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id) AND f.uploaded_by = sqlc.arg(uploaded_by) AND f.deleted_at IS NULL
FOR SHARE OF s;

-- name: CountOpenSessions :one
-- The open sessions of the uploader that have not reached their expiry, and the earliest expiry.
SELECT count(*)::int AS open, coalesce(min(s.expires_at), 'epoch'::timestamptz)::timestamptz AS earliest
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE f.uploaded_by = sqlc.arg(uploaded_by) AND s.state = 'open' AND s.expires_at > sqlc.arg(now);

-- name: SetStorageUploadID :execrows
UPDATE file_upload_sessions SET storage_upload_id = sqlc.arg(storage_upload_id)
WHERE file_id = sqlc.arg(file_id) AND storage_upload_id IS NULL AND state = 'open';

-- name: CompleteSession :execrows
UPDATE file_upload_sessions SET state = 'completed', completed_at = sqlc.arg(completed_at)
WHERE file_id = sqlc.arg(file_id) AND state = 'open';

-- name: AbortSession :execrows
UPDATE file_upload_sessions SET state = 'aborted' WHERE file_id = sqlc.arg(file_id) AND state = 'open';
