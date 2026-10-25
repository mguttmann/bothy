-- SPDX-License-Identifier: Apache-2.0
-- The statistics reads of runs (M6-05, runs.StatsSource). runs has no channel_id (D-M2-05-9): the
-- stats use case resolves the channels the caller may read (Scope) to their threads through chat
-- and passes that thread set; every read keeps only those threads and never widens the set
-- (tools/lint/sql_visibility.yaml, category system_aggregate). group_ids is parallel to
-- thread_ids (the space of each thread); by_agent groups by the agent instead. Only counts and
-- durations leave, never the triggering person.

-- name: RunStateStats :many
-- The ended runs per bucket of finished_at, group, end state and state_reason.
SELECT date_trunc(sqlc.arg(granularity)::text, r.finished_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    (CASE WHEN sqlc.arg(by_agent)::boolean THEN r.agent_id ELSE g.group_id END)::uuid AS group_id,
    r.state, coalesce(r.state_reason, '')::text AS state_reason, count(*)::bigint AS value
FROM runs r
JOIN (SELECT unnest(sqlc.arg(thread_ids)::uuid[]) AS thread_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
    ON g.thread_id = r.thread_id
WHERE r.thread_id = ANY (sqlc.arg(thread_ids)::uuid[]) AND r.state IN ('completed', 'failed', 'cancelled')
  AND r.finished_at >= sqlc.arg(from_at) AND r.finished_at < sqlc.arg(to_at)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR r.agent_id = sqlc.narg(agent_id)::uuid)
GROUP BY 1, 2, 3, 4
ORDER BY 1, 2, 3, 4;

-- name: RunDurationStats :many
-- The duration percentiles of the ended runs that started, per bucket of finished_at and group,
-- computed here with percentile_cont so no sample leaves the database.
SELECT date_trunc(sqlc.arg(granularity)::text, r.finished_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    (CASE WHEN sqlc.arg(by_agent)::boolean THEN r.agent_id ELSE g.group_id END)::uuid AS group_id,
    (percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM r.finished_at - r.started_at) * 1000))::float8 AS p50,
    (percentile_cont(0.95) WITHIN GROUP (ORDER BY extract(epoch FROM r.finished_at - r.started_at) * 1000))::float8 AS p95
FROM runs r
JOIN (SELECT unnest(sqlc.arg(thread_ids)::uuid[]) AS thread_id, unnest(sqlc.arg(group_ids)::uuid[]) AS group_id) g
    ON g.thread_id = r.thread_id
WHERE r.thread_id = ANY (sqlc.arg(thread_ids)::uuid[]) AND r.state IN ('completed', 'failed', 'cancelled')
  AND r.started_at IS NOT NULL
  AND r.finished_at >= sqlc.arg(from_at) AND r.finished_at < sqlc.arg(to_at)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR r.agent_id = sqlc.narg(agent_id)::uuid)
GROUP BY 1, 2
ORDER BY 1, 2;

-- name: ToolStats :many
-- The tool calls (run_steps of kind tool_call) per bucket of their start and tool name: calls,
-- failed and the median duration of the finished calls (-1 when none finished). Only the name
-- leaves, never title, detail, input or output (25 section 1 rule 3).
SELECT date_trunc(sqlc.arg(granularity)::text, s.started_at, sqlc.arg(time_zone)::text)::timestamptz AS bucket,
    s.name, count(*)::bigint AS calls, (count(*) FILTER (WHERE s.status = 'failed'))::bigint AS failed,
    coalesce(percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM s.finished_at - s.started_at) * 1000)
        FILTER (WHERE s.finished_at IS NOT NULL), -1)::float8 AS p50
FROM run_steps s
JOIN runs r ON r.id = s.run_id
WHERE r.thread_id = ANY (sqlc.arg(thread_ids)::uuid[]) AND s.kind = 'tool_call'
  AND s.started_at >= sqlc.arg(from_at) AND s.started_at < sqlc.arg(to_at)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR r.agent_id = sqlc.narg(agent_id)::uuid)
GROUP BY 1, 2
ORDER BY 1, 2;

-- name: TimelineRuns :many
-- The runs of the timeline (M6-14, docs/spec/25-visualization.md 6.5): the runs of the thread set
-- whose interval [queued_at, finished_at) intersects [from_at, to_at), a run without finished_at
-- reaching to now, ordered by queued_at and id and limited by the caller to
-- stats.timeline_max_runs plus one. Only the interval, state and agent leave, never the
-- triggering person or a title.
SELECT r.id, r.thread_id, r.agent_id, r.state, r.queued_at, r.started_at, r.finished_at
FROM runs r
WHERE r.thread_id = ANY (sqlc.arg(thread_ids)::uuid[])
  AND r.queued_at < sqlc.arg(to_at) AND (r.finished_at IS NULL OR r.finished_at >= sqlc.arg(from_at))
ORDER BY r.queued_at, r.id
LIMIT sqlc.arg(max_rows);

-- name: TimelineAgents :many
-- The distinct agents of the runs of the thread set whose interval intersects [from_at, to_at):
-- the agents of the backend view (M6-14, docs/spec/25-visualization.md 6.2).
SELECT DISTINCT r.agent_id
FROM runs r
WHERE r.thread_id = ANY (sqlc.arg(thread_ids)::uuid[])
  AND r.queued_at < sqlc.arg(to_at) AND (r.finished_at IS NULL OR r.finished_at >= sqlc.arg(from_at))
ORDER BY r.agent_id;
