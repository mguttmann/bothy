-- SPDX-License-Identifier: Apache-2.0
-- Status page (M7-09): status_components, status_incidents, status_incident_updates and
-- status_incident_components exactly as docs/spec/03-data-model.md section 4.12 states them. The
-- tables are new and empty, so their keys, checks and indexes are created with them. Only the
-- module status reads and writes them (03 principle 12): the owner maintains components and
-- incidents (status.manage), the job status.probe of the worker sets the automatic states and
-- incidents (docs/spec/16-operations.md section 12), and the job retention.ops removes old rows
-- of the class ops (docs/spec/20-privacy.md section 4.1), including old rows of backup_runs, for
-- which bothy_app gets DELETE here.

-- +goose Up
CREATE TABLE status_components (
    id            uuid        NOT NULL,
    name          text        NOT NULL,
    description   text        NOT NULL DEFAULT '',
    sort_key      text        COLLATE "C" NOT NULL,
    state         text        NOT NULL DEFAULT 'operational'
                              CHECK (state IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    health_source text,
    agent_id      uuid,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX status_components_name_lower_key ON status_components (lower(name));
CREATE INDEX status_components_sort_key_idx ON status_components (sort_key);

CREATE TABLE status_incidents (
    id          uuid        NOT NULL,
    title       text        NOT NULL,
    kind        text        NOT NULL DEFAULT 'incident' CHECK (kind IN ('incident', 'maintenance')),
    impact      text        NOT NULL CHECK (impact IN ('none', 'minor', 'major', 'critical')),
    state       text        NOT NULL DEFAULT 'investigating'
                            CHECK (state IN ('scheduled', 'investigating', 'identified', 'monitoring', 'resolved')),
    started_at  timestamptz NOT NULL,
    resolved_at timestamptz,
    created_by  uuid,
    PRIMARY KEY (id),
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT
);
CREATE INDEX status_incidents_state_started_at_idx ON status_incidents (state, started_at DESC);

CREATE TABLE status_incident_updates (
    id          uuid        NOT NULL,
    incident_id uuid        NOT NULL,
    state       text        NOT NULL
                            CHECK (state IN ('scheduled', 'investigating', 'identified', 'monitoring', 'resolved')),
    body_md     text        NOT NULL,
    created_by  uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    FOREIGN KEY (incident_id) REFERENCES status_incidents (id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT
);
CREATE INDEX status_incident_updates_incident_id_created_at_idx ON status_incident_updates (incident_id, created_at);

CREATE TABLE status_incident_components (
    incident_id  uuid NOT NULL,
    component_id uuid NOT NULL,
    PRIMARY KEY (incident_id, component_id),
    FOREIGN KEY (incident_id) REFERENCES status_incidents (id) ON DELETE CASCADE,
    FOREIGN KEY (component_id) REFERENCES status_components (id) ON DELETE CASCADE
);
CREATE INDEX status_incident_components_component_id_idx ON status_incident_components (component_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON status_components, status_incidents, status_incident_updates,
    status_incident_components TO bothy_app;
-- retention.ops removes old backup runs (M7-09); bothy_backup keeps its rights of 00050.
GRANT DELETE ON backup_runs TO bothy_app;

-- +goose Down
REVOKE DELETE ON backup_runs FROM bothy_app;
DROP TABLE status_incident_components;
DROP TABLE status_incident_updates;
DROP TABLE status_incidents;
DROP TABLE status_components;
