-- SPDX-License-Identifier: Apache-2.0
-- The scan pass of M3-04 (docs/spec/12-files.md 3.4, 5). The job files.scan has no principal; it
-- reads and locks its session with GetSessionByID and LockScanTarget (upload_sessions.sql, the
-- exceptions of AZ-P12 in tools/lint/sql_visibility.yaml); the count of the health monitor reads
-- no content.

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
