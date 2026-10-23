-- SPDX-License-Identifier: Apache-2.0
-- The hold of an agent invocation while an attachment of the triggering message is not final
-- (docs/spec/04-sync-protocol.md 15.3 point 1, 12 section 10, R-DM-44; M3-07). The lock of the
-- run in ReleaseInvocation and in the job runs.invocation_hold_timeout is LockRun of delivery.sql.
-- runs has no channel_id; the runs are named by the finalized file's message or by the job
-- (tools/lint/sql_visibility.yaml, category system_job).

-- name: LockHeldRunsOfMessage :many
-- The queued runs of a message whose agent.invocation is held, locked in ascending id (the lock
-- order of 03 6.3 for several runs). A held run is always queued, so the partial index of the
-- active runs (state, agent_id) narrows the rows; active runs are few (no own index, M3-07).
SELECT id FROM runs
WHERE state = 'queued' AND triggering_message_id = sqlc.arg(message_id) AND invocation_held_until IS NOT NULL
ORDER BY id
FOR UPDATE;

-- name: ReleaseHeldInvocation :execrows
-- The release: the column is cleared and version rises by one like every stored change of a run
-- (03 runs.version), without run.state_changed. A run that ended or was released meanwhile
-- matches no row, so a second release changes nothing.
UPDATE runs
SET invocation_held_until = NULL, version = version + 1
WHERE id = sqlc.arg(id) AND state = 'queued' AND invocation_held_until IS NOT NULL;
