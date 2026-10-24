-- SPDX-License-Identifier: Apache-2.0
-- Queries of the gateway across several api nodes (M7-17, R-DM-51): the shared presence state
-- gateway_presence, which only this module reads and writes (03 principle 12), and the signals on
-- the channel ephemeral. None appends an event, so none takes event_seq (02 section 11.7).

-- name: UpsertPresence :exec
-- The rows of the live connections of one node, every one with updated_at = @at: an upsert, so a
-- row lost with the UNLOGGED table comes back at the next flush. focus is one uuid[] literal per
-- row, because an array of arrays must be rectangular; the five arrays have one entry per row
-- and unnest in the select list zips them.
INSERT INTO gateway_presence (gateway_session_id, node_id, principal_id, device_id, focus_thread_ids, idle, updated_at)
SELECT r.session_id, @node_id::uuid, r.principal_id, r.device_id, r.focus::uuid[], r.idle, @at::timestamptz
FROM (
    SELECT unnest(@session_ids::uuid[]) AS session_id, unnest(@principal_ids::uuid[]) AS principal_id,
        unnest(@device_ids::uuid[]) AS device_id, unnest(@focus::text[]) AS focus, unnest(@idle::boolean[]) AS idle
) AS r
ON CONFLICT (gateway_session_id) DO UPDATE
    SET node_id = EXCLUDED.node_id, principal_id = EXCLUDED.principal_id, device_id = EXCLUDED.device_id,
        focus_thread_ids = EXCLUDED.focus_thread_ids, idle = EXCLUDED.idle, updated_at = EXCLUDED.updated_at;

-- name: DeleteGonePresence :exec
-- The rows of one node whose connection ended since the last flush.
DELETE FROM gateway_presence WHERE node_id = @node_id AND gateway_session_id <> ALL(@session_ids::uuid[]);

-- name: DeleteStalePresence :exec
-- The rows of every node that stopped refreshing (cluster.presence_ttl).
DELETE FROM gateway_presence WHERE updated_at < @stale_before;

-- name: DeleteNodePresence :exec
-- Every row of one node, at its orderly shutdown.
DELETE FROM gateway_presence WHERE node_id = @node_id;

-- name: PresenceOf :many
-- The fresh rows of the requested people, the newest first per person, so the first row of a
-- device wins (its newest connection).
SELECT gateway_session_id, principal_id, device_id, focus_thread_ids, idle
FROM gateway_presence
WHERE principal_id = ANY(@principal_ids::uuid[]) AND updated_at >= @fresh_after
ORDER BY principal_id, updated_at DESC, gateway_session_id DESC;

-- name: NotifyEphemeral :exec
-- One gateway signal on the channel ephemeral (03 section 6.6): kind, node and IDs only, sent in
-- its own statement.
SELECT pg_notify('ephemeral', sqlc.arg(payload)::text);
