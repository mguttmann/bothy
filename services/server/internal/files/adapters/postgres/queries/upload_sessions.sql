-- SPDX-License-Identifier: Apache-2.0
-- file_upload_sessions (M3-03, 03 4.7). Reads are keyed by the file id and the uploader.

-- name: InsertSession :exec
INSERT INTO file_upload_sessions (file_id, mode, state, declared_size_bytes, declared_sha256, part_size_bytes,
                                  part_count, created_at, expires_at)
VALUES (sqlc.arg(file_id), sqlc.arg(mode), sqlc.arg(state), sqlc.arg(declared_size_bytes), sqlc.arg(declared_sha256),
        sqlc.arg(part_size_bytes), sqlc.arg(part_count), sqlc.arg(created_at), sqlc.arg(expires_at));

-- The row shape of one session with its file (srv-files-adapters-18): the four session reads
-- select the same columns, so their generated rows convert into each other and one mapping
-- (sessionOf) reads them all. The file columns equal those of GetFileByID (files.sql);
-- TestSessionRowCarriesTheFileRow holds that.

-- name: GetSessionByID :one
-- The session of one file with the file, deleted or not. Session keeps it only for the uploader
-- and a file that is not deleted (authorize of the upload operations, M3-03); the job files.scan
-- reads the session it was inserted for (design notes 2 and 6a of M3-04).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_text, f.transcript_language, f.transcript_source, f.created_at AS file_created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id);

-- name: LockSessionForUpdate :one
-- The session of one file of the uploader, not deleted, with the session row locked FOR UPDATE
-- (abort and completion, design notes 12 and 13).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_text, f.transcript_language, f.transcript_source, f.created_at AS file_created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id) AND f.uploaded_by = sqlc.arg(uploaded_by) AND f.deleted_at IS NULL
FOR UPDATE OF s;

-- name: LockSessionForShare :one
-- LockSessionForUpdate with the session row locked FOR SHARE (the confirmation of a part, design
-- note 9 h).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_text, f.transcript_language, f.transcript_source, f.created_at AS file_created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id) AND f.uploaded_by = sqlc.arg(uploaded_by) AND f.deleted_at IS NULL
FOR SHARE OF s;

-- name: LockScanTarget :one
-- The session of one file with the file, deleted or not, with the session row and then the files
-- row locked FOR UPDATE, the order of the completion and the abort of M3-03 (session, then file).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_text, f.transcript_language, f.transcript_source, f.created_at AS file_created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id)
FOR UPDATE OF s, f;

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
