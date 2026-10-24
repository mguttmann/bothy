-- SPDX-License-Identifier: Apache-2.0
-- Upload sessions of imported files (M8-11; docs/spec/22-import-migration.md 7.2 item 3,
-- docs/spec/12-files.md 11). Only the port files.ImportFiles runs them.

-- name: ReopenImportSession :execrows
-- A file item whose session was aborted or expired keeps its file ID (22 7.2 item 3): the session
-- opens again with a new expiry and without a multipart upload, its file is pending again.
UPDATE file_upload_sessions
SET state = 'open', created_at = sqlc.arg(created_at), expires_at = sqlc.arg(expires_at), completed_at = NULL,
    failure_reason = NULL, storage_upload_id = NULL
WHERE file_id = sqlc.arg(file_id) AND state <> 'completed';

-- name: PendImportFile :exec
-- The file of a reopened session is pending again.
UPDATE files SET upload_state = 'pending', scan_status = 'pending'
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND upload_state IN ('pending', 'failed');
