-- SPDX-License-Identifier: Apache-2.0
-- Queries on agent_consumer_cursors (agents module, M2-07; docs/spec/03-datenmodell.md). Only this
-- module reads and writes the table (03 Grundsatz 12). Each statement touches one row, appends no
-- event and takes no lock beyond its row, so none of them needs event_seq or NOTIFY (02 section
-- 11.7, 03 section 6.3).

-- name: TakeConsumer :one
-- The takeover of a consumer (04 section 15.1 items 2 and 3): a missing row is created at the
-- head, an existing one keeps acked_seq and gets the new holder. created reports the insert
-- (xmax = 0 only for a row this statement inserted).
INSERT INTO agent_consumer_cursors (agent_id, consumer_name, acked_seq, holder_session_id)
VALUES (@agent_id, @consumer_name, @head_seq, @holder_session_id)
ON CONFLICT (agent_id, consumer_name) DO UPDATE
    SET holder_session_id = EXCLUDED.holder_session_id, updated_at = now()
RETURNING acked_seq, (xmax = 0)::boolean AS created;

-- name: SaveConsumerAck :execrows
-- The fenced save of a held acknowledgement (R-SY-23): monotone, and only while the holder still
-- holds the row; no row means another connection took the consumer over.
UPDATE agent_consumer_cursors
SET acked_seq = GREATEST(acked_seq, @acked_seq), updated_at = now()
WHERE agent_id = @agent_id AND consumer_name = @consumer_name AND holder_session_id = @holder_session_id;

-- name: RestartConsumer :execrows
-- The fenced restart of a consumer at the head after resync_required (retention, cursor_ahead).
UPDATE agent_consumer_cursors
SET acked_seq = @acked_seq, updated_at = now()
WHERE agent_id = @agent_id AND consumer_name = @consumer_name AND holder_session_id = @holder_session_id;

-- name: AckConsumer :one
-- ackAgentEvents: monotone and immediate, without holder condition, so an acknowledgement from
-- another process can never lower acked_seq.
UPDATE agent_consumer_cursors
SET acked_seq = GREATEST(acked_seq, @acked_seq), updated_at = now()
WHERE agent_id = @agent_id AND consumer_name = @consumer_name
RETURNING acked_seq;

-- name: GetConsumer :one
SELECT acked_seq FROM agent_consumer_cursors WHERE agent_id = @agent_id AND consumer_name = @consumer_name;

-- name: MoveAllConsumers :execrows
-- bothy restore finalize (M7-04, 03 6.7 item 1): every consumer of every agent to the head of the
-- restored log, in the caller's transaction, so no adapter catches up events of the old timeline.
UPDATE agent_consumer_cursors SET acked_seq = @head_seq, updated_at = now();
