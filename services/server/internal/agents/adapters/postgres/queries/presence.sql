-- SPDX-License-Identifier: Apache-2.0
-- Queries of the presence of agents (agents module, M2-20; docs/spec/03-data-model.md table
-- agents, docs/spec/09-agents.md section 4.8). Only this module reads and writes the table (03
-- Grundsatz 12). A transaction of the heartbeat locks at most one agents row (03 section 6.3).

-- name: GetPresenceRow :one
-- The configured mode and class and the stored reported state of one agent, read on every
-- heartbeat so a configured value changed in another process counts at once.
SELECT principal_id, billing_mode, tool_class, reported_runtime_state, reported_billing_mode, reported_tool_class,
       reported_runtime_version, reported_details, last_heartbeat_at
FROM agents WHERE principal_id = @principal_id;

-- name: LockPresenceRow :one
-- The same row locked for a change of the reported mode or class: exactly one process audits it.
SELECT principal_id, billing_mode, tool_class, reported_runtime_state, reported_billing_mode, reported_tool_class,
       reported_runtime_version, reported_details, last_heartbeat_at
FROM agents WHERE principal_id = @principal_id FOR UPDATE;

-- name: GetPresenceRows :many
-- The configured and stored values of the given agents in ascending id: the targets of a change
-- of the configured values and the stored presence of untracked agents.
SELECT principal_id, billing_mode, tool_class, reported_runtime_state, reported_billing_mode, reported_tool_class,
       reported_runtime_version, reported_details, last_heartbeat_at
FROM agents WHERE principal_id = ANY (@ids::uuid[]) ORDER BY principal_id;

-- name: ListLivePresence :many
-- The agents stored as online or degraded, for the start rule (D-M2-20-6).
SELECT principal_id, billing_mode, tool_class, reported_runtime_state, reported_billing_mode, reported_tool_class,
       reported_runtime_version, reported_details, last_heartbeat_at
FROM agents WHERE reported_runtime_state IN ('online', 'degraded') ORDER BY principal_id;

-- name: WriteReportedChange :exec
-- A change of the reported mode or class, stored at once with the whole reported state of the
-- heartbeat; the only statement that writes reported_billing_mode and reported_tool_class, so
-- neither changes without its audit entry (D-M2-20-3). version stays: it is the ETag of the record.
UPDATE agents
SET reported_billing_mode = sqlc.narg(reported_billing_mode),
    reported_tool_class = sqlc.narg(reported_tool_class),
    reported_runtime_state = @reported_runtime_state,
    reported_runtime_version = sqlc.narg(reported_runtime_version),
    reported_details = @reported_details,
    last_heartbeat_at = @last_heartbeat_at
WHERE principal_id = @principal_id;

-- name: PersistHeartbeat :execrows
-- The monotone flush of the tracker (D-M2-20-5): a row with a fresher heartbeat, stored by
-- another process, is left as it is, so the newest heartbeat wins. No counter, no NOTIFY, no audit.
UPDATE agents
SET reported_runtime_state = @reported_runtime_state,
    reported_runtime_version = sqlc.narg(reported_runtime_version),
    reported_details = @reported_details,
    last_heartbeat_at = @last_heartbeat_at
WHERE principal_id = @principal_id AND (last_heartbeat_at IS NULL OR last_heartbeat_at <= @last_heartbeat_at);

-- name: WriteProjectFolders :exec
-- The folder names of the heartbeat member project_folders (M4-44, R-DM-56), written only when
-- the list changed; no other column, no version, no event.
UPDATE agents SET reported_project_folders = @reported_project_folders::text[] WHERE principal_id = @principal_id;

-- name: GetProjectFolders :one
-- The stored folder names of one agent for listAgentProjectFolders, read after authorize allowed
-- agent.project_folders.read; NULL while the runner never reported them.
SELECT (reported_project_folders IS NOT NULL)::boolean AS reported, coalesce(reported_project_folders, '{}')::text[] AS names
FROM agents WHERE principal_id = @principal_id;
