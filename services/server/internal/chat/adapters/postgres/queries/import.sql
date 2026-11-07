-- SPDX-License-Identifier: Apache-2.0
-- Queries of the import structure and the read-only switch (M8-03).

-- name: SaveChannelAutonomousRuns :exec
-- channel.autonomous_runs.set writes the switch and the version (AR-04).
UPDATE channels SET allow_autonomous_runs = @allow_autonomous_runs, version = @version WHERE id = @id;

-- name: SaveChannelReadOnly :exec
-- channel.read_only.set and setImportLead write the flag and the version.
UPDATE channels SET read_only = @read_only, version = @version WHERE id = @id;
