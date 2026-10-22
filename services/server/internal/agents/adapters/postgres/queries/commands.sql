-- SPDX-License-Identifier: Apache-2.0
-- Queries on commands (agents module, M2-16; docs/spec/03-data-model.md section 4.4). Only this
-- module reads and writes the table (03 Grundsatz 12). The owner kind comes from agents, the other
-- table of this module: an owner with an agents row is an agent, any other owner an app (03
-- commands: "registered by agents or apps").

-- name: InsertCommand :one
INSERT INTO commands (id, owner_principal_id, name, description, scope_kind, scope_space_id, scope_channel_id, options,
    required_level, control, version)
VALUES (@id, @owner_principal_id, @name, @description, @scope_kind, sqlc.narg(scope_space_id), sqlc.narg(scope_channel_id),
    @options, @required_level, @control, 1)
RETURNING created_at;

-- name: GetCommand :one
-- One command, removed or not, with the kind of its owner.
SELECT sqlc.embed(c), (a.principal_id IS NOT NULL)::boolean AS owner_is_agent
FROM commands c
LEFT JOIN agents a ON a.principal_id = c.owner_principal_id
WHERE c.id = @id;

-- name: CountLiveCommands :one
-- The live commands of one owner, for command.max_per_principal; the caller holds the owner's
-- principal row (design note 4 c).
SELECT count(*)::bigint FROM commands WHERE owner_principal_id = @owner_principal_id AND removed_at IS NULL;

-- name: UpdateCommandDefinition :one
-- The compare-and-swap of updateCommand on the live command and its version.
UPDATE commands
SET name = @name, description = @description, scope_kind = @scope_kind, scope_space_id = sqlc.narg(scope_space_id),
    scope_channel_id = sqlc.narg(scope_channel_id), options = @options, required_level = @required_level, control = @control,
    version = version + 1
WHERE id = @id AND version = @version AND removed_at IS NULL
RETURNING version;

-- name: RemoveCommand :one
-- The compare-and-swap of removeCommand: removed_at and the next version on the live command.
UPDATE commands SET removed_at = now(), version = version + 1
WHERE id = @id AND version = @version AND removed_at IS NULL
RETURNING version, removed_at;

-- name: ListCoveringCommands :many
-- The live commands whose scope covers one channel (09 section 10.3): instance, the channel's
-- space, the channel; owner_principal_id and name narrow it to one owner and name when set.
SELECT sqlc.embed(c), (a.principal_id IS NOT NULL)::boolean AS owner_is_agent
FROM commands c
LEFT JOIN agents a ON a.principal_id = c.owner_principal_id
WHERE c.removed_at IS NULL
  AND (c.scope_kind = 'instance'
    OR (c.scope_kind = 'space' AND c.scope_space_id = @space_id)
    OR (c.scope_kind = 'channel' AND c.scope_channel_id = @channel_id))
  AND (sqlc.narg(owner_principal_id)::uuid IS NULL OR c.owner_principal_id = sqlc.narg(owner_principal_id)::uuid)
  AND (sqlc.narg(lower_name)::text IS NULL OR lower(c.name) = sqlc.narg(lower_name)::text)
ORDER BY c.id;
