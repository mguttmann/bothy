-- SPDX-License-Identifier: Apache-2.0
-- Steps of a run (M2-05): run_steps exactly as docs/spec/03-data-model.md section 4.8 states it.
-- The server gives ordinal (the next one of the run, under the row lock of the run); id comes from
-- the adapter (UUIDv7) so that repetitions are idempotent, or from the server for its own status
-- steps. Rows end with their run and with their parent step through ON DELETE CASCADE.
-- Retention class: content (03 section 3).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete, because
-- the server folds older finished steps into one status step once a run holds
-- run.max_steps_persisted steps (03 run_steps; M2-10), which deletes the folded rows.

-- +goose Up
CREATE TABLE run_steps (
    id             uuid        NOT NULL,
    run_id         uuid        NOT NULL,
    ordinal        integer     NOT NULL,
    parent_step_id uuid,
    kind           text        NOT NULL,
    name           text        NOT NULL,
    title          text        NOT NULL DEFAULT '',
    status         text        NOT NULL DEFAULT 'running',
    detail         jsonb       NOT NULL DEFAULT '{}',
    started_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    PRIMARY KEY (id),
    UNIQUE (run_id, ordinal),
    FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE CASCADE,
    FOREIGN KEY (parent_step_id) REFERENCES run_steps (id) ON DELETE CASCADE,
    CHECK (kind IN ('tool_call', 'status', 'subagent', 'workflow', 'background_job')),
    CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'))
);

-- Open steps of a run; loading in order goes through the unique index (run_id, ordinal).
CREATE INDEX run_steps_running_idx ON run_steps (run_id) WHERE status = 'running';

GRANT SELECT, INSERT, UPDATE, DELETE ON run_steps TO bothy_app;

-- +goose Down
DROP TABLE run_steps;
