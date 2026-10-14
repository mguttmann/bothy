-- SPDX-License-Identifier: Apache-2.0
-- Queries on device_pairings (identity, docs/spec/03-datenmodell.md section 4.2, M1-38). A pairing
-- code is stored only as its HMAC (pairing_code_hash); no query reads or writes the code itself.

-- name: InsertPairing :execrows
-- A code hash taken by another requested pairing inserts nothing; the caller draws a new code.
INSERT INTO device_pairings (id, pairing_code_hash, device_registration, status, created_at, expires_at)
VALUES (@id, @pairing_code_hash, @device_registration, 'requested', @created_at, @expires_at)
ON CONFLICT (pairing_code_hash) WHERE status = 'requested' DO NOTHING;

-- name: LockRequestedPairingByHash :one
-- The open request of a typed code, locked until the end of the approving unit of work.
SELECT * FROM device_pairings WHERE pairing_code_hash = @pairing_code_hash AND status = 'requested' FOR UPDATE;

-- name: LockPairing :one
SELECT * FROM device_pairings WHERE id = @id FOR UPDATE;

-- name: UpdatePairing :exec
UPDATE device_pairings
SET status = @status, user_id = sqlc.narg(user_id), approved_by_device_id = sqlc.narg(approved_by_device_id),
    device_id = sqlc.narg(device_id), expires_at = @expires_at, completed_at = sqlc.narg(completed_at)
WHERE id = @id;

-- name: ListDuePairings :many
-- The expiry of the job identity.auth_cleanup: open pairings past expires_at.
SELECT * FROM device_pairings
WHERE status IN ('requested', 'approved') AND expires_at <= @now
ORDER BY expires_at, id
LIMIT @row_limit;
