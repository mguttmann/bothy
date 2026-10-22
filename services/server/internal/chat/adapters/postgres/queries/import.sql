-- SPDX-License-Identifier: Apache-2.0
-- Queries of the import structure and the read-only switch (M8-03).

-- name: SaveChannelReadOnly :exec
-- channel.read_only.set and setImportLead write the flag and the version.
UPDATE channels SET read_only = @read_only, version = @version WHERE id = @id;
