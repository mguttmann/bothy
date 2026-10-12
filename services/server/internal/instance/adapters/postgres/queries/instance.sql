-- SPDX-License-Identifier: Apache-2.0

-- name: LoadInstance :one
SELECT instance_id, setup_completed_at, schema_version, min_server_version, sync_epoch, created_at
FROM instance WHERE id = 1;

-- name: EnsureInstance :exec
-- Creates the row unless it exists; concurrent bothy migrate runs are safe. schema_version is set
-- by SetVersions in the same transaction.
INSERT INTO instance (id, instance_id, schema_version, sync_epoch)
VALUES (1, sqlc.arg(instance_id), 0, sqlc.arg(sync_epoch))
ON CONFLICT (id) DO NOTHING;

-- name: SetVersions :exec
-- Never lowers schema_version: an older binary racing a newer one leaves the row unchanged.
UPDATE instance
SET schema_version = sqlc.arg(schema_version)::bigint, min_server_version = sqlc.arg(min_server_version)::text
WHERE id = 1 AND schema_version <= sqlc.arg(schema_version)::bigint;
