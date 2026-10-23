-- SPDX-License-Identifier: Apache-2.0
-- History reads of the event log for the worker consumers (M3-19 design notes 5 and 16;
-- sync.History). They carry no visibility predicate: the consumer reads like the log tail and
-- every hint is checked with push.deliver before it is sent (07 section 7.4 rule 1).

-- name: ResourceEvents :many
-- The retained events of one resource with one of the types and seq below a bound, in seq
-- order (index event_log_resource_idx on (resource_kind, resource_id)).
SELECT seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id,
       channel_id, thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
       receive_action, payload, truncated, redacted_at, correlation_id, trace_context,
       client_msg_id, permission_version
FROM event_log
WHERE resource_kind = sqlc.arg(resource_kind)::text
  AND resource_id = sqlc.arg(resource_id)::uuid
  AND type = ANY(sqlc.arg(types)::text[])
  AND seq < sqlc.arg(before_seq)::bigint
ORDER BY seq;

-- name: PersonalEvents :many
-- The retained events of one audience with one of the types and seq above a bound, in seq
-- order (index event_log_audience_seq_idx on (audience_principal_id, seq)).
SELECT seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id,
       channel_id, thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
       receive_action, payload, truncated, redacted_at, correlation_id, trace_context,
       client_msg_id, permission_version
FROM event_log
WHERE audience_principal_id = sqlc.arg(audience_principal_id)::uuid
  AND type = ANY(sqlc.arg(types)::text[])
  AND seq > sqlc.arg(after_seq)::bigint
ORDER BY seq;
