-- SPDX-License-Identifier: Apache-2.0
-- Queries on message_templates (messaging, docs/spec/03-data-model.md section 4.6, brief M1-60).

-- name: ListPersonalTemplates :many
-- The personal templates of a person, by lower(name), then id (listMyTemplates).
SELECT * FROM message_templates
WHERE owner_user_id = @owner_user_id AND deleted_at IS NULL
ORDER BY lower(name), id;

-- name: ListChannelTemplates :many
-- The personal templates of the caller and the templates of one visible channel, by lower(name),
-- then id (listTemplates, brief note 7).
SELECT * FROM message_templates
WHERE deleted_at IS NULL
  AND (owner_user_id = @caller_id OR (channel_id = @channel_id AND channel_id = ANY (@visible_channel_ids::uuid[])))
ORDER BY lower(name), id;

-- name: GetVisibleTemplate :one
-- A template of the caller or of a visible channel, locked for the change (brief notes 10 and
-- 11). A deleted one only with include_deleted. A foreign personal template and one in an
-- invisible channel are no row, like an unknown id.
SELECT * FROM message_templates
WHERE id = @id
  AND (owner_user_id = @caller_id OR channel_id = ANY (@visible_channel_ids::uuid[]))
  AND (@include_deleted::boolean OR deleted_at IS NULL)
FOR UPDATE;

-- name: CountPersonalTemplates :one
-- The templates of a person without deleted ones (template.max_per_scope).
SELECT count(*)::int FROM message_templates WHERE owner_user_id = @owner_user_id AND deleted_at IS NULL;

-- name: CountChannelTemplates :one
-- The templates of a channel without deleted ones (template.max_per_scope).
SELECT count(*)::int FROM message_templates WHERE channel_id = @channel_id AND deleted_at IS NULL;

-- name: InsertTemplate :one
INSERT INTO message_templates (id, owner_user_id, channel_id, name, content_md, created_by, created_at, version)
VALUES (@id, @owner_user_id, @channel_id, @name, @content_md, @created_by, @created_at, 1)
RETURNING *;

-- name: UpdateTemplate :one
-- A change of name and content raises the version (06 "Concurrency").
UPDATE message_templates SET name = @name, content_md = @content_md, version = version + 1
WHERE id = @id
RETURNING *;

-- name: DeleteTemplate :one
-- The soft delete raises the version.
UPDATE message_templates SET deleted_at = @deleted_at, version = version + 1
WHERE id = @id AND deleted_at IS NULL
RETURNING *;
