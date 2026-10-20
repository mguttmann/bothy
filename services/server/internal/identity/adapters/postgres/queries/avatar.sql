-- SPDX-License-Identifier: Apache-2.0
-- Queries of the avatar use cases on principals (identity, M3-10; docs/spec/03-datenmodell.md
-- section 4.2). The row is locked first with LockUser of people.sql (03 6.3: the anchor row before
-- the files rows); principals has no version or updated_at column, so the avatar is the only
-- column written.

-- name: SetPrincipalAvatar :execrows
-- Sets avatar_file_id of the person id, NULL clears it. The foreign key to files keeps an
-- unknown file out; the id comes from the files module, so that is an internal error.
UPDATE principals SET avatar_file_id = sqlc.narg(avatar_file_id)
WHERE id = @id AND kind = 'user';
