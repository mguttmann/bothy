-- SPDX-License-Identifier: Apache-2.0
-- Transcripts of voice files (M3-09, docs/spec/12-files.md 8.2, 03 4.7 files.transcript_*). The
-- job files.transcribe has no principal, so its reads are keyed by the file id only; requestTranscription
-- and submitTranscript lock the row after authorize (the exceptions of AZ-P12 in
-- tools/lint/sql_visibility.yaml). Only the files row is locked (03 6.3).

-- name: GetTranscriptFile :one
-- One file by id with its transcript, deleted or not (the job reads outside a transaction).
SELECT id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by, purpose,
       scope_channel_id, upload_state, scan_status, width, height, duration_ms, transcript_status, transcript_text,
       transcript_language, transcript_source, created_at, (deleted_at IS NOT NULL)::boolean AS deleted
FROM files
WHERE id = sqlc.arg(id);

-- name: LockTranscriptFile :one
-- GetTranscriptFile with the files row locked FOR UPDATE, for the result write of the job.
SELECT id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by, purpose,
       scope_channel_id, upload_state, scan_status, width, height, duration_ms, transcript_status, transcript_text,
       transcript_language, transcript_source, created_at, (deleted_at IS NOT NULL)::boolean AS deleted
FROM files
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: LockVoiceFile :one
-- requestTranscription and submitTranscript: the file the caller was authorized for, locked FOR
-- UPDATE after authorize.
SELECT id, object_key, sha256, size_bytes, mime_type, declared_mime_type, original_name, uploaded_by, purpose,
       scope_channel_id, upload_state, scan_status, width, height, duration_ms, transcript_status, transcript_text,
       transcript_language, transcript_source, created_at, (deleted_at IS NOT NULL)::boolean AS deleted
FROM files
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: SetTranscript :execrows
-- Every change of the transcript (pending, done, failed); NULL for an absent text, language or
-- source. search_vector follows transcript_text as a generated column (03 5).
UPDATE files
SET transcript_status = sqlc.arg(transcript_status), transcript_text = sqlc.narg(transcript_text),
    transcript_language = sqlc.narg(transcript_language), transcript_source = sqlc.narg(transcript_source)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
