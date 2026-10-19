-- SPDX-License-Identifier: Apache-2.0
-- Queries on app_installations (apps module, docs/spec/03-data-model.md section 4.4).

-- name: LatestInstallation :one
SELECT sqlc.embed(i) FROM app_installations i
WHERE i.app_id = @app_id
ORDER BY i.installed_at DESC, i.id DESC
LIMIT 1;

-- name: LockLiveInstallation :one
SELECT sqlc.embed(i) FROM app_installations i
WHERE i.app_id = @app_id AND i.state <> 'uninstalled'
FOR UPDATE;

-- name: ActiveInstallation :one
-- Read by identity on every request of an app token and for the rights snapshot (M2-24 design
-- notes 10 and 11).
SELECT sqlc.embed(i) FROM app_installations i
WHERE i.app_id = @app_id AND i.state = 'active';

-- name: ShareActiveInstallation :one
-- createToken for an app: FOR SHARE serializes with an uninstall that holds the row FOR UPDATE
-- (M2-24 design note 10 a).
SELECT sqlc.embed(i) FROM app_installations i
WHERE i.app_id = @app_id AND i.state = 'active'
FOR SHARE;

-- name: InsertInstallation :one
INSERT INTO app_installations (id, app_id, granted_scopes, event_subscriptions, state, installed_by)
VALUES (@id, @app_id, @granted_scopes, @event_subscriptions, @state, @installed_by)
RETURNING *;

-- name: UpdateInstallationGrants :execrows
UPDATE app_installations SET granted_scopes = @granted_scopes, event_subscriptions = @event_subscriptions
WHERE id = @id;

-- name: SetInstallationState :execrows
UPDATE app_installations SET state = @state, uninstalled_at = sqlc.narg(uninstalled_at), uninstalled_by = sqlc.narg(uninstalled_by)
WHERE id = @id;
