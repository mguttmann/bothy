-- SPDX-License-Identifier: Apache-2.0
-- Every parameter form of the visibility predicate, with and without a cast.

-- name: ByPositional :many
SELECT id FROM messages WHERE channel_id = ANY($1::uuid[]);

-- name: ByAtName :many
SELECT id FROM messages WHERE channel_id = ANY (@visible::uuid[]);

-- name: BySqlcArg :many
SELECT id FROM threads WHERE channel_id = ANY(sqlc.arg(visible_channels)::uuid[]);

-- name: BySqlcNarg :many
SELECT id FROM threads WHERE space_id = ANY(sqlc.narg(visible_spaces));

-- name: ByPositionalNoCast :many
SELECT id FROM mentions WHERE channel_id = ANY($2);

-- name: SpacesByKey :many
SELECT id, name FROM spaces WHERE archived_at IS NULL AND id = ANY(@visible_spaces::uuid[]);
