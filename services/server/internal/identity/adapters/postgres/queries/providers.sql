-- SPDX-License-Identifier: Apache-2.0
-- Queries on identity_providers (identity, docs/spec/03-data-model.md section 4.2, M1-36).

-- name: InsertProvider :one
INSERT INTO identity_providers (id, kind, name, config, enabled, allow_provisioning, created_at, updated_at)
VALUES (@id, @kind, @name, @config, @enabled, @allow_provisioning, @at, @at)
RETURNING *;

-- name: GetProvider :one
SELECT * FROM identity_providers WHERE id = @id;

-- name: LockProvider :one
SELECT * FROM identity_providers WHERE id = @id FOR UPDATE;

-- name: GetEnabledProvider :one
SELECT * FROM identity_providers WHERE id = @id AND enabled;

-- name: ShareEnabledProvider :one
-- Inside a login unit: the share lock makes a concurrent disable wait until the login has
-- committed its session, so the disable ends that session too (08 section 3.3).
SELECT * FROM identity_providers WHERE id = @id AND enabled FOR SHARE;

-- name: ListProviders :many
SELECT * FROM identity_providers ORDER BY lower(name), id;

-- name: ListEnabledProviders :many
SELECT * FROM identity_providers WHERE enabled ORDER BY lower(name), id;

-- name: UpdateProvider :one
UPDATE identity_providers
SET name = @name, config = @config, enabled = @enabled, allow_provisioning = @allow_provisioning, updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: DeleteProvider :execrows
DELETE FROM identity_providers WHERE id = @id;

-- name: LiveSessionsOfProvider :many
-- The sessions that are neither revoked nor past their end and were started with an identity of
-- the provider, ascending.
SELECT s.id FROM auth_sessions s
JOIN user_identities i ON i.id = s.identity_id
WHERE i.provider_id = @provider_id AND s.revoked_at IS NULL AND s.expires_at > @now
ORDER BY s.id;
