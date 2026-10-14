-- SPDX-License-Identifier: Apache-2.0
-- Queries of the table hosts (M2-39, docs/spec/03-datenmodell.md section 4.14).

-- name: GetHost :one
SELECT * FROM hosts WHERE id = @id;

-- name: LockHost :one
-- The host row of a write, locked until the end of the unit of work (03 section 6.3).
SELECT * FROM hosts WHERE id = @id FOR UPDATE;

-- name: ListHostsPage :many
-- One keyset page of hosts ordered by lower(name), then id (listHosts): every host for an owner
-- (all_hosts), the given ids otherwise.
SELECT * FROM hosts
WHERE (@all_hosts::boolean OR id = ANY (@host_ids::uuid[]))
  AND (lower(name), id) > (lower(@after_name::text), @after_id::uuid)
ORDER BY lower(name), id
LIMIT @max_rows;

-- name: ListHostIDs :many
-- Every host, ascending (the host source of authz).
SELECT id FROM hosts ORDER BY id;

-- name: ListHostIDsAssignedTo :many
-- The hosts of a holder, ascending (the host source of authz).
SELECT id FROM hosts WHERE assigned_user_id = @user_id ORDER BY id;

-- name: InsertHost :exec
INSERT INTO hosts (id, name, state, assigned_user_id, pairing_code_hash, pairing_expires_at, created_by, created_at)
VALUES (@id, @name, @state, @assigned_user_id, @pairing_code_hash, @pairing_expires_at, @created_by, @created_at);

-- name: UpdateHost :execrows
-- Writes the columns host.manage changes (name, holder, state and its timestamps) and the version.
UPDATE hosts
SET name = @name, assigned_user_id = @assigned_user_id, state = @state, disabled_at = @disabled_at,
    disabled_reason = @disabled_reason, retired_at = @retired_at, version = @version
WHERE id = @id;
