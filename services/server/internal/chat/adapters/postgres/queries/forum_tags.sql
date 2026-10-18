-- SPDX-License-Identifier: Apache-2.0
-- Queries on forum_tags (chat module, M1-53; docs/spec/03-data-model.md section 4.5). Only this
-- module reads and writes the table (03 principle 12). A tag is never deleted hard: deleted_at.

-- name: ListForumTags :many
-- The tags of a forum that are not deleted, in their order.
SELECT * FROM forum_tags WHERE channel_id = @channel_id AND deleted_at IS NULL ORDER BY sort_key, id;

-- name: GetForumTag :one
SELECT * FROM forum_tags WHERE id = @id;

-- name: InsertForumTag :exec
INSERT INTO forum_tags (id, channel_id, name, color, sort_key, restricted)
VALUES (@id, @channel_id, @name, sqlc.narg(color), @sort_key, @restricted);

-- name: SaveForumTag :exec
-- Writes the columns of updateForumTag, moveForumTag and deleteForumTag.
UPDATE forum_tags
SET name = @name, color = sqlc.narg(color), sort_key = @sort_key, restricted = @restricted, deleted_at = sqlc.narg(deleted_at)
WHERE id = @id;
