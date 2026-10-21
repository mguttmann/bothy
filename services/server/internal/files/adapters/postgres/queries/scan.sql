-- SPDX-License-Identifier: Apache-2.0
-- The scan pass of M3-04 (docs/spec/12-files.md 3.4, 5). The job files.scan has no principal, so
-- its reads are keyed by the file id only (the exceptions of AZ-P12 in
-- tools/lint/sql_visibility.yaml); the count of the health monitor reads no content.

-- name: GetScanTarget :one
-- The session of one file with the file, deleted or not (design notes 2 and 6a).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at AS file_created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id);

-- name: LockScanTarget :one
-- GetScanTarget with the session row and then the files row locked FOR UPDATE, the order of the
-- completion and the abort of M3-03 (session, then file).
SELECT s.file_id, s.mode, s.state, s.declared_size_bytes, s.declared_sha256, s.part_size_bytes, s.part_count,
       s.storage_upload_id, s.created_at, s.completed_at, s.expires_at,
       f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at AS file_created_at,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.file_id = sqlc.arg(file_id)
FOR UPDATE OF s, f;

-- name: FinishScanFile :execrows
-- The new state of a file that still awaits the pass (design note 6b).
UPDATE files
SET upload_state = sqlc.arg(upload_state), scan_status = sqlc.arg(scan_status), mime_type = sqlc.arg(mime_type),
    sha256 = sqlc.arg(sha256), size_bytes = sqlc.arg(size_bytes), scanned_at = sqlc.narg(scanned_at),
    scan_engine = sqlc.narg(scan_engine)
WHERE id = sqlc.arg(id) AND upload_state = 'pending' AND scan_status = 'pending' AND deleted_at IS NULL;

-- name: SetSessionFailureReason :exec
UPDATE file_upload_sessions SET failure_reason = sqlc.narg(failure_reason) WHERE file_id = sqlc.arg(file_id);

-- name: CountAwaitingScan :one
-- The files awaiting the pass for bothy.file.scan.queue, over the partial index on
-- scan_status = 'pending'.
SELECT count(*)::bigint AS awaiting
FROM files f
JOIN file_upload_sessions s ON s.file_id = f.id
WHERE f.scan_status = 'pending' AND f.upload_state = 'pending' AND f.deleted_at IS NULL AND s.state = 'completed';
