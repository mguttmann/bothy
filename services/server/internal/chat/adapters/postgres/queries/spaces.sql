-- SPDX-License-Identifier: Apache-2.0
-- Queries on spaces (chat module, M1-52; docs/spec/03-data-model.md section 4.5). Only this
-- module reads and writes the table (03 Grundsatz 12). Orders are sort_key (COLLATE "C" from the
-- column), then id.

-- name: GetSpace :one
-- One space for authorize resolution and for a resource authorize checks next (07 section 7.4
-- rule 1, exception list).
SELECT * FROM spaces WHERE id = @id;

-- name: LockSpace :one
-- The space locked for a change (03 section 6.3: domain rows after permission_version).
SELECT * FROM spaces WHERE id = @id FOR UPDATE;

-- name: LockActiveSpaces :many
-- Every space that is not archived locked in ascending id before create and move compute a key,
-- so concurrent writers keep the keys distinct (docs/DECISIONS.md M1-52).
SELECT id FROM spaces WHERE archived_at IS NULL ORDER BY id FOR UPDATE;

-- name: ActiveSpaceKeys :many
-- The global order of the spaces that are not archived, the neighbours of create and move.
SELECT id, sort_key FROM spaces WHERE archived_at IS NULL ORDER BY sort_key, id;

-- name: ListVisibleSpaces :many
-- listSpaces with the predicate of 07 section 7.4: every space for an instance-wide reader,
-- else the visible ones of Scope; authorize then checks each row.
SELECT * FROM spaces
WHERE archived_at IS NULL AND (@all_spaces::boolean OR id = ANY (@visible::uuid[]))
ORDER BY sort_key, id;

-- name: SpaceIDs :many
-- Every space, archived ones included, for the structure source of authz.
SELECT id FROM spaces ORDER BY id;

-- name: InsertSpace :exec
INSERT INTO spaces (id, name, icon, color, sort_key, created_by, created_at, version)
VALUES (@id, @name, sqlc.narg(icon), sqlc.narg(color), @sort_key, @created_by, @created_at, @version);

-- name: UpdateSpace :exec
UPDATE spaces SET name = @name, icon = sqlc.narg(icon), color = sqlc.narg(color), version = @version WHERE id = @id;

-- name: MoveSpace :exec
-- A move writes exactly this one row.
UPDATE spaces SET sort_key = @sort_key, version = @version WHERE id = @id;

-- name: ArchiveSpace :exec
UPDATE spaces SET archived_at = @archived_at, version = @version WHERE id = @id;

-- name: ShareSpace :one
-- The space held FOR SHARE while notify writes a preference below it (M3-18): an archive holds
-- the row FOR UPDATE, so the write either commits before it or sees the archive.
SELECT * FROM spaces WHERE id = @id FOR SHARE;
