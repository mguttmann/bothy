-- SPDX-License-Identifier: Apache-2.0
-- Queries on incoming_webhooks (apps module, M2-27, docs/spec/03-datenmodell.md incoming_webhooks).
-- A post holds the webhook row FOR SHARE for its unit of work; a revoke, an uninstall and a channel
-- archive update the row and so wait for posts in flight (M2-27 design note 6 a).

-- name: ListActiveIncomingByChannel :many
-- listIncomingWebhooks: the active webhooks of one channel (the partial index of 03).
SELECT * FROM incoming_webhooks
WHERE channel_id = @channel_id AND revoked_at IS NULL
ORDER BY created_at, id;

-- name: InsertIncoming :one
INSERT INTO incoming_webhooks (id, app_id, channel_id, thread_id, name, token_hash, created_by)
VALUES (@id, @app_id, @channel_id, sqlc.narg(thread_id), @name, @token_hash, @created_by)
RETURNING *;

-- name: GetIncoming :one
SELECT * FROM incoming_webhooks WHERE id = @id;

-- name: GetIncomingForAuth :one
-- The token check: the webhook with the installation of its app that is not uninstalled, by the
-- primary key, so a known and an unknown id cost one indexed lookup each (AZ-P4).
SELECT sqlc.embed(w), i.id AS installation_id, i.state AS installation_state, i.granted_scopes
FROM incoming_webhooks w
JOIN app_installations i ON i.app_id = w.app_id AND i.state <> 'uninstalled'
WHERE w.id = @id;

-- name: LockIncomingForPost :one
SELECT * FROM incoming_webhooks WHERE id = @id FOR SHARE;

-- name: RevokeIncoming :execrows
UPDATE incoming_webhooks SET revoked_at = @revoked_at WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeIncomingByApp :execrows
-- uninstallApp revokes every active webhook of the app in its unit (11 section 4.3).
UPDATE incoming_webhooks SET revoked_at = @revoked_at WHERE app_id = @app_id AND revoked_at IS NULL;

-- name: RevokeIncomingByChannel :execrows
-- archiveChannel revokes the active webhooks of the channel in its unit (D-M2-27-14).
UPDATE incoming_webhooks SET revoked_at = @revoked_at WHERE channel_id = @channel_id AND revoked_at IS NULL;

-- name: TouchIncomingLastUsed :execrows
-- last_used_at moves at most once per identity.last_active_granularity (D-M2-27-9).
UPDATE incoming_webhooks SET last_used_at = @used_at
WHERE id = @id AND revoked_at IS NULL AND (last_used_at IS NULL OR last_used_at < @older_than);
