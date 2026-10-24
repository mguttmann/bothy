-- SPDX-License-Identifier: Apache-2.0
-- The reads of the run context (runs, M2-13; docs/spec/09-agents.md section 4.6). Each names one
-- run, one thread or one session row of the run whose run.context.read the use case authorized
-- first (07 section 7.4 rule 1, exception "loading a resource that has already been checked";
-- tools/lint/sql_visibility.yaml). Neither returns content.

-- name: EarlierSessionRun :one
-- Whether the agent's session row was used by a started run of the thread queued before the run
-- (the difference of resume and bind, 09 9.1), and the triggering message of the latest such run
-- that has one (the floor context_floor_seq, M2-13 design note 5). The thread prefix of
-- runs_thread_queued_idx serves both.
SELECT EXISTS (
    SELECT 1 FROM runs e
    WHERE e.thread_id = @thread_id AND e.agent_id = @agent_id AND e.agent_session_id = @session_id
      AND e.started_at IS NOT NULL AND e.id <> @run_id
      AND (e.queued_at, e.id) < (@queued_at::timestamptz, @run_id::uuid)
) AS used,
(
    SELECT t.triggering_message_id FROM runs t
    WHERE t.thread_id = @thread_id AND t.agent_id = @agent_id AND t.agent_session_id = @session_id
      AND t.started_at IS NOT NULL AND t.id <> @run_id AND t.triggering_message_id IS NOT NULL
      AND (t.queued_at, t.id) < (@queued_at::timestamptz, @run_id::uuid)
    ORDER BY t.queued_at DESC, t.id DESC
    LIMIT 1
)::uuid AS trigger_message_id;

-- name: DeliveredFollowupsOfSession :many
-- The messages among @message_ids that a run of the session row (in the run's thread) received as
-- follow-ups: run_followups in delivered or absorbed (09 4.6 already_delivered, M2-13 design note
-- 9). The unique message_id of run_followups keeps one row per message.
SELECT f.message_id FROM run_followups f
JOIN runs r ON r.id = f.run_id
WHERE r.thread_id = @thread_id AND r.agent_session_id = @session_id AND f.state IN ('delivered', 'absorbed')
  AND f.message_id = ANY(@message_ids::uuid[])
ORDER BY f.message_id;
