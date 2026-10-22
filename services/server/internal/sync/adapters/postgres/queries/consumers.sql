-- SPDX-License-Identifier: Apache-2.0
-- The worker consumers of the event log (M2-01, docs/spec/02-architecture.md
-- "Worker consumers"). The reads of event_log here and ConsumerEventsAfter, which reuses
-- EventsAfter of event_log.sql, read every event without the visible-channel predicate, like the
-- log tail: each consumer job checks push.deliver or webhook_out.deliver for its receiver before
-- it sends (07 "Query pattern" rule 1, R-AZ-11).

-- name: EnsureConsumerCursor :exec
-- Inserts the cursor of a consumer at the head of the log unless it exists (docs/DECISIONS.md
-- D-M2-01-1). A reader that sees seq = k sees every smaller number, and an uncommitted
-- transaction holds only larger ones (04 "Pattern with a counter row" item 3), so max(seq) is safe.
INSERT INTO event_consumer_cursors (name, last_seq)
SELECT sqlc.arg(name)::text, COALESCE(max(seq), 0)::bigint FROM event_log
ON CONFLICT (name) DO NOTHING;

-- name: LockConsumerCursor :many
-- Locks the cursor row of a round; SKIP LOCKED never waits, so a row another worker holds yields
-- no row and the round ends at once (02 "Worker consumers" item 2).
SELECT last_seq FROM event_consumer_cursors
WHERE name = sqlc.arg(name)::text
FOR UPDATE SKIP LOCKED;

-- name: AdvanceConsumerCursor :execrows
-- Moves the cursor at the end of a round, in the round's transaction.
UPDATE event_consumer_cursors SET last_seq = sqlc.arg(last_seq)::bigint, updated_at = now()
WHERE name = sqlc.arg(name)::text;

-- name: ListConsumerCursors :many
-- Every cursor with the head of the log in one statement, for the lag gauges.
SELECT name, last_seq, updated_at,
       (SELECT COALESCE(max(seq), 0) FROM event_log)::bigint AS head_seq
FROM event_consumer_cursors
ORDER BY name;

-- name: OldestEventAfter :many
-- occurred_at of the oldest event after a cursor, for bothy.event.consumer.pending_age.
SELECT occurred_at FROM event_log
WHERE seq > sqlc.arg(after_seq)::bigint
ORDER BY seq
LIMIT 1;

-- name: MoveAllConsumerCursors :execrows
-- bothy restore finalize (M7-04, 03 6.7 item 1): every worker consumer to the head of the
-- restored log, in the caller's transaction.
UPDATE event_consumer_cursors SET last_seq = sqlc.arg(head_seq)::bigint, updated_at = now();
