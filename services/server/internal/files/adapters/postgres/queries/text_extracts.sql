-- SPDX-License-Identifier: Apache-2.0
-- The text extraction of files (M3-15, docs/spec/14-search.md section 5): file_text_extracts and
-- the facts of files the jobs files.extract_text and files.extract_text_refresh need. The jobs have
-- no principal: every read is keyed by a file id or walks the files by id (the system_job
-- exceptions of tools/lint/sql_visibility.yaml). The text is never selected outside SearchFiles.
-- search_language goes in as text with a cast and search_vector is never selected, so pgx needs no
-- codec for regconfig or tsvector (brief M3-13 design note 7).

-- name: InsertTextExtract :execrows
-- InsertTextExtract creates the pending row of a file that became deliverable (design note 2);
-- an existing row stays, so a second call inserts no second job.
INSERT INTO file_text_extracts (file_id, search_language)
VALUES (sqlc.arg(file_id), sqlc.arg(search_language)::text::regconfig)
ON CONFLICT (file_id) DO NOTHING;

-- name: GetExtractTarget :one
-- GetExtractTarget reads the row of one run with the facts of its file (design note 4a).
SELECT e.state, f.purpose, f.mime_type, f.size_bytes, f.object_key, f.upload_state, f.scan_status,
       (f.deleted_at IS NOT NULL)::boolean AS deleted
FROM file_text_extracts e
JOIN files f ON f.id = e.file_id
WHERE e.file_id = sqlc.arg(file_id);

-- name: WriteTextExtract :execrows
-- WriteTextExtract writes the result of one run to the pending row of a file that is not deleted
-- (design note 8); no row changed means another run was first, the row is gone or the file was
-- deleted. text_content, extractor and error are NULL where the caller passes NULL.
UPDATE file_text_extracts e
SET state = sqlc.arg(state), text_content = sqlc.narg(text_content), truncated = sqlc.arg(truncated),
    extractor = sqlc.narg(extractor), extracted_at = sqlc.arg(extracted_at), error = sqlc.narg(error)
FROM files f
WHERE e.file_id = sqlc.arg(file_id)
  AND e.state = 'pending'
  AND f.id = e.file_id
  AND f.deleted_at IS NULL;

-- name: DeleteTextExtract :exec
-- DeleteTextExtract removes the row of one file (a run found it deleted or not deliverable).
DELETE FROM file_text_extracts WHERE file_id = sqlc.arg(file_id);

-- name: DeleteTextExtractsOfDeletedFiles :exec
-- DeleteTextExtractsOfDeletedFiles removes the rows of the listed files that are deleted, in the
-- unit of work that set their deleted_at (design note 10).
DELETE FROM file_text_extracts e
USING files f
WHERE e.file_id = ANY(sqlc.arg(file_ids)::uuid[])
  AND f.id = e.file_id
  AND f.deleted_at IS NOT NULL;

-- name: ListRefreshCandidates :many
-- ListRefreshCandidates is one batch of the walk (design note 9): deliverable attachments after
-- the id after, ascending, each with its extract row if any.
SELECT f.id, f.mime_type, f.size_bytes, (e.file_id IS NOT NULL)::boolean AS has_row,
       coalesce(e.state, '')::text AS state, coalesce(e.extractor, '')::text AS extractor,
       coalesce(e.error, '')::text AS error
FROM files f
LEFT JOIN file_text_extracts e ON e.file_id = f.id
WHERE f.id > sqlc.arg(after)::uuid
  AND f.purpose = 'attachment'
  AND f.upload_state = 'complete'
  AND f.scan_status IN ('clean', 'skipped')
  AND f.deleted_at IS NULL
ORDER BY f.id
LIMIT sqlc.arg(batch_size)::integer;

-- name: ResetTextExtract :execrows
-- ResetTextExtract sets a row pending again when it is still as the walk saw it (compare-and-set);
-- the text, truncated, extractor and extracted_at stay, so the file stays findable by its old
-- text until the job writes the new result.
UPDATE file_text_extracts
SET state = 'pending'
WHERE file_id = sqlc.arg(file_id)
  AND state = sqlc.arg(seen_state)
  AND extractor IS NOT DISTINCT FROM sqlc.narg(seen_extractor)::text;

-- name: DeleteOrphanTextExtracts :execrows
-- DeleteOrphanTextExtracts removes up to batch_size rows whose file has deleted_at set (the
-- catch-up of the walk for design note 10).
DELETE FROM file_text_extracts
WHERE file_id IN (
    SELECT e.file_id
    FROM file_text_extracts e
    JOIN files f ON f.id = e.file_id
    WHERE f.deleted_at IS NOT NULL
    ORDER BY e.file_id
    LIMIT sqlc.arg(batch_size)::integer
);
