-- SPDX-License-Identifier: Apache-2.0
-- The status page (docs/spec/03-data-model.md 4.12, docs/spec/16-operations.md 12, M7-09):
-- components, incidents, their updates and their components. Only the module status runs SQL on
-- these tables (03 principle 12).

-- name: ListComponents :many
SELECT id, name, description, sort_key, state, health_source, agent_id, updated_at
FROM status_components
ORDER BY sort_key, id;

-- name: LockComponents :many
-- Every writer of the page locks all components first, in one order, so the owner and the job
-- status.probe never interleave and never deadlock.
SELECT id, name, description, sort_key, state, health_source, agent_id, updated_at
FROM status_components
ORDER BY sort_key, id
FOR UPDATE;

-- name: InsertComponent :execrows
-- A name another component has (unique lower(name)) inserts nothing.
INSERT INTO status_components (id, name, description, sort_key, state, health_source, agent_id, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(description), sqlc.arg(sort_key), sqlc.arg(state),
        sqlc.narg(health_source), sqlc.narg(agent_id), sqlc.arg(updated_at))
ON CONFLICT ((lower(name))) DO NOTHING;

-- name: UpdateComponent :exec
UPDATE status_components
SET name = sqlc.arg(name), description = sqlc.arg(description), state = sqlc.arg(state),
    health_source = sqlc.narg(health_source), agent_id = sqlc.narg(agent_id), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: SetComponentSortKeys :exec
UPDATE status_components c
SET sort_key = k.sort_key
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(sort_keys)::text[]) AS sort_key) k
WHERE c.id = k.id;

-- name: SetComponentState :exec
UPDATE status_components SET state = sqlc.arg(state), updated_at = sqlc.arg(updated_at) WHERE id = sqlc.arg(id);

-- name: DeleteComponent :execrows
DELETE FROM status_components WHERE id = sqlc.arg(id);

-- name: DeleteAgentComponents :many
DELETE FROM status_components WHERE agent_id = sqlc.arg(agent_id) RETURNING id;

-- name: AgentComponentIDs :many
SELECT id FROM status_components WHERE agent_id = sqlc.arg(agent_id) ORDER BY id;

-- name: HideIncidentsOfGoneAgentComponents :exec
-- Before the components component_ids are deleted (their links go by cascade): every incident
-- that names one of them belonging to an agent and no component outside them is hidden for good,
-- because its text may name the agent and an incident without components is shown to everybody
-- (D-M7-09-5).
UPDATE status_incidents i SET hidden = true
WHERE i.id IN (SELECT l.incident_id
               FROM status_incident_components l
               JOIN status_components c ON c.id = l.component_id
               WHERE l.component_id = ANY (sqlc.arg(component_ids)::uuid[])
                 AND (c.agent_id IS NOT NULL OR c.health_source = 'agent.presence'))
  AND NOT EXISTS (SELECT 1 FROM status_incident_components o
                  WHERE o.incident_id = i.id AND o.component_id <> ALL (sqlc.arg(component_ids)::uuid[]));

-- name: ListIncidents :many
-- The open incidents and the ones resolved at or after the cutoff (status.history_window); the
-- index (state, started_at DESC) serves the open ones.
SELECT id, title, kind, impact, state, started_at, resolved_at, created_by, hidden
FROM status_incidents
WHERE state <> 'resolved' OR resolved_at >= sqlc.arg(resolved_since)
ORDER BY started_at DESC, id DESC;

-- name: LockIncident :one
SELECT id, title, kind, impact, state, started_at, resolved_at, created_by, hidden
FROM status_incidents
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: ListIncidentComponents :many
SELECT incident_id, component_id
FROM status_incident_components
WHERE incident_id = ANY (sqlc.arg(incident_ids)::uuid[])
ORDER BY incident_id, component_id;

-- name: ListIncidentUpdates :many
SELECT id, incident_id, state, body_md, created_by, created_at
FROM status_incident_updates
WHERE incident_id = ANY (sqlc.arg(incident_ids)::uuid[])
ORDER BY incident_id, created_at, id;

-- name: InsertIncident :exec
INSERT INTO status_incidents (id, title, kind, impact, state, started_at, resolved_at, created_by)
VALUES (sqlc.arg(id), sqlc.arg(title), sqlc.arg(kind), sqlc.arg(impact), sqlc.arg(state), sqlc.arg(started_at),
        sqlc.narg(resolved_at), sqlc.narg(created_by));

-- name: InsertIncidentComponents :exec
INSERT INTO status_incident_components (incident_id, component_id)
SELECT sqlc.arg(incident_id), unnest(sqlc.arg(component_ids)::uuid[]);

-- name: InsertIncidentUpdate :exec
INSERT INTO status_incident_updates (id, incident_id, state, body_md, created_by, created_at)
VALUES (sqlc.arg(id), sqlc.arg(incident_id), sqlc.arg(state), sqlc.arg(body_md), sqlc.narg(created_by), sqlc.arg(created_at));

-- name: SetIncidentState :exec
UPDATE status_incidents SET state = sqlc.arg(state), resolved_at = sqlc.narg(resolved_at) WHERE id = sqlc.arg(id);

-- name: OpenAutomaticIncidents :many
-- The automatic incidents (created_by NULL) that are not resolved, per component.
SELECT c.component_id, i.id
FROM status_incidents i
JOIN status_incident_components c ON c.incident_id = i.id
WHERE i.created_by IS NULL AND i.state <> 'resolved'
ORDER BY c.component_id, i.id;

-- name: LatestAutomaticIncidents :many
-- The newest automatic incident of every component by started_at, with whether the job recorded
-- the end of its outage (an update of the job with state resolved).
SELECT DISTINCT ON (c.component_id) c.component_id, i.id, i.state, i.started_at, i.resolved_at,
       EXISTS (SELECT 1 FROM status_incident_updates u
               WHERE u.incident_id = i.id AND u.created_by IS NULL AND u.state = 'resolved')::boolean AS ended
FROM status_incidents i
JOIN status_incident_components c ON c.incident_id = i.id
WHERE i.created_by IS NULL
ORDER BY c.component_id, i.started_at DESC, i.id DESC;
