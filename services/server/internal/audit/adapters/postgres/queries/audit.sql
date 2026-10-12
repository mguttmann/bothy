-- SPDX-License-Identifier: Apache-2.0

-- name: InsertRows :exec
-- Inserts the rows of one audit hook run in one statement (M1-24). The text arrays carry ''
-- for NULL, because sqlc maps text[] to []string; details arrive as JSON text.
INSERT INTO audit_events (
    seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id, action, decision,
    deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
)
SELECT r.seq, r.id, r.occurred_at, r.actor_principal_id, r.actor_device_id, r.actor_session_id,
       r.action, r.decision, NULLIF(r.deny_reason, ''), NULLIF(r.resource_kind, ''), r.resource_id,
       r.details::jsonb, r.correlation_id, r.prev_hash, r.hash
FROM (
    -- Set-returning functions in one select list run in lock step, so row i of every array
    -- forms row i (PostgreSQL 10 and later).
    SELECT
           unnest(sqlc.arg(seqs)::bigint[]) AS seq,
           unnest(sqlc.arg(ids)::uuid[]) AS id,
           unnest(sqlc.arg(occurred_ats)::timestamptz[]) AS occurred_at,
           unnest(sqlc.arg(actor_principal_ids)::uuid[]) AS actor_principal_id,
           unnest(sqlc.arg(actor_device_ids)::uuid[]) AS actor_device_id,
           unnest(sqlc.arg(actor_session_ids)::uuid[]) AS actor_session_id,
           unnest(sqlc.arg(actions)::text[]) AS action,
           unnest(sqlc.arg(decisions)::text[]) AS decision,
           unnest(sqlc.arg(deny_reasons)::text[]) AS deny_reason,
           unnest(sqlc.arg(resource_kinds)::text[]) AS resource_kind,
           unnest(sqlc.arg(resource_ids)::uuid[]) AS resource_id,
           unnest(sqlc.arg(details)::text[]) AS details,
           unnest(sqlc.arg(correlation_ids)::uuid[]) AS correlation_id,
           unnest(sqlc.arg(prev_hashes)::bytea[]) AS prev_hash,
           unnest(sqlc.arg(hashes)::bytea[]) AS hash
) AS r
ORDER BY r.seq;

-- name: HashAt :one
-- The hash of one row, the prev_hash of the next.
SELECT hash FROM audit_events WHERE seq = sqlc.arg(seq);

-- name: CurrentIPKey :one
-- The IP key whose period contains at.
SELECT id, valid_from, valid_until, key_ciphertext, key_ref
FROM audit_ip_keys
WHERE valid_from <= sqlc.arg(at) AND valid_until > sqlc.arg(at)
ORDER BY valid_from DESC
LIMIT 1;

-- name: InsertIPKey :exec
-- The key of a new period; a concurrent process that inserted first wins.
INSERT INTO audit_ip_keys (id, valid_from, valid_until, key_ciphertext, key_ref)
VALUES (sqlc.arg(id), sqlc.arg(valid_from), sqlc.arg(valid_until), sqlc.arg(key_ciphertext), sqlc.arg(key_ref))
ON CONFLICT (valid_from) DO NOTHING;

-- name: IPKeyByID :one
-- One IP key by the ip_key_id of an audit row.
SELECT id, valid_from, valid_until, key_ciphertext, key_ref FROM audit_ip_keys WHERE id = sqlc.arg(id);

-- name: Rows :many
-- The rows from_seq .. to_seq in chain order, for recomputing the chain (tests, M1-50).
SELECT seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id, action, decision,
       deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
FROM audit_events
WHERE seq BETWEEN sqlc.arg(from_seq) AND sqlc.arg(to_seq)
ORDER BY seq;
