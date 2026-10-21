-- SPDX-License-Identifier: Apache-2.0
-- Agent sessions of the agents module (M2-12): agent_sessions exactly as
-- docs/spec/03-datenmodell.md section 4.5 states it, every column with its type, default and
-- nullability. A row maps a thread to an external session of an agent; older rows of the thread are
-- its earlier session references (09 section 9). Enumerations are text with CHECK (R-DM-02). The two
-- partial unique indexes keep one current session per thread and an external session current in at
-- most one thread; the repository inserts with ON CONFLICT DO NOTHING on them, so no unique
-- violation reaches a client (M2-12 design note 16). Retention class: content (03 section 3).
--
-- Deliberate deviation from 03 agent_sessions: the foreign key dev_environment_id ->
-- dev_environments(id) ON DELETE RESTRICT comes with V2-01, which creates that table; the column
-- exists now. cost_entries.agent_session_id gets its key with M2-22.
--
-- runs.agent_session_id gets the foreign key 00053 left out (its deviation list names M2-12). It
-- starts NOT VALID (docs/operations/upgrade.md rule M5) and 00058 validates it.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update agent_sessions,
-- not delete them: a session ends with ended_at and end_reason and stays as the history of its
-- thread (its thread_id is ON DELETE RESTRICT).

-- +goose Up
CREATE TABLE agent_sessions (
    id                     uuid        NOT NULL,
    thread_id              uuid        NOT NULL,
    agent_id               uuid        NOT NULL,
    external_session_ref   text        NOT NULL,
    is_current             boolean     NOT NULL DEFAULT true,
    started                boolean     NOT NULL DEFAULT false,
    model                  text,
    cumulative_cost_micros bigint      NOT NULL DEFAULT 0,
    cost_baseline_micros   bigint      NOT NULL DEFAULT 0,
    currency               text        NOT NULL DEFAULT 'USD',
    cost_is_estimate       boolean     NOT NULL DEFAULT true,
    dev_environment_id     uuid,
    created_at             timestamptz NOT NULL DEFAULT now(),
    ended_at               timestamptz,
    end_reason             text,
    version                bigint      NOT NULL DEFAULT 1,
    CONSTRAINT agent_sessions_pkey PRIMARY KEY (id),
    CONSTRAINT agent_sessions_thread_id_fkey FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE RESTRICT,
    CONSTRAINT agent_sessions_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    CONSTRAINT agent_sessions_end_reason_check CHECK (end_reason IN ('reset', 'rebind', 'import', 'agent_changed')),
    CONSTRAINT agent_sessions_current_check CHECK (is_current = (ended_at IS NULL))
);

-- One current session per thread. The repository inserts with ON CONFLICT DO NOTHING, which
-- covers this index and the next one.
CREATE UNIQUE INDEX agent_sessions_thread_current_key ON agent_sessions (thread_id) WHERE is_current;
-- An external session is current in at most one thread.
CREATE UNIQUE INDEX agent_sessions_agent_ref_current_key ON agent_sessions (agent_id, external_session_ref) WHERE is_current;
-- The session history of a thread (listAgentSessions).
CREATE INDEX agent_sessions_thread_created_idx ON agent_sessions (thread_id, created_at DESC);

ALTER TABLE runs ADD CONSTRAINT runs_agent_session_id_fkey
    FOREIGN KEY (agent_session_id) REFERENCES agent_sessions (id) ON DELETE SET NULL NOT VALID;

GRANT SELECT, INSERT, UPDATE ON agent_sessions TO bothy_app;

-- +goose Down
ALTER TABLE runs DROP CONSTRAINT runs_agent_session_id_fkey;
DROP TABLE agent_sessions;
