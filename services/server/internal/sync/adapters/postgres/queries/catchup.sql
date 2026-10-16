-- SPDX-License-Identifier: Apache-2.0
-- Catch-up (M1-63; docs/spec/04-sync-protokoll.md section 7.2 point 1). The predicate is the
-- performance, authorize per event the truth: the use case drops every row authorize refuses.
-- The smallest retained seq is MinRetainedSeq of event_log.sql.

-- name: CatchUpPage :many
-- One page after the cursor up to the head: events of a visible channel, of a fully visible
-- space, of a visible space without channel, of any space for an instance-wide reader, the
-- personal events of the caller and the events without space, channel and audience.
SELECT seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id,
       channel_id, thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
       receive_action, payload, truncated, redacted_at, correlation_id, trace_context,
       client_msg_id, permission_version
FROM event_log
WHERE seq > sqlc.arg(after_seq)::bigint AND seq <= sqlc.arg(head_seq)::bigint
  AND (channel_id = ANY(sqlc.arg(visible_channels)::uuid[])
       OR space_id = ANY(sqlc.arg(fully_visible_spaces)::uuid[])
       OR (channel_id IS NULL AND space_id = ANY(sqlc.arg(visible_spaces)::uuid[]))
       OR (sqlc.arg(instance_wide)::boolean AND space_id IS NOT NULL)
       OR audience_principal_id = sqlc.arg(principal)::uuid
       OR (space_id IS NULL AND channel_id IS NULL AND audience_principal_id IS NULL))
ORDER BY seq
LIMIT sqlc.arg(max_rows)::int;
