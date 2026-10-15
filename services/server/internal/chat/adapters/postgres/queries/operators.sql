-- SPDX-License-Identifier: Apache-2.0
-- Queries of the operators (chat module, M1-54; docs/spec/03-data-model.md section 4.5 and
-- section 6.4, docs/spec/10-backend-policy.md section 5.3). Only this module reads and writes
-- the operator columns of spaces, channels and threads (03 principle 12). The effective operator
-- of a channel is operator_inherit ? spaces.default_operator_agent_id : operator_agent_id.

-- name: SaveSpaceOperator :exec
-- space.operator.set writes the default operator and the version.
UPDATE spaces SET default_operator_agent_id = sqlc.narg(default_operator_agent_id), version = @version WHERE id = @id;

-- name: SaveChannelOperator :exec
-- channel.operator.set writes the operator columns and the version.
UPDATE channels SET operator_inherit = @operator_inherit, operator_agent_id = sqlc.narg(operator_agent_id), version = @version
WHERE id = @id;

-- name: TouchChannel :exec
-- An inheriting channel whose effective operator changed with the default of its space: only the
-- version (04 "Zustellgarantie" point 3).
UPDATE channels SET version = @version WHERE id = @id;

-- name: LockInheritingChannels :many
-- The channels of a space that inherit the default operator and are not archived, locked in
-- ascending id after the space row (03 section 6.3).
SELECT sqlc.embed(c), s.default_operator_agent_id AS space_operator_id,
       (s.archived_at IS NOT NULL)::boolean AS space_archived, t.id AS main_thread_id
FROM channels c
JOIN spaces s ON s.id = c.space_id
LEFT JOIN threads t ON t.channel_id = c.id AND t.kind = 'main'
WHERE c.space_id = @space_id AND c.operator_inherit AND c.archived_at IS NULL
ORDER BY c.id
FOR UPDATE OF c;

-- name: ServedChannels :many
-- The channels agent_id serves for I1 (10 section 5.2 points 1 and 2, BR-02): channels that are
-- not archived in a space that is not archived whose effective operator is the agent, and those
-- with a thread of kind thread whose fixed operator is the agent, open or closed (the index
-- threads (operator_agent_id, channel_id) without a condition on state). dm channels have no
-- space and never match.
SELECT c.id, c.space_id
FROM channels c
JOIN spaces s ON s.id = c.space_id
WHERE c.archived_at IS NULL AND s.archived_at IS NULL
  AND ((c.operator_inherit AND s.default_operator_agent_id = @agent_id)
       OR (NOT c.operator_inherit AND c.operator_agent_id = @agent_id))
UNION
SELECT c.id, c.space_id
FROM threads t
JOIN channels c ON c.id = t.channel_id
JOIN spaces s ON s.id = c.space_id
WHERE t.kind = 'thread' AND t.operator_agent_id = @agent_id
  AND c.archived_at IS NULL AND s.archived_at IS NULL;

-- name: DefaultOperatedSpaces :many
-- The spaces that are not archived whose default operator is agent_id, also without a channel
-- (10 section 5.2 point 3, 03 section 6.4 point 3), ascending.
SELECT id FROM spaces WHERE default_operator_agent_id = @agent_id AND archived_at IS NULL ORDER BY id;
