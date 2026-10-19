-- SPDX-License-Identifier: Apache-2.0
-- Queries on apps (apps module, docs/spec/03-data-model.md section 4.4).

-- name: GetApp :one
SELECT sqlc.embed(a) FROM apps a WHERE a.principal_id = @principal_id;

-- name: GetAppBySlug :one
SELECT sqlc.embed(a) FROM apps a WHERE lower(a.slug) = lower(@slug);

-- name: LockAppBySlug :one
SELECT sqlc.embed(a) FROM apps a WHERE lower(a.slug) = lower(@slug) FOR UPDATE;

-- name: LockApp :one
SELECT sqlc.embed(a) FROM apps a WHERE a.principal_id = @principal_id FOR UPDATE;

-- name: InsertApp :one
INSERT INTO apps (principal_id, slug, manifest, manifest_version, manifest_sha256, publisher, homepage_url)
VALUES (@principal_id, @slug, @manifest, @manifest_version, @manifest_sha256, @publisher, sqlc.narg(homepage_url))
RETURNING *;

-- name: UpdateApp :one
-- An upgrade or a reinstall replaces the manifest columns; slug and principal stay.
UPDATE apps SET manifest = @manifest, manifest_version = @manifest_version, manifest_sha256 = @manifest_sha256,
    publisher = @publisher, homepage_url = sqlc.narg(homepage_url), updated_at = @updated_at
WHERE principal_id = @principal_id
RETURNING *;

-- name: ListLiveApps :many
-- listApps: the apps with an installation that is not uninstalled, by (lower(slug), principal_id)
-- after the keyset position (M2-24 design note 13).
SELECT sqlc.embed(a), sqlc.embed(i)
FROM apps a
JOIN app_installations i ON i.app_id = a.principal_id AND i.state <> 'uninstalled'
WHERE (lower(a.slug), a.principal_id) > (@after_slug::text, @after_id::uuid)
ORDER BY lower(a.slug), a.principal_id
LIMIT @lim;
