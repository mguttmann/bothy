-- SPDX-License-Identifier: Apache-2.0
-- deletion_journal_state (M7-07): the numbering of the deletion journal of
-- docs/spec/20-privacy.md section 4.6 (03 4.12, R-DM-52). Only the module privacy reads and writes
-- the table (03 principle 12); it has exactly one row with id 1.

-- name: TakeJournalNumber :one
UPDATE deletion_journal_state SET next_number = next_number + 1, updated_at = now()
WHERE id = 1
RETURNING (next_number - 1)::bigint AS number;

-- name: JournalNumbers :one
SELECT next_number, low_number FROM deletion_journal_state WHERE id = 1;

-- name: SetJournalNext :execrows
UPDATE deletion_journal_state SET next_number = @next_number, updated_at = now()
WHERE id = 1;

-- name: SetJournalLow :execrows
UPDATE deletion_journal_state SET low_number = @low_number, updated_at = now()
WHERE id = 1 AND low_number < @low_number AND @low_number <= next_number;

-- name: LockDeletionOf :one
SELECT * FROM gdpr_requests
WHERE subject_user_id = @subject_user_id AND kind = 'deletion' AND state IN ('running', 'completed', 'failed')
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;
