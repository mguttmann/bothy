-- SPDX-License-Identifier: Apache-2.0
-- The personal data participant of files (M7-06, docs/spec/20-privacy.md sections 4.4 and 5.2):
-- the jobs privacy.export and privacy.erase run without a principal and read the rows of one
-- data subject by uploaded_by (the exceptions of AZ-P12 in tools/lint/sql_visibility.yaml).

-- name: ListPersonalFiles :many
-- The own uploads of the subject that the export package holds: purpose attachment or avatar, not
-- deleted, complete and deliverable (12 section 2), so the object exists and was scanned. The
-- channel is that of the attachment row when attached (it follows a thread move), else the scope
-- channel of the upload.
SELECT f.id, f.object_key, f.original_name, f.mime_type, f.purpose, f.created_at,
       coalesce(a.channel_id, f.scope_channel_id) AS channel_id,
       f.transcript_status, f.transcript_text, f.transcript_language
FROM files f
LEFT JOIN message_attachments a ON a.file_id = f.id
WHERE f.uploaded_by = sqlc.arg(uploaded_by) AND f.deleted_at IS NULL
  AND f.purpose IN ('attachment', 'avatar')
  AND f.upload_state = 'complete' AND f.scan_status IN ('clean', 'skipped')
ORDER BY f.created_at, f.id;

-- name: ListOpenUploadsOfUploader :many
-- The files of the open upload sessions of the subject that are not deleted, for the abort of
-- step 3 of the account deletion.
SELECT s.file_id
FROM file_upload_sessions s
JOIN files f ON f.id = s.file_id
WHERE f.uploaded_by = sqlc.arg(uploaded_by) AND s.state = 'open' AND f.deleted_at IS NULL
ORDER BY s.file_id;

-- name: ListUnattachedFilesOfUploader :many
-- The files of one purpose of the subject that are neither deleted nor attached to a message, for
-- steps 5 (attachment) and 6 (avatar) of the account deletion.
SELECT f.id
FROM files f
WHERE f.uploaded_by = sqlc.arg(uploaded_by) AND f.purpose = sqlc.arg(purpose) AND f.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM message_attachments a WHERE a.file_id = f.id)
ORDER BY f.id;
