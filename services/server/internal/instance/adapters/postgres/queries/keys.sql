-- SPDX-License-Identifier: Apache-2.0

-- name: LockSigningKeyPurpose :exec
-- Serializes every change to the keys of one purpose until the commit, also while no row exists
-- (brief M1-32 section 5; the domain step of 03 section 6.3).
SELECT pg_advisory_xact_lock(hashtext('bothy.server_signing_keys'), hashtext(sqlc.arg(purpose)::text));

-- name: ListSigningKeys :many
-- The unretired keys of one purpose, oldest first.
SELECT kid, purpose, alg, public_jwk, private_key_ref, state, created_at, not_after
FROM server_signing_keys
WHERE purpose = sqlc.arg(purpose) AND state <> 'retired'
ORDER BY created_at, kid;

-- name: ActiveSigningKey :one
-- The one active key of a purpose (partial unique index).
SELECT kid, purpose, alg, public_jwk, private_key_ref, state, created_at, not_after
FROM server_signing_keys
WHERE purpose = sqlc.arg(purpose) AND state = 'active';

-- name: PublishedSigningKeys :many
-- Every next, active and retiring key of every purpose.
SELECT kid, purpose, alg, public_jwk, private_key_ref, state, created_at, not_after
FROM server_signing_keys
WHERE state IN ('next', 'active', 'retiring')
ORDER BY purpose, kid;

-- name: SigningKeyByKID :one
-- One key in any state.
SELECT kid, purpose, alg, public_jwk, private_key_ref, state, created_at, not_after
FROM server_signing_keys
WHERE kid = sqlc.arg(kid);

-- name: InsertSigningKey :exec
INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state, created_at)
VALUES (sqlc.arg(kid), sqlc.arg(purpose), sqlc.arg(alg), sqlc.arg(public_jwk), sqlc.arg(private_key_ref),
        sqlc.arg(state), sqlc.arg(created_at));

-- name: TransitionSigningKey :execrows
-- Moves a key from one state to another; not_after is set when given and kept otherwise. No row
-- changes when the key left from_state in the meantime.
UPDATE server_signing_keys
SET state = sqlc.arg(to_state)::text,
    not_after = coalesce(sqlc.narg(not_after)::timestamptz, not_after)
WHERE kid = sqlc.arg(kid) AND state = sqlc.arg(from_state)::text;

-- name: PurposesWithoutActiveKey :many
-- The purposes of the list that have no active key.
SELECT p::text AS purpose
FROM unnest(sqlc.arg(purposes)::text[]) AS p
WHERE NOT EXISTS (SELECT 1 FROM server_signing_keys k WHERE k.purpose = p AND k.state = 'active')
ORDER BY p;

-- name: ExpiredSigningKeys :many
-- Retention class security (M7-05): retired keys whose not_after lies before the cutoff, oldest
-- first. Keys of purpose audit_checkpoint stay: audit.verify checks every checkpoint at
-- audit.checkpoint_sink, which keeps them all, with the key of its kid in any state.
SELECT kid FROM server_signing_keys
WHERE state = 'retired' AND purpose <> 'audit_checkpoint' AND not_after < sqlc.arg(cutoff)
ORDER BY not_after, kid
LIMIT sqlc.arg(row_limit);

-- name: DeleteSigningKey :execrows
DELETE FROM server_signing_keys
WHERE kid = sqlc.arg(kid) AND state = 'retired' AND purpose <> 'audit_checkpoint' AND not_after < sqlc.arg(cutoff);
