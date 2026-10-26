-- SPDX-License-Identifier: Apache-2.0
-- Idempotency keys (M1-20; 03 idempotency_keys, 02 "Idempotency in the Unit of Work").

-- name: DeleteExpiredKey :execrows
-- Removes the row of one key when it expired at now, so the key counts as absent and can be
-- claimed again (the cleanup job may not have run yet).
DELETE FROM idempotency_keys
WHERE principal_id = sqlc.arg(principal_id) AND key = sqlc.arg(key)
  AND expires_at <= sqlc.arg(now)::timestamptz;

-- name: ClaimKey :execrows
-- Claims a key in state in_progress. A committed row of the same key makes it do nothing (0
-- rows); a concurrent uncommitted insert of the same key makes it wait at the primary key until
-- that transaction ends, at most lock_timeout.
INSERT INTO idempotency_keys (principal_id, key, action, request_sha256, state, created_at, expires_at)
VALUES (sqlc.arg(principal_id), sqlc.arg(key), sqlc.arg(action), sqlc.arg(request_sha256),
        'in_progress', sqlc.arg(created_at), sqlc.arg(expires_at))
ON CONFLICT (principal_id, key) DO NOTHING;

-- name: GetKey :one
-- Reads the row of a key that ClaimKey found taken.
SELECT request_sha256, state, response_status, response_body, resource_id
FROM idempotency_keys
WHERE principal_id = sqlc.arg(principal_id) AND key = sqlc.arg(key);

-- name: CompleteKey :execrows
-- Stores the final response of a claimed key in the transaction of the effect.
UPDATE idempotency_keys
SET state = 'completed', response_status = sqlc.arg(response_status),
    response_body = sqlc.narg(response_body), resource_id = sqlc.narg(resource_id)
WHERE principal_id = sqlc.arg(principal_id) AND key = sqlc.arg(key) AND state = 'in_progress';

-- name: SaveFinalKey :execrows
-- Stores a final error response after the unit of work rolled back; a row that exists is kept.
INSERT INTO idempotency_keys (principal_id, key, action, request_sha256, state, response_status,
                              response_body, resource_id, created_at, expires_at)
VALUES (sqlc.arg(principal_id), sqlc.arg(key), sqlc.arg(action), sqlc.arg(request_sha256),
        'completed', sqlc.arg(response_status), sqlc.narg(response_body), sqlc.narg(resource_id),
        sqlc.arg(created_at), sqlc.arg(expires_at))
ON CONFLICT (principal_id, key) DO NOTHING;

-- name: DeleteExpiredKeys :execrows
-- One batch of the job sync.idempotency_cleanup: the oldest expired rows, at most batch_size;
-- rows a request holds right now are skipped, a later batch or run takes them.
DELETE FROM idempotency_keys
WHERE (principal_id, key) IN (
    SELECT k.principal_id, k.key FROM idempotency_keys k
    WHERE k.expires_at <= sqlc.arg(now)::timestamptz
    ORDER BY k.expires_at
    LIMIT sqlc.arg(batch_size)::integer
    FOR UPDATE SKIP LOCKED
);

-- name: DeleteKeysOfPrincipal :execrows
-- Every key of the person with its stored response, in step personal of the account deletion
-- (M7-06, 20 section 4.4 item 3): a stored response is a copy of the person's data and would
-- otherwise wait for sync.idempotency_key_ttl.
DELETE FROM idempotency_keys WHERE principal_id = sqlc.arg(principal_id);
