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
-- The rows from_seq .. to_seq in chain order: the chain walk and the export (M1-50).
SELECT seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id, action, decision,
       deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
FROM audit_events
WHERE seq BETWEEN sqlc.arg(from_seq) AND sqlc.arg(to_seq)
ORDER BY seq;

-- name: Bounds :one
-- The smallest retained and the largest seq; both 0 for an empty log (seq starts at 1; M1-50).
SELECT coalesce(min(seq), 0)::bigint AS first_seq, coalesce(max(seq), 0)::bigint AS head_seq FROM audit_events;

-- name: ListEvents :many
-- One page of listAuditEvents, newest first by seq after the keyset before_seq (M1-50). Every
-- filter is optional; NULL filters nothing.
SELECT seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id, action, decision,
       deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
FROM audit_events
WHERE (sqlc.narg(actor_id)::uuid IS NULL OR actor_principal_id = sqlc.narg(actor_id))
  AND (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action))
  AND (sqlc.narg(decision)::text IS NULL OR decision = sqlc.narg(decision))
  AND (sqlc.narg(resource_kind)::text IS NULL OR resource_kind = sqlc.narg(resource_kind))
  AND (sqlc.narg(resource_id)::uuid IS NULL OR resource_id = sqlc.narg(resource_id))
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR occurred_at >= sqlc.narg(from_at))
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR occurred_at < sqlc.narg(to_at))
  AND (sqlc.narg(before_seq)::bigint IS NULL OR seq < sqlc.narg(before_seq))
ORDER BY seq DESC
LIMIT sqlc.arg(row_limit);

-- name: ExportSpan :one
-- The smallest and the largest seq with occurred_at in [from_at, to_at); both 0 for none
-- (M1-50, exportAuditEvents).
SELECT coalesce(min(seq), 0)::bigint AS first_seq, coalesce(max(seq), 0)::bigint AS last_seq
FROM audit_events
WHERE occurred_at >= sqlc.arg(from_at) AND occurred_at < sqlc.arg(to_at);

-- name: LastCheckpoint :one
-- The checkpoint with the largest seq (M1-50, audit.chain_verify).
SELECT seq, hash, previous_checkpoint_hash, signature, kid
FROM audit_checkpoints
ORDER BY seq DESC
LIMIT 1;

-- name: LockCheckpoints :exec
-- Serialises the insert of checkpoints until the commit, so two runs never fork the chain.
SELECT pg_advisory_xact_lock(hashtext('bothy.audit_checkpoints'));

-- name: InsertCheckpoint :exec
-- A checkpoint the job signed after it recomputed the chain up to seq.
INSERT INTO audit_checkpoints (seq, hash, previous_checkpoint_hash, signature, kid)
VALUES (sqlc.arg(seq), sqlc.arg(hash), sqlc.narg(previous_checkpoint_hash), sqlc.arg(signature), sqlc.arg(kid));

-- name: UnexportedCheckpoints :many
-- The checkpoints not yet at the sink, in seq order (index audit_checkpoints_unexported_idx).
SELECT seq, hash, previous_checkpoint_hash, signature, kid
FROM audit_checkpoints
WHERE exported_at IS NULL
ORDER BY seq
LIMIT sqlc.arg(row_limit);

-- name: MarkExported :exec
-- The sink took checkpoint seq.
UPDATE audit_checkpoints SET exported_at = sqlc.arg(exported_at) WHERE seq = sqlc.arg(seq) AND exported_at IS NULL;

-- name: LatestAnchor :many
-- The newest anchor entry of the trim (M7-05): the row whose details name the last removed seq
-- and hash. :many so that no row is an empty list, not an error.
SELECT seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id, action, decision,
       deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
FROM audit_events
WHERE action = 'audit.retention_trim'
ORDER BY seq DESC
LIMIT 1;

-- name: TrimCandidates :many
-- The rows first_seq .. last_seq with their time, in chain order: the trim removes the prefix
-- whose occurred_at lies before the cutoff (M7-05). Runs as bothy_maint.
SELECT seq, occurred_at FROM audit_events WHERE seq BETWEEN sqlc.arg(first_seq) AND sqlc.arg(last_seq) ORDER BY seq;

-- name: TrimEvents :execrows
-- Removes the rows first_seq .. last_seq; only bothy_maint passes the trigger (03 audit_events).
DELETE FROM audit_events WHERE seq BETWEEN sqlc.arg(first_seq) AND sqlc.arg(last_seq);

-- name: TrimCheckpoints :execrows
-- Removes the checkpoints up to the anchor (M7-05); only bothy_maint passes the trigger.
DELETE FROM audit_checkpoints WHERE seq <= sqlc.arg(last_seq);

-- name: DestroyIPKeys :execrows
-- Destroys at most row_limit keys whose period ended before the cutoff (valid_until before now minus
-- privacy.audit_ip_key_retention): the key is emptied, the row stays (03 audit_ip_keys, DP-T06).
UPDATE audit_ip_keys SET key_ciphertext = NULL, destroyed_at = sqlc.arg(destroyed_at)
WHERE id IN (
    SELECT k.id FROM audit_ip_keys k
    WHERE k.destroyed_at IS NULL AND k.valid_until < sqlc.arg(cutoff)
    ORDER BY k.valid_until
    LIMIT sqlc.arg(row_limit)
);

-- name: LastExportedCheckpoint :one
-- The newest checkpoint audit.checkpoint_sink holds (M7-05): the trim never removes its row or a
-- later one, so no row leaves before a checkpoint outside the database sealed it.
SELECT seq, hash, previous_checkpoint_hash, signature, kid
FROM audit_checkpoints
WHERE exported_at IS NOT NULL
ORDER BY seq DESC
LIMIT 1;

-- name: PersonalEntries :many
-- One page of the entries of a person for the privacy export (M7-06, 20 section 5.2): the person
-- as actor or as resource of kind user or principal, ascending by seq after the keyset after_seq.
-- Only the columns the export writes; no hashes and no details (they may hold the free-text
-- reason of another person and the IP pseudonym).
SELECT seq, occurred_at, actor_principal_id, action, decision, deny_reason, resource_kind, resource_id
FROM audit_events
WHERE seq > sqlc.arg(after_seq)
  AND (actor_principal_id = sqlc.arg(subject)
       OR (resource_kind IN ('user', 'principal') AND resource_id = sqlc.arg(subject)))
ORDER BY seq
LIMIT sqlc.arg(row_limit);
