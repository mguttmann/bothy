-- SPDX-License-Identifier: Apache-2.0

-- name: AdvanceCounter :one
-- Locks the counter row until the commit and returns the new value (04 "Pattern with a counter row").
UPDATE sequence_counters SET value = value + sqlc.arg(n)::bigint WHERE name = sqlc.arg(name) RETURNING value;

-- name: CounterValue :one
-- Reads a counter without a lock.
SELECT value FROM sequence_counters WHERE name = sqlc.arg(name);
