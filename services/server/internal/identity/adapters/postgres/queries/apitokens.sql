-- SPDX-License-Identifier: Apache-2.0
-- Queries on api_tokens (identity, docs/spec/03-data-model.md section 4.2, docs/spec/08-identity.md
-- section 12, M1-44). A token is stored only as HMAC (token_hash) and token_prefix; the lookup of a
-- request goes through the unique token_hash.

-- name: InsertAPIToken :exec
INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, scopes, dpop_jkt, expires_at, created_by, created_at)
VALUES (@id, @principal_id, @name, @token_prefix, @token_hash, @scopes, sqlc.narg(dpop_jkt), @expires_at, @created_by, @created_at);

-- name: GetAPIToken :one
SELECT * FROM api_tokens WHERE id = @id;

-- name: LockAPIToken :one
SELECT * FROM api_tokens WHERE id = @id FOR UPDATE;

-- name: GetAPITokenByHash :one
-- The authenticator: the token and the kind of its principal in one query.
SELECT sqlc.embed(t), p.kind AS principal_kind
FROM api_tokens t
JOIN principals p ON p.id = t.principal_id
WHERE t.token_hash = @token_hash;

-- name: ListActiveAPITokens :many
-- Keyset on (created_at DESC, id DESC) over the partial index of the active tokens; expired tokens
-- stay listed so they can be seen and revoked.
SELECT * FROM api_tokens
WHERE principal_id = @principal_id AND revoked_at IS NULL
  AND (sqlc.narg(after_id)::uuid IS NULL OR (created_at, id) < (@after_created::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: RevokeAPIToken :execrows
UPDATE api_tokens SET revoked_at = @revoked_at, revoked_by = @revoked_by
WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeAPITokensOfPrincipal :execrows
UPDATE api_tokens SET revoked_at = @revoked_at, revoked_by = @revoked_by
WHERE principal_id = @principal_id AND revoked_at IS NULL;

-- name: TouchAPIToken :exec
-- last_used_at at most once per identity.last_active_granularity (03 api_tokens).
UPDATE api_tokens SET last_used_at = @used_at
WHERE id = @id AND (last_used_at IS NULL OR last_used_at < @threshold);
