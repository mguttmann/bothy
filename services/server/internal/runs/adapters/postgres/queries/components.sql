-- SPDX-License-Identifier: Apache-2.0
-- message_components (docs/spec/03-datenmodell.md section 4.8, M2-15). The table has no
-- channel_id: every read names the card messages or the components of a message whose visibility
-- the use case checked first (authorize on the message or the component path), or that
-- messaging loaded for an event of the same transaction, or that a job or the end of a run names
-- (tools/lint/sql_visibility.yaml lists each read, 07 7.4 rule 1). The lock order of the
-- README holds: threads, runs, messages, then message_components in ascending id.

-- name: InsertComponent :exec
INSERT INTO message_components (id, message_id, custom_id, type, definition, position, single_use, required_capability,
    restricted_to, run_id, state, expires_at, version)
VALUES (sqlc.arg(id), sqlc.arg(message_id), sqlc.arg(custom_id), sqlc.arg(type), sqlc.arg(definition), sqlc.arg(position),
    sqlc.arg(single_use), sqlc.narg(required_capability), sqlc.arg(restricted_to), sqlc.narg(run_id), 'active',
    sqlc.arg(expires_at), 1);

-- name: ComponentsOfMessages :many
-- The components of a page or an answer in one statement, ordered by position per message.
SELECT * FROM message_components
WHERE message_id = ANY(sqlc.arg(message_ids)::uuid[])
ORDER BY message_id, position;

-- name: GetComponent :one
SELECT * FROM message_components WHERE id = sqlc.arg(id);

-- name: LockComponent :one
SELECT * FROM message_components WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: LockSingleUseGroup :many
-- The statement of 03 "Einmal-Regel je Nachricht": the active single_use components of one
-- message in ascending id, so two concurrent clicks serialize on the same rows in the same order.
SELECT * FROM message_components
WHERE message_id = sqlc.arg(message_id) AND single_use AND state = 'active'
ORDER BY id
FOR UPDATE;

-- name: LockSingleUseSet :many
-- Every single_use component of one message in ascending id, whatever its state: a state change
-- of updateComponent decides against the whole set (09 11.3).
SELECT * FROM message_components
WHERE message_id = sqlc.arg(message_id) AND single_use
ORDER BY id
FOR UPDATE;

-- name: LockDueComponents :many
-- The active components of one message whose expires_at has passed, for the expiry job.
SELECT * FROM message_components
WHERE message_id = sqlc.arg(message_id) AND state = 'active' AND expires_at <= sqlc.arg(now)::timestamptz
ORDER BY id
FOR UPDATE;

-- name: NextComponentExpiry :one
-- The earliest expires_at of the active components of one message; NULL when none is left.
SELECT min(expires_at)::timestamptz AS next_at FROM message_components
WHERE message_id = sqlc.arg(message_id) AND state = 'active';

-- name: SetComponent :one
-- A change of state, definition or use; the caller holds the row lock and passes the whole new
-- row content, version rises by one (03 message_components.version).
UPDATE message_components
SET state = sqlc.arg(state)::text,
    definition = sqlc.arg(definition)::jsonb,
    used_by = sqlc.narg(used_by)::uuid,
    used_at = sqlc.narg(used_at)::timestamptz,
    version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ActiveComponentMessagesOfRun :many
-- The card messages with active components of one run, ascending, over the partial index
-- message_components_run_active_idx (09 8 point 5).
SELECT DISTINCT message_id FROM message_components
WHERE run_id = sqlc.arg(run_id) AND state = 'active'
ORDER BY message_id;

-- name: LockActiveRunComponents :many
SELECT * FROM message_components
WHERE run_id = sqlc.arg(run_id) AND message_id = sqlc.arg(message_id) AND state = 'active'
ORDER BY id
FOR UPDATE;

-- name: DeleteComponentsOfMessage :execrows
-- The components of a deleted message; its interactions follow by cascade and
-- runs.triggering_interaction_id becomes NULL (03 messages "Deletion ... deletes ... components").
DELETE FROM message_components WHERE message_id = sqlc.arg(message_id);
