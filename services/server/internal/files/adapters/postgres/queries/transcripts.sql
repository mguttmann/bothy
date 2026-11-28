-- SPDX-License-Identifier: Apache-2.0
-- Transcripts of voice files (M3-09, docs/spec/12-files.md 8.2, 03 4.7 files.transcript_*). The
-- job files.transcribe reads and locks its file with GetFileByID and LockFileByID (files.sql),
-- requestTranscription and submitTranscript lock it with LockFileByID after authorize. Only the
-- files row is locked (03 6.3).

-- name: SetTranscript :execrows
-- Every change of the transcript (pending, done, failed); NULL for an absent text, language or
-- source. search_vector follows transcript_text as a generated column (03 5).
UPDATE files
SET transcript_status = sqlc.arg(transcript_status), transcript_text = sqlc.narg(transcript_text),
    transcript_language = sqlc.narg(transcript_language), transcript_source = sqlc.narg(transcript_source)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
