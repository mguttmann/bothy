-- SPDX-License-Identifier: Apache-2.0
-- Queries on refresh_token_families and refresh_tokens (identity, docs/spec/03-data-model.md
-- section 4.2, M1-34). Tokens are looked up by their HMAC only.

-- name: InsertFamily :exec
INSERT INTO refresh_token_families (id, session_id, dpop_jkt, current_generation, created_at)
VALUES (@id, @session_id, @dpop_jkt, @current_generation, @created_at);

-- name: InsertRefreshToken :exec
INSERT INTO refresh_tokens (id, family_id, generation, token_hash, issued_at, expires_at)
VALUES (@id, @family_id, @generation, @token_hash, @issued_at, @expires_at);

-- name: GetRefreshTokenByHash :one
SELECT sqlc.embed(t), f.session_id
FROM refresh_tokens t
JOIN refresh_token_families f ON f.id = t.family_id
WHERE t.token_hash = @token_hash;

-- name: GetActiveFamily :one
SELECT * FROM refresh_token_families WHERE session_id = @session_id AND revoked_at IS NULL;

-- name: LockFamily :one
SELECT * FROM refresh_token_families WHERE id = @id FOR UPDATE;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE family_id = @family_id AND generation = @generation;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens SET used_at = @used_at
WHERE family_id = @family_id AND generation = @generation AND used_at IS NULL;

-- name: AdvanceFamily :exec
UPDATE refresh_token_families SET current_generation = @current_generation, rotated_at = @rotated_at
WHERE id = @id;

-- name: RevokeFamily :exec
UPDATE refresh_token_families SET revoked_at = @revoked_at, revoke_reason = @revoke_reason
WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeActiveFamiliesOfSessions :exec
UPDATE refresh_token_families SET revoked_at = @revoked_at, revoke_reason = @revoke_reason
WHERE session_id = ANY(@session_ids::uuid[]) AND revoked_at IS NULL;
