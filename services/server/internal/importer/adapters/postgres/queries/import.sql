-- SPDX-License-Identifier: Apache-2.0
-- Queries of the importer (M8-03): import_sources and import_items, read and written only here
-- (03 Grundsatz 12).

-- name: InsertSource :one
-- A new source is a draft (03 import_sources.state).
INSERT INTO import_sources (id, kind, name, config, state, created_by, created_at)
VALUES (@id, @kind, @name, @config, 'draft', @created_by, @created_at)
RETURNING *;

-- name: GetSource :one
SELECT * FROM import_sources WHERE id = @id;

-- name: LockSource :one
-- Every change of a source locks its row first: state changes, the ownership list, batches and
-- the cursor of one source are serialized (M8-03, D-M8-03-1).
SELECT * FROM import_sources WHERE id = @id FOR UPDATE;

-- name: SaveSource :exec
UPDATE import_sources
SET state = @state, config = @config, stats = @stats, cursor = @cursor,
    started_at = sqlc.narg(started_at), finished_at = sqlc.narg(finished_at)
WHERE id = @id;

-- name: ListSources :many
-- Newest first, keyset (created_at DESC, id DESC).
SELECT * FROM import_sources
WHERE NOT sqlc.arg(has_after)::boolean
   OR (created_at, id) < (sqlc.arg(after_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: InsertPendingItem :one
-- The pending row of a new origin key (22 7.2); a key that exists returns no row, and a key a
-- concurrent batch inserted makes this statement wait for that batch's commit.
INSERT INTO import_items (id, source_id, origin_key, item_kind, state, content_sha256, created_at, updated_at)
VALUES (@id, @source_id, @origin_key, @item_kind, 'pending', sqlc.narg(content_sha256), @created_at, @created_at)
ON CONFLICT (source_id, origin_key) DO NOTHING
RETURNING *;

-- name: LockItemByKey :one
SELECT * FROM import_items WHERE source_id = @source_id AND origin_key = @origin_key FOR UPDATE;

-- name: GetItemByKey :one
SELECT * FROM import_items WHERE source_id = @source_id AND origin_key = @origin_key;

-- name: FinishItem :one
-- The pending item becomes posted (with its target) or skipped in the same transaction as the
-- target (22 7.2).
UPDATE import_items
SET state = @state, target_kind = sqlc.narg(target_kind), target_id = sqlc.narg(target_id), updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: ListItems :many
-- Ascending id, optionally of one state; the index (source_id, state) serves the filter.
SELECT * FROM import_items
WHERE source_id = @source_id
  AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
  AND id > sqlc.arg(after_id)::uuid
ORDER BY id
LIMIT sqlc.arg(page_limit);

-- name: ItemsOfKind :many
-- The items of one kind of a source in one state (the channel items of the completion check).
SELECT * FROM import_items WHERE source_id = @source_id AND item_kind = @item_kind AND state = @state ORDER BY id;

-- name: ControlledChannel :one
-- Whether the channel is the target of a posted channel item of a source that is neither
-- completed nor failed (22 12.1); the index (target_kind, target_id) serves it.
SELECT EXISTS (
    SELECT 1 FROM import_items i JOIN import_sources s ON s.id = i.source_id
    WHERE i.target_kind = 'channel' AND i.target_id = @channel_id AND i.item_kind = 'channel'
      AND i.state = 'posted' AND s.state NOT IN ('completed', 'failed')
) AS controlled;

-- name: OriginOfTarget :one
-- "Where does this object come from": the oldest posted item of the target.
SELECT * FROM import_items WHERE target_kind = @target_kind AND target_id = @target_id AND state = 'posted'
ORDER BY id LIMIT 1;
