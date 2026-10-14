-- SPDX-License-Identifier: Apache-2.0
-- Queries on enrollment_codes (identity, docs/spec/03-datenmodell.md section 4.2, M1-37). A code is
-- stored only as its HMAC (code_hash); no query reads or writes the code itself.

-- name: InsertEnrollmentCode :exec
INSERT INTO enrollment_codes (id, user_id, purpose, code_hash, expires_at, created_by, created_at)
VALUES (@id, @user_id, @purpose, @code_hash, @expires_at, sqlc.narg(created_by), @created_at);

-- name: ExpireOtherEnrollmentCodes :exec
-- At most one valid code per person (08 section 3.2): issuing a code ends every other unused,
-- unexpired code of the person.
UPDATE enrollment_codes SET expires_at = @now
WHERE user_id = @user_id AND id <> @keep AND used_at IS NULL AND expires_at > @now;

-- name: LockEnrollmentCodeByHash :one
-- The row of a matching code, locked until the end of the redeeming unit of work, so two parallel
-- redemptions of one code serialise and the second sees used_at.
SELECT * FROM enrollment_codes WHERE code_hash = @code_hash FOR UPDATE;

-- name: IncrementEnrollmentAttempts :execrows
-- One failed attempt with a matching code, in an own transaction after the failed unit rolled back.
UPDATE enrollment_codes SET attempts = attempts + 1 WHERE id = @id AND used_at IS NULL;

-- name: MarkEnrollmentCodeUsed :execrows
UPDATE enrollment_codes SET used_at = @used_at, used_by_device_id = @device_id
WHERE id = @id AND used_at IS NULL;

-- name: LockEnrollmentCode :one
-- The row of the code a passkey enrollment checked at begin, locked again at complete (M1-40).
SELECT * FROM enrollment_codes WHERE id = @id FOR UPDATE;
