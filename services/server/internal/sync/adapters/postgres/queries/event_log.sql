-- SPDX-License-Identifier: Apache-2.0

-- name: InsertEvents :one
-- Inserts the events of one sync hook run in one statement (M1-16); the adapter passes the seqs
-- first, first+1, ... in append order. The text arrays carry '' and resource_versions 0 for NULL,
-- because sqlc maps them to slices of plain values; the payloads arrive as JSON text.
-- occurred_at keeps its default now(), the transaction time. The same statement wakes the log
-- tails with only the highest seq (03 "Notifying the gateway processes"), so the counter
-- lock is held one round trip less; the CTE runs the insert before the notification.
WITH inserted AS (
INSERT INTO event_log (
    seq, id, type, schema_version, actor_kind, actor_principal_id, space_id, channel_id,
    thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
    receive_action, payload, truncated, correlation_id, trace_context, client_msg_id,
    permission_version
)
SELECT r.seq, r.id, r.type, r.schema_version, NULLIF(r.actor_kind, ''), r.actor_principal_id,
       r.space_id, r.channel_id, r.thread_id, r.resource_kind, r.resource_id, NULLIF(r.resource_version, 0),
       r.audience_principal_id, r.receive_action, r.payload::jsonb, r.truncated, r.correlation_id,
       NULLIF(r.trace_context, ''), r.client_msg_id, sqlc.arg(permission_version)::bigint
FROM (
    -- Set-returning functions in one select list run in lock step, so element i of every array
    -- forms row i (PostgreSQL 10 and later).
    SELECT
           unnest(sqlc.arg(seqs)::bigint[]) AS seq,
           unnest(sqlc.arg(ids)::uuid[]) AS id,
           unnest(sqlc.arg(types)::text[]) AS type,
           unnest(sqlc.arg(schema_versions)::smallint[]) AS schema_version,
           unnest(sqlc.arg(actor_kinds)::text[]) AS actor_kind,
           unnest(sqlc.arg(actor_principal_ids)::uuid[]) AS actor_principal_id,
           unnest(sqlc.arg(space_ids)::uuid[]) AS space_id,
           unnest(sqlc.arg(channel_ids)::uuid[]) AS channel_id,
           unnest(sqlc.arg(thread_ids)::uuid[]) AS thread_id,
           unnest(sqlc.arg(resource_kinds)::text[]) AS resource_kind,
           unnest(sqlc.arg(resource_ids)::uuid[]) AS resource_id,
           unnest(sqlc.arg(resource_versions)::bigint[]) AS resource_version,
           unnest(sqlc.arg(audience_principal_ids)::uuid[]) AS audience_principal_id,
           unnest(sqlc.arg(receive_actions)::text[]) AS receive_action,
           unnest(sqlc.arg(payloads)::text[]) AS payload,
           unnest(sqlc.arg(truncated)::boolean[]) AS truncated,
           unnest(sqlc.arg(correlation_ids)::uuid[]) AS correlation_id,
           unnest(sqlc.arg(trace_contexts)::text[]) AS trace_context,
           unnest(sqlc.arg(client_msg_ids)::uuid[]) AS client_msg_id
) AS r
ORDER BY r.seq
RETURNING seq
)
SELECT count(*)::bigint AS inserted, pg_notify('event_log', max(seq)::text)::text AS notified FROM inserted;

-- name: HeadSeq :one
-- The head of the log, 0 when it is empty; it equals the committed counter because the partition
-- holding the head is never dropped.
SELECT COALESCE(max(seq), 0)::bigint FROM event_log;

-- name: MinRetainedSeq :one
-- The smallest retained seq, 0 when the log is empty (min_retained_seq of 04 "resync_required").
SELECT COALESCE(min(seq), 0)::bigint FROM event_log;

-- name: EventBySeq :many
-- One event by its seq for the webhook delivery (M2-26); no row when retention removed it.
SELECT seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id,
       channel_id, thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
       receive_action, payload, truncated, redacted_at, correlation_id, trace_context,
       client_msg_id, permission_version
FROM event_log
WHERE seq = sqlc.arg(seq)::bigint;

-- name: EventByID :many
-- One event by its id for the interaction address of an app (M2-25); no row when retention
-- removed it. id is unique together with seq (03 event_log), so at most one row matches.
SELECT seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id,
       channel_id, thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
       receive_action, payload, truncated, redacted_at, correlation_id, trace_context,
       client_msg_id, permission_version
FROM event_log
WHERE id = sqlc.arg(id)::uuid
LIMIT 1;

-- name: EventsAfter :many
-- One batch of the log tail: the events after a seq in seq order.
SELECT seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id,
       channel_id, thread_id, resource_kind, resource_id, resource_version, audience_principal_id,
       receive_action, payload, truncated, redacted_at, correlation_id, trace_context,
       client_msg_id, permission_version
FROM event_log
WHERE seq > sqlc.arg(after_seq)::bigint
ORDER BY seq
LIMIT sqlc.arg(max_rows)::int;

-- name: EnsurePartitions :one
-- Creates partitions ahead of the head through the SECURITY DEFINER function of bothy_owner.
SELECT sync_event_log_ensure_partitions(sqlc.arg(size)::bigint, sqlc.arg(ahead)::int, sqlc.arg(head)::bigint)::int;

-- name: DropExpiredPartitions :one
-- Detaches and drops expired partitions through the SECURITY DEFINER function of bothy_owner.
SELECT sync_event_log_drop_expired(sqlc.arg(cutoff)::timestamptz, sqlc.arg(head)::bigint, sqlc.arg(lock_timeout)::interval)::int;

-- name: LockResourceEvents :many
-- The stored events of one resource for the redaction of a deletion (03 section 6.5), locked in
-- seq order through the index (resource_kind, resource_id).
SELECT seq, type, schema_version, payload
FROM event_log
WHERE resource_kind = sqlc.arg(resource_kind)::text AND resource_id = sqlc.arg(resource_id)::uuid
ORDER BY seq
FOR UPDATE;

-- name: RedactEvents :execrows
-- Writes the redacted payloads of the locked events and sets redacted_at once (R-DM-18): bothy_app
-- holds the column right UPDATE (payload, redacted_at) only, so no other column can change.
UPDATE event_log AS e
SET payload = r.payload::jsonb, redacted_at = COALESCE(e.redacted_at, now())
FROM (
    SELECT unnest(sqlc.arg(seqs)::bigint[]) AS seq, unnest(sqlc.arg(payloads)::text[]) AS payload
) AS r
WHERE e.seq = r.seq;
