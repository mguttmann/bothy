-- SPDX-License-Identifier: Apache-2.0
-- The reads of the consumer push of notify (M3-19 design notes 5 and 9 d; docs/spec/13-push-
-- benachrichtigungen.md section 2.1). None carries the visible channel predicate: the consumer
-- has no principal, it reads like the log tail, and notify checks every hint with push.deliver
-- (07 7.4 rule 1). They return positions only, never content.

-- name: LastAnchorBefore :one
-- The newest live box anchor of a run (kind run, the only rows with run_id; 09 section 5.4) in the
-- thread strictly below the position, backwards over messages_thread_seq_key.
SELECT id, run_id, thread_seq
FROM messages
WHERE thread_id = @thread_id AND thread_seq < @before_thread_seq AND kind = 'run'
ORDER BY thread_seq DESC
LIMIT 1;

-- name: AnchorOfRun :one
-- The anchor of one run, over messages_run_id_key.
SELECT id, run_id, thread_seq
FROM messages
WHERE run_id = @run_id;

-- name: LastAgentMessageBefore :one
-- The newest answer of an agent in the thread strictly between the two positions: origin agent,
-- a kind an agent writes as an answer (text, voice, card; the messages check of 03), not a
-- tombstone. Backwards over messages_thread_seq_key.
SELECT id, thread_seq
FROM messages
WHERE thread_id = @thread_id AND author_principal_id = @author_principal_id
    AND thread_seq > @after_thread_seq AND thread_seq < @before_thread_seq
    AND origin = 'agent' AND kind IN ('text', 'voice', 'card') AND deleted_at IS NULL
ORDER BY thread_seq DESC
LIMIT 1;

-- name: NewestMessage :one
-- The newest message of the thread a flush hint points at: not a tombstone, not a run anchor and
-- not a system line. Backwards over messages_thread_seq_key.
SELECT id, thread_seq
FROM messages
WHERE thread_id = @thread_id AND deleted_at IS NULL AND kind NOT IN ('run', 'system')
ORDER BY thread_seq DESC
LIMIT 1;
