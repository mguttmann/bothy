-- SPDX-License-Identifier: Apache-2.0
-- Cleanup and deletion of files (M3-08, docs/spec/12-files.md section 12). The jobs
-- files.cleanup_pending and files.purge_objects have no principal, so their reads are keyed by a
-- file id or by state and time (the exceptions of AZ-P12 in tools/lint/sql_visibility.yaml,
-- category system_job). deleteFile resolves its resource by id (authorize_resolution) and locks it
-- after authorize (after_authorize). The candidate reads page by a key of (time, id), so a
-- candidate that stays (a held draft, a failed unit) is never read twice in one run. No index is
-- added (03 4.7 names the indexes of files; docs/decisions.d/M3-08.md D-M3-08-17).

-- name: ExpiredOpenSessions :many
-- Step a: open sessions past expires_at of files that are not deleted, over the partial index
-- (expires_at) WHERE state = 'open'.
SELECT s.file_id, s.expires_at
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE s.state = 'open' AND s.expires_at <= sqlc.arg(now) AND f.deleted_at IS NULL
  AND (s.expires_at, s.file_id) > (sqlc.arg(after_time)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY s.expires_at, s.file_id
LIMIT sqlc.arg(batch_size);

-- name: UnattachedCandidates :many
-- Step b: deliverable attachment files that are not deleted and not attached, created at or
-- before the cutoff (a cheap prefilter; the age of D-M3-08-9 is checked after the lock).
SELECT f.id, f.created_at
FROM files f
WHERE f.purpose = 'attachment' AND f.deleted_at IS NULL AND f.upload_state = 'complete'
  AND f.scan_status IN ('clean', 'skipped') AND f.created_at <= sqlc.arg(cutoff)
  AND NOT EXISTS (SELECT 1 FROM message_attachments a WHERE a.file_id = f.id)
  AND (f.created_at, f.id) > (sqlc.arg(after_time)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY f.created_at, f.id
LIMIT sqlc.arg(batch_size);

-- name: FailedRowCandidates :many
-- Step c: failed or blocked attachment files that are not deleted and not attached, created at or
-- before the cutoff.
SELECT f.id, f.created_at
FROM files f
WHERE f.purpose = 'attachment' AND f.deleted_at IS NULL
  AND (f.upload_state = 'failed' OR f.scan_status IN ('infected', 'error')) AND f.created_at <= sqlc.arg(cutoff)
  AND NOT EXISTS (SELECT 1 FROM message_attachments a WHERE a.file_id = f.id)
  AND (f.created_at, f.id) > (sqlc.arg(after_time)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY f.created_at, f.id
LIMIT sqlc.arg(batch_size);

-- name: DeletedRows :many
-- Step d: the files marked deleted whose purge is still due, oldest mark first.
SELECT f.id, f.deleted_at::timestamptz AS deleted_at
FROM files f
WHERE f.deleted_at IS NOT NULL
  AND (f.deleted_at, f.id) > (sqlc.arg(after_time)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY f.deleted_at, f.id
LIMIT sqlc.arg(batch_size);

-- name: PurgeBacklog :one
-- Step f: the number of files marked deleted and the oldest mark ('epoch' without one).
SELECT count(*)::bigint AS backlog, coalesce(min(f.deleted_at), 'epoch'::timestamptz)::timestamptz AS oldest
FROM files f
WHERE f.deleted_at IS NOT NULL;

-- name: PurgeFacts :one
-- files.purge_objects: what decides the plan and which objects and parts exist, by file id,
-- deleted or not.
SELECT f.object_key, (f.deleted_at IS NOT NULL)::boolean AS deleted, f.upload_state, f.scan_status, f.uploaded_by,
       s.state AS session_state, s.part_count, s.storage_upload_id
FROM files f
LEFT JOIN file_upload_sessions s ON s.file_id = f.id
WHERE f.id = sqlc.arg(id);

-- name: LockFileForCleanup :one
-- The cleanup and the purge row delete: the files row FOR UPDATE, deleted or not; the facts are
-- read again in the next statement (CleanupFileFacts).
SELECT f.id
FROM files f
WHERE f.id = sqlc.arg(id)
FOR UPDATE OF f;

-- name: CleanupFileFacts :one
-- After LockFileForCleanup or LockFileForDelete: the state, the stored times, the session and
-- the attachment row of one file, read in a new statement after the lock.
SELECT f.uploaded_by, f.purpose, f.upload_state, f.scan_status, (f.deleted_at IS NOT NULL)::boolean AS deleted,
       f.created_at, f.scanned_at, s.state AS session_state, s.completed_at, s.expires_at,
       EXISTS (SELECT 1 FROM message_attachments a WHERE a.file_id = f.id)::boolean AS attached
FROM files f
LEFT JOIN file_upload_sessions s ON s.file_id = f.id
WHERE f.id = sqlc.arg(id);

-- name: FileForDelete :one
-- deleteFile (design note 9 a): the row by id, a deleted one included, to resolve the resource of
-- file.delete.
SELECT f.id, f.uploaded_by, f.purpose, f.scope_channel_id, (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM files f
WHERE f.id = sqlc.arg(id);

-- name: LockFileForDelete :one
-- deleteFile (design note 9 e): the files row FOR UPDATE after authorize.
SELECT f.id
FROM files f
WHERE f.id = sqlc.arg(id)
FOR UPDATE OF f;

-- name: AbortOpenSessionsOfFiles :exec
-- markDeleted (design note 2 b): the open session of a file just marked deleted becomes aborted;
-- upload_state stays and no file.updated is written.
UPDATE file_upload_sessions SET state = 'aborted' WHERE file_id = ANY(sqlc.arg(ids)::uuid[]) AND state = 'open';

-- name: ExpireSession :execrows
-- Step a: an open session becomes expired.
UPDATE file_upload_sessions SET state = 'expired' WHERE file_id = sqlc.arg(file_id) AND state = 'open';

-- name: DeleteFileRow :execrows
-- files.purge_objects (design note 4 c): the row of a deleted file once its objects are gone;
-- sessions, parts, thumbnails and text extracts follow by ON DELETE CASCADE, an avatar reference
-- becomes NULL by ON DELETE SET NULL.
DELETE FROM files WHERE id = sqlc.arg(id) AND deleted_at IS NOT NULL;
