-- SPDX-License-Identifier: Apache-2.0
-- The job files.thumbnail of M3-06 (docs/spec/12-files.md 7, brief M3-06 design note 10). The
-- job has no principal, so its reads are keyed by the file id only (the exceptions of AZ-P12 in
-- tools/lint/sql_visibility.yaml); only the files row is locked (03 6.3).

-- name: GetImageFile :one
-- One file by id, deleted or not (design note 3b).
SELECT id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by, purpose,
       scope_channel_id, upload_state, scan_status, width, height, duration_ms, transcript_status,
       transcript_language, transcript_source, created_at, (deleted_at IS NOT NULL)::boolean AS deleted
FROM files
WHERE id = sqlc.arg(id);

-- name: LockImageFile :one
-- GetImageFile with the files row locked FOR UPDATE (design note 10).
SELECT id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by, purpose,
       scope_channel_id, upload_state, scan_status, width, height, duration_ms, transcript_status,
       transcript_language, transcript_source, created_at, (deleted_at IS NOT NULL)::boolean AS deleted
FROM files
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: StoreImageOriginal :execrows
-- T1 (design note 10a): the cleaned original becomes the stored object and the file deliverable.
UPDATE files
SET object_key = sqlc.arg(object_key), sha256 = sqlc.arg(sha256), size_bytes = sqlc.arg(size_bytes),
    mime_type = sqlc.arg(mime_type), width = sqlc.narg(width), height = sqlc.narg(height), upload_state = 'complete'
WHERE id = sqlc.arg(id) AND upload_state = 'pending' AND deleted_at IS NULL;

-- name: FailImageFile :execrows
-- The failure transaction (design note 10b): a JPEG or PNG with a broken structure.
UPDATE files SET upload_state = 'failed'
WHERE id = sqlc.arg(id) AND upload_state = 'pending' AND deleted_at IS NULL;

-- name: SetFileMediaInfo :execrows
-- T3 (design note 10d): the display size and duration of an MP4 or QuickTime file; NULL keeps a value.
UPDATE files
SET width = coalesce(sqlc.narg(width), width), height = coalesce(sqlc.narg(height), height),
    duration_ms = coalesce(sqlc.narg(duration_ms), duration_ms)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CompleteContainerFile :execrows
-- The container phase of MP4 audio (M3-09 design note 5): the media values and upload_state
-- complete together, for a pending file whose scan allows delivery; NULL keeps a value.
UPDATE files
SET width = coalesce(sqlc.narg(width), width), height = coalesce(sqlc.narg(height), height),
    duration_ms = coalesce(sqlc.narg(duration_ms), duration_ms), upload_state = 'complete'
WHERE id = sqlc.arg(id) AND upload_state = 'pending' AND scan_status IN ('clean', 'skipped') AND deleted_at IS NULL;

-- name: ListFileThumbnails :many
-- The thumbnails of one file (12 7.3); the caller orders them by variant.
SELECT file_id, variant, object_key, mime_type, width, height, size_bytes
FROM file_thumbnails
WHERE file_id = sqlc.arg(file_id);

-- name: InsertFileThumbnail :execrows
-- T2 (design note 10c): a row is inserted once and never updated, so its object is never written again.
INSERT INTO file_thumbnails (file_id, variant, object_key, mime_type, width, height, size_bytes)
VALUES (sqlc.arg(file_id), sqlc.arg(variant), sqlc.arg(object_key), sqlc.arg(mime_type), sqlc.arg(width),
        sqlc.arg(height), sqlc.arg(size_bytes))
ON CONFLICT DO NOTHING;
