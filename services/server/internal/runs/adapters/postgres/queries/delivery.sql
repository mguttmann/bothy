-- SPDX-License-Identifier: Apache-2.0
-- The delivery of an invocation-bearing event (M2-08, 04 15.3 step 2) and the job
-- runs.queued_expiry: the run row is locked first, so two deliveries of one run, a delivery and
-- the expiry, and a delivery and a start serialize on it (03 runs.version). runs has no
-- channel_id; the run is named by the event or the job (tools/lint/sql_visibility.yaml).

-- name: LockRun :one
-- The row lock of the run for the rest of the unit of work.
SELECT * FROM runs WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: RecordDelivery :one
-- An allowed delivery: invocation_jti is replaced and renewed_from_jti set to renewed_from (NULL
-- for a delivery, the replaced jti for a renewal of M2-09) when record is true (a token that may
-- start the run), billing_mode_expected is the configured mode now. The
-- caller holds the row lock and calls this only when one of the columns changes, so version rises
-- by exactly one per change (03 runs.version).
UPDATE runs
SET invocation_jti = CASE WHEN sqlc.arg(record)::boolean THEN sqlc.arg(jti)::uuid ELSE invocation_jti END,
    renewed_from_jti = CASE WHEN sqlc.arg(record)::boolean THEN sqlc.narg(renewed_from)::uuid ELSE renewed_from_jti END,
    billing_mode_expected = sqlc.arg(billing_mode_expected)::text,
    version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;
