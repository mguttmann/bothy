-- SPDX-License-Identifier: Apache-2.0
-- interactions (docs/spec/03-data-model.md section 4.8, M2-15). The table has no channel_id:
-- every read names one interaction, or one (principal, client_interaction_id), whose component
-- path the use case checked with authorize first, or that a delivery names
-- (tools/lint/sql_visibility.yaml lists each read, 07 7.4 rule 1).

-- name: InsertInteraction :one
INSERT INTO interactions (id, component_id, message_id, principal_id, client_interaction_id, "values", state,
    created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(component_id), sqlc.arg(message_id), sqlc.arg(principal_id), sqlc.arg(client_interaction_id),
    sqlc.arg(values), 'received', sqlc.arg(at), sqlc.arg(at))
RETURNING *;

-- name: InteractionByClientID :one
-- The idempotency lookup of createInteraction (04 5.1), under the locks of the card.
SELECT * FROM interactions
WHERE principal_id = sqlc.arg(principal_id) AND client_interaction_id = sqlc.arg(client_interaction_id);

-- name: GetInteraction :one
SELECT * FROM interactions WHERE id = sqlc.arg(id);

-- name: LockInteraction :one
SELECT * FROM interactions WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: SetInteractionState :one
-- The state of an interaction: dispatched or failed by the delivery, the final state and the
-- response of respondToInteraction; the caller holds the row lock. A NULL response keeps the
-- stored one.
UPDATE interactions
SET state = sqlc.arg(state)::text,
    response = COALESCE(sqlc.narg(response)::jsonb, response),
    updated_at = sqlc.arg(at)::timestamptz
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RunOfInteraction :one
-- The run an interaction opened (runs.triggering_interaction_id); a component bound to a run names
-- that run itself.
SELECT id FROM runs WHERE triggering_interaction_id = sqlc.arg(interaction_id);
