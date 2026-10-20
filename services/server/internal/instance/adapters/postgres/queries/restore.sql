-- SPDX-License-Identifier: Apache-2.0
-- The identity of the database cluster and bothy restore finalize (M7-04, R-DM-50, 03 6.7).

-- name: ClusterIdentity :one
-- system_identifier as decimal text (unsigned 64-bit, does not fit bigint) and the timeline of the
-- last checkpoint. PostgreSQL 18 grants EXECUTE on both functions to PUBLIC by default.
SELECT s.system_identifier::text AS system_identifier, c.timeline_id::bigint AS timeline_id
FROM pg_control_system() AS s, pg_control_checkpoint() AS c;

-- name: FillIdentity :execrows
-- bothy migrate up: stores the identity only while both columns are empty (03 4.1); a stored
-- identity changes only through bothy restore finalize.
UPDATE instance
SET pg_system_identifier = sqlc.arg(system_identifier)::text, pg_timeline_id = sqlc.arg(timeline_id)::bigint
WHERE id = 1 AND pg_system_identifier IS NULL AND pg_timeline_id IS NULL;

-- name: FinalizeInstance :execrows
-- bothy restore finalize: the epoch and the identity in the transaction that moves the cursors.
UPDATE instance
SET sync_epoch = sqlc.arg(sync_epoch), pg_system_identifier = sqlc.arg(system_identifier)::text,
    pg_timeline_id = sqlc.arg(timeline_id)::bigint
WHERE id = 1;

-- name: OtherSessionsOfRole :one
-- Sessions of a role on this database other than the caller's own (brief M7-04 requirement 16).
-- usename and datname of every row are visible to every role.
SELECT count(*)::bigint FROM pg_catalog.pg_stat_activity
WHERE usename::text = sqlc.arg(role)::text AND datname = current_database() AND pid <> pg_backend_pid();
