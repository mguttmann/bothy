-- SPDX-License-Identifier: Apache-2.0
-- files (M3-03, docs/spec/03-data-model.md 4.7). Every read is keyed by the file id or by the
-- uploader, never a list across channels (07 7.4 rule 1; the exceptions of AZ-P12 in
-- tools/lint/sql_visibility.yaml).

-- name: InsertFile :exec
INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by,
                   purpose, scope_channel_id, upload_state, scan_status, transcript_status, created_at)
VALUES (sqlc.arg(id), sqlc.arg(object_key), sqlc.arg(sha256), sqlc.arg(size_bytes), sqlc.arg(mime_type),
        sqlc.narg(declared_mime_type), sqlc.arg(original_name), sqlc.arg(uploaded_by), sqlc.arg(purpose),
        sqlc.narg(scope_channel_id), sqlc.arg(upload_state), sqlc.arg(scan_status), sqlc.arg(transcript_status),
        sqlc.arg(created_at));

-- name: GetFile :one
-- One file that is not deleted, for authorize and getFile.
SELECT id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by, purpose,
       scope_channel_id, upload_state, scan_status, width, height, duration_ms, transcript_status,
       transcript_language, transcript_source, created_at
FROM files
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: QuotaUsedBytes :one
-- The quota of the uploader: the files that are neither deleted nor failed (D-M3-03-6).
SELECT coalesce(sum(size_bytes), 0)::bigint AS used
FROM files
WHERE uploaded_by = sqlc.arg(uploaded_by) AND deleted_at IS NULL AND upload_state <> 'failed';

-- name: LockPrincipalUploads :exec
-- The transaction-level advisory lock of the uploads of one principal (D-M3-03-7), a domain lock
-- in step 2 of the lock order of 03 6.3.
SELECT pg_advisory_xact_lock(hashtextextended('bothy.files.uploads:' || sqlc.arg(principal)::uuid::text, 0));

-- name: FailFile :exec
UPDATE files SET upload_state = 'failed' WHERE id = sqlc.arg(id) AND upload_state = 'pending';

-- name: MarkFilesDeleted :many
-- The one deleted_at write of the module (app markDeleted, brief M3-07 design note 7 c): the listed
-- files that are not deleted yet, optionally only of one purpose (a replaced avatar or logo, M3-10,
-- docs/DECISIONS.md D-M3-10-5) or of one uploader. It returns the files it marked, which alone take
-- the further steps of markDeleted (M3-08); the objects go with files.purge_objects.
UPDATE files SET deleted_at = sqlc.arg(deleted_at)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL
  AND (sqlc.narg(purpose)::text IS NULL OR purpose = sqlc.narg(purpose)::text)
  AND (sqlc.narg(uploaded_by)::uuid IS NULL OR uploaded_by = sqlc.narg(uploaded_by)::uuid)
RETURNING id;
