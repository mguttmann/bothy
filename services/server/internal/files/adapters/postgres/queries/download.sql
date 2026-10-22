-- SPDX-License-Identifier: Apache-2.0
-- The read path of downloads, tickets and transcripts (M3-05, docs/spec/12-files.md 9). Every
-- read is keyed by the file id (07 7.4 rule 1); nothing here writes.

-- name: GetFileForDelivery :one
-- One file with what a delivery needs: the object, the digest, the type, the name, the facts of
-- file.read, the states and the transcript. A deleted file is read too, so the use case answers it
-- like an unknown id (D-M3-05-5).
SELECT id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id,
       upload_state, scan_status, transcript_status, transcript_text, transcript_language, transcript_source,
       created_at, deleted_at
FROM files
WHERE id = sqlc.arg(id);

-- name: GetThumbnail :one
-- One thumbnail of a file by its variant (03 4.7 file_thumbnails).
SELECT file_id, variant, object_key, mime_type, width, height, size_bytes
FROM file_thumbnails
WHERE file_id = sqlc.arg(file_id) AND variant = sqlc.arg(variant);
