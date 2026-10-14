-- SPDX-License-Identifier: Apache-2.0
-- Queries on agents (agents module, M1-51; docs/spec/03-datenmodell.md section 4.4). Only this
-- module reads and writes the table (03 Grundsatz 12).

-- name: GetAgent :one
SELECT * FROM agents WHERE principal_id = @principal_id;

-- name: LockAgent :one
-- The record locked for a change (03 section 6.3: domain rows after permission_version).
SELECT * FROM agents WHERE principal_id = @principal_id FOR UPDATE;

-- name: LockAgents :many
-- The records of a list locked in ascending id (BR-08, docs/DECISIONS.md M1-51).
SELECT * FROM agents WHERE principal_id = ANY (@ids::uuid[]) ORDER BY principal_id FOR UPDATE;

-- name: LockPersonalAgents :many
-- Every personal record locked in ascending id, disabled agents included (10 section 9.1).
SELECT * FROM agents WHERE billing_mode = 'personal' ORDER BY principal_id FOR UPDATE;

-- name: ListAgents :many
-- One batch of candidates of listAgents, keyset on principal_id ascending.
SELECT * FROM agents
WHERE (sqlc.narg(after_id)::uuid IS NULL OR principal_id > sqlc.narg(after_id)::uuid)
ORDER BY principal_id
LIMIT @row_limit;

-- name: ListPersonalAgents :many
-- The personal records for the I1 list of the policy source; the status filter runs through
-- identity.
SELECT * FROM agents WHERE billing_mode = 'personal' ORDER BY principal_id;

-- name: InsertAgent :exec
INSERT INTO agents (principal_id, adapter_kind, description, operator_config, billing_mode, credential_holder_id, tool_class, version)
VALUES (@principal_id, @adapter_kind, @description, @operator_config, @billing_mode, sqlc.narg(credential_holder_id), @tool_class, @version);

-- name: UpdateAgent :exec
UPDATE agents
SET description = @description,
    operator_config = @operator_config,
    billing_mode = @billing_mode,
    credential_holder_id = sqlc.narg(credential_holder_id),
    billing_mode_changed_at = sqlc.narg(billing_mode_changed_at),
    billing_mode_changed_by = sqlc.narg(billing_mode_changed_by),
    tool_class = @tool_class,
    version = @version
WHERE principal_id = @principal_id;
