-- SPDX-License-Identifier: Apache-2.0
-- run_followups (docs/spec/03-datenmodell.md section 4.8, M2-11). The table has no channel_id:
-- every read names the rows of one run (or one message) whose run the use case authorized in the
-- same unit of work or is changing in it (07 section 7.4 rule 1, exception "loading a resource that
-- has already been checked"; docs/DECISIONS.md D-M2-05-9). tools/lint/sql_visibility.yaml lists
-- each read under after_authorize. Lock order (03 section 6.3): the runs row first, then the
-- follow-up rows in ascending message_id, then messages.

-- name: InsertFollowup :exec
-- A new follow-up of the active run (state queued); the unique message_id is the last line
-- against a second row of one message.
INSERT INTO run_followups (run_id, message_id) VALUES (sqlc.arg(run_id), sqlc.arg(message_id));

-- name: LockOpenFollowups :many
-- The open follow-ups of an ended run, locked in ascending message_id (03 section 6.3).
SELECT run_id, message_id, state FROM run_followups
WHERE run_id = sqlc.arg(run_id) AND state IN ('queued', 'delivered')
ORDER BY message_id
FOR UPDATE;

-- name: RunFollowups :many
-- Every follow-up of one run for its run document, in ascending message_id.
SELECT run_id, message_id, state FROM run_followups
WHERE run_id = sqlc.arg(run_id)
ORDER BY message_id;

-- name: FollowupStatesOf :many
-- The states of the given messages among the follow-ups of one run (the patch of M2-10).
SELECT run_id, message_id, state FROM run_followups
WHERE run_id = sqlc.arg(run_id) AND message_id = ANY (sqlc.arg(message_ids)::uuid[])
ORDER BY message_id;

-- name: LockFollowup :one
-- One follow-up of one run with its row lock.
SELECT run_id, message_id, state FROM run_followups
WHERE run_id = sqlc.arg(run_id) AND message_id = sqlc.arg(message_id)
FOR UPDATE;

-- name: SetFollowupStates :exec
-- The new state of the given follow-ups of one run; the caller holds their row locks.
UPDATE run_followups SET state = sqlc.arg(state), updated_at = now()
WHERE run_id = sqlc.arg(run_id) AND message_id = ANY (sqlc.arg(message_ids)::uuid[]);

-- name: MoveFollowups :exec
-- The rest of the deferred follow-ups of an ended run become queued follow-ups of the follow-on
-- run: the row moves, because 03 allows one row per message.
UPDATE run_followups SET run_id = sqlc.arg(to_run_id), state = 'queued', updated_at = now()
WHERE run_id = sqlc.arg(from_run_id) AND message_id = ANY (sqlc.arg(message_ids)::uuid[]);

-- name: FollowupOfMessage :one
-- The follow-up of one message (a deleted message, M1-58), without a lock.
SELECT run_id, message_id, state FROM run_followups WHERE message_id = sqlc.arg(message_id);

-- name: RaiseRunVersion :one
-- A stored change of the run document outside a patch (a follow-up accepted, refused or deleted):
-- runs.version rises by one (03 runs.version). The caller holds the row lock.
UPDATE runs SET version = version + 1 WHERE id = sqlc.arg(id) RETURNING version;

-- name: LockActiveRunOfThread :one
-- The active run of a thread with its row lock (M2-11 design note 3 a): a run that ended while
-- the statement waited no longer matches and is not returned.
SELECT * FROM runs
WHERE thread_id = sqlc.arg(thread_id) AND state IN ('queued', 'running', 'waiting_input')
FOR UPDATE;
