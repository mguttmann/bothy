-- SPDX-License-Identifier: Apache-2.0
-- Queries of the definitions of built-in agents (agents module, A1-02; docs/spec/03-data-model.md
-- table agent_definitions, docs/spec/26-builtin-agents.md 4.1). Only this module reads and writes
-- the table (03 principle 12). Rows are never updated (R-DM-59); a write locks the agents row
-- first, so the primary key orders the versions without a lock of its own (03 section 6.3).

-- name: GetCurrentDefinition :one
-- The current definition: the highest version of the agent, the last entry of the primary key.
SELECT agent_id, version, definition, origin_kind, created_at, created_by
FROM agent_definitions WHERE agent_id = @agent_id ORDER BY version DESC LIMIT 1;

-- name: GetDefinitionAt :one
-- One stored version, for undo (26 8.4).
SELECT agent_id, version, definition, origin_kind, created_at, created_by
FROM agent_definitions WHERE agent_id = @agent_id AND version = @version;

-- name: InsertDefinition :exec
-- The next version, in the transaction of agent.update.
INSERT INTO agent_definitions (agent_id, version, definition, origin_kind, created_at, created_by)
VALUES (@agent_id, @version, @definition, @origin_kind, @created_at, @created_by);

-- name: ListDefinitionVersions :many
-- One page of versions below a keyset position, newest first, without the definition document.
SELECT agent_id, version, origin_kind, created_at, created_by
FROM agent_definitions WHERE agent_id = @agent_id AND version < @before_version
ORDER BY version DESC LIMIT @row_limit;
