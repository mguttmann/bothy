-- SPDX-License-Identifier: Apache-2.0
-- gdpr_requests (M7-06): the privacy requests of docs/spec/20-privacy.md section 5. Only the module
-- privacy reads and writes the table (03 principle 12).

-- name: InsertRequest :exec
INSERT INTO gdpr_requests (id, subject_user_id, kind, state, requested_by, approved_by, reason, due_at, progress, created_at)
VALUES (@id, @subject_user_id, @kind, @state, @requested_by, @approved_by, @reason, @due_at, @progress, @created_at);

-- name: OpenRequestOf :one
SELECT * FROM gdpr_requests
WHERE subject_user_id = @subject_user_id AND kind = @kind AND state IN ('requested', 'approved', 'running');

-- name: GetRequest :one
SELECT * FROM gdpr_requests WHERE id = @id;

-- name: LockRequest :one
SELECT * FROM gdpr_requests WHERE id = @id FOR UPDATE;

-- name: UpdateRequest :exec
UPDATE gdpr_requests
SET state = @state, approved_by = @approved_by, due_at = @due_at, result_file_id = @result_file_id,
    result_expires_at = @result_expires_at, progress = @progress, error = @error, completed_at = @completed_at
WHERE id = @id;

-- name: ListRequests :many
SELECT * FROM gdpr_requests
WHERE (sqlc.narg(subject_user_id)::uuid IS NULL OR subject_user_id = sqlc.narg(subject_user_id))
  AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state))
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: ReminderCandidates :many
SELECT * FROM gdpr_requests
WHERE state IN ('requested', 'approved', 'running') AND due_at <= @until
  AND NOT COALESCE((progress ->> 'reminder_sent')::boolean, false)
ORDER BY due_at, id
LIMIT @row_limit;

-- name: ExpiredPackages :many
SELECT * FROM gdpr_requests
WHERE state = 'completed' AND kind = 'export' AND result_file_id IS NOT NULL AND result_expires_at <= @now
ORDER BY result_expires_at, id
LIMIT @row_limit;

-- name: RequestsOfSubject :many
SELECT * FROM gdpr_requests WHERE subject_user_id = @subject_user_id ORDER BY created_at, id;

-- name: PurgeRequests :execrows
-- A deletion that may have pseudonymized the person (step 6 begins after last_step 5) stays until
-- the consumer privacy.journal wrote its account entry and marked it journaled, so the entry
-- still finds the content mode (M7-07 design note 1).
DELETE FROM gdpr_requests
WHERE id IN (
    SELECT g.id FROM gdpr_requests g
    WHERE g.state IN ('completed', 'failed', 'rejected', 'cancelled') AND g.completed_at < @cutoff
      AND g.result_file_id IS NULL
      AND NOT (g.kind = 'deletion' AND COALESCE((g.progress ->> 'last_step')::int, 0) >= 5
               AND NOT COALESCE((g.progress ->> 'journaled')::boolean, false))
    ORDER BY g.completed_at, g.id
    LIMIT @row_limit
);
