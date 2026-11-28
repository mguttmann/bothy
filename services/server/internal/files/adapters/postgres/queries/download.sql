-- SPDX-License-Identifier: Apache-2.0
-- The read path of downloads, tickets and transcripts (M3-05, docs/spec/12-files.md 9). Every
-- read is keyed by the file id (07 7.4 rule 1); the file itself comes from GetFileByID
-- (files.sql); nothing here writes.

-- name: GetThumbnail :one
-- One thumbnail of a file by its variant (03 4.7 file_thumbnails).
SELECT file_id, variant, object_key, mime_type, width, height, size_bytes
FROM file_thumbnails
WHERE file_id = sqlc.arg(file_id) AND variant = sqlc.arg(variant);
