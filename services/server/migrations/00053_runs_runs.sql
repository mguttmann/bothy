-- SPDX-License-Identifier: Apache-2.0
-- Runs of the runs module (M2-05): runs exactly as docs/spec/03-datenmodell.md section 4.8 states
-- it, every column with its type, default and nullability. A run is one agent turn and at the same
-- time the live box (ADR 0025). IDs come from kernel/ids (R-DM-01); enumerations are text with
-- CHECK (R-DM-02). Every state change is a compare-and-swap on state and version (03 runs); the
-- partial unique index runs_thread_active_key keeps at most one active run per thread (R-DM-14),
-- and the repository names its predicate in ON CONFLICT. Retention class: content (03 section 3).
--
-- Deliberate deviations from 03 runs, each added by the ticket that creates the referenced table:
--   - the foreign key runs.agent_session_id -> agent_sessions(id) ON DELETE SET NULL: M2-12.
--   - the foreign key runs.triggering_command_id -> commands(id) ON DELETE SET NULL: M2-16.
--   - the foreign key runs.triggering_interaction_id -> interactions(id) ON DELETE SET NULL: M2-15.
--   - the foreign key runs.dev_environment_id -> dev_environments(id) ON DELETE RESTRICT: V2-01.
-- The four columns exist now. Every other column, default, CHECK, key and index is created as 03
-- states it. The CHECK on the triggering_* columns allows only the column that matches
-- trigger_kind; because trigger_kind holds one value, at most one of them is set, and each may
-- become NULL later through ON DELETE SET NULL.
--
-- messages.run_id gets its foreign key here, because runs did not exist when M1-56 created messages
-- (00032). It is DEFERRABLE INITIALLY DEFERRED (03 messages: "aufgeschoben"), so the live-box anchor
-- and its run may be written in either order inside one transaction, and it starts NOT VALID
-- (docs/operations/upgrade.md rule M5); 00055 validates it.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update runs, not
-- delete them: a run is the history of its thread and ends with an end state, never with a delete
-- of the server (its thread_id is ON DELETE RESTRICT).

-- +goose Up
CREATE TABLE runs (
    id                        uuid        NOT NULL,
    thread_id                 uuid        NOT NULL,
    agent_id                  uuid        NOT NULL,
    agent_session_id          uuid,
    trigger_kind              text        NOT NULL,
    triggering_message_id     uuid,
    triggering_command_id     uuid,
    triggering_interaction_id uuid,
    triggered_by              uuid        NOT NULL,
    state                     text        NOT NULL DEFAULT 'queued',
    state_reason              text,
    version                   bigint      NOT NULL DEFAULT 1,
    billing_mode_expected     text        NOT NULL,
    billing_mode_used         text,
    billing_evidence          jsonb       NOT NULL DEFAULT '{}',
    invocation_jti            uuid        NOT NULL,
    started_jti               uuid,
    renewed_from_jti          uuid,
    dev_environment_id        uuid,
    model                     text,
    status_line               text,
    last_text_excerpt         text,
    plan                      jsonb       NOT NULL DEFAULT '{}',
    metrics                   jsonb       NOT NULL DEFAULT '{}',
    cost_micros               bigint      NOT NULL DEFAULT 0,
    currency                  text        NOT NULL DEFAULT 'USD',
    cost_is_estimate          boolean     NOT NULL DEFAULT true,
    summary                   text,
    stall_warned_at           timestamptz,
    queued_at                 timestamptz NOT NULL DEFAULT now(),
    invocation_held_until     timestamptz,
    started_at                timestamptz,
    finished_at               timestamptz,
    duration_ms               bigint,
    PRIMARY KEY (id),
    UNIQUE (invocation_jti),
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE RESTRICT,
    FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (triggering_message_id) REFERENCES messages (id) ON DELETE SET NULL,
    FOREIGN KEY (triggered_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CHECK (trigger_kind IN ('message', 'command', 'interaction')),
    CHECK (state IN ('queued', 'running', 'waiting_input', 'completed', 'failed', 'cancelled')),
    CHECK (billing_mode_expected IN ('personal', 'organization')),
    CHECK (state NOT IN ('completed', 'failed', 'cancelled') OR finished_at IS NOT NULL),
    CHECK ((triggering_message_id IS NULL OR trigger_kind = 'message')
        AND (triggering_command_id IS NULL OR trigger_kind = 'command')
        AND (triggering_interaction_id IS NULL OR trigger_kind = 'interaction'))
);

-- At most one active run per thread (R-DM-14). The repository inserts with
-- ON CONFLICT (thread_id) WHERE state IN ('queued', 'running', 'waiting_input') DO NOTHING, which
-- infers exactly this index.
CREATE UNIQUE INDEX runs_thread_active_key ON runs (thread_id)
    WHERE state IN ('queued', 'running', 'waiting_input');
-- The run history of a thread (listThreadRuns).
CREATE INDEX runs_thread_queued_idx ON runs (thread_id, queued_at DESC);
-- "Running now" and resumption (listActiveRuns, the gauge bothy.run.active).
CREATE INDEX runs_state_agent_active_idx ON runs (state, agent_id)
    WHERE state IN ('queued', 'running', 'waiting_input');
-- Statistics per agent (M6).
CREATE INDEX runs_agent_finished_idx ON runs (agent_id, finished_at DESC);

ALTER TABLE messages ADD CONSTRAINT messages_run_id_fkey
    FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED NOT VALID;

GRANT SELECT, INSERT, UPDATE ON runs TO bothy_app;

-- +goose Down
ALTER TABLE messages DROP CONSTRAINT messages_run_id_fkey;
DROP TABLE runs;
