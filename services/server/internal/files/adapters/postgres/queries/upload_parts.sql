-- SPDX-License-Identifier: Apache-2.0
-- file_upload_parts (M3-03, 03 4.7). Reads are keyed by the file id of a session the use case
-- authorized.

-- name: InsertPart :execrows
INSERT INTO file_upload_parts (file_id, part_number, size_bytes, sha256, etag, received_at)
VALUES (sqlc.arg(file_id), sqlc.arg(part_number), sqlc.arg(size_bytes), sqlc.arg(sha256), sqlc.narg(etag),
        sqlc.arg(received_at))
ON CONFLICT (file_id, part_number) DO NOTHING;

-- name: ListParts :many
SELECT file_id, part_number, size_bytes, sha256, etag, received_at
FROM file_upload_parts
WHERE file_id = sqlc.arg(file_id)
ORDER BY part_number;

-- name: GetPart :one
SELECT file_id, part_number, size_bytes, sha256, etag, received_at
FROM file_upload_parts
WHERE file_id = sqlc.arg(file_id) AND part_number = sqlc.arg(part_number);

-- name: DeleteParts :exec
DELETE FROM file_upload_parts WHERE file_id = sqlc.arg(file_id);
