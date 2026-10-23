-- SPDX-License-Identifier: Apache-2.0
-- Follow-ups of a run (M2-11): run_followups exactly as docs/spec/03-data-model.md section 4.8
-- states it. A human message that would trigger the agent of the thread's active run becomes one
-- row here instead of a second run (09 section 7); the unique message makes it one follow-up per
-- message, also when the rest of a follow-up set moves to the follow-on run. Rows end with their
-- run and with their message through ON DELETE CASCADE.
-- Retention class: content (03 section 3); deleted with the run (20 row run_followups).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update; rows are
-- never deleted by the server, only through the cascade of their run or message.

-- +goose Up
CREATE TABLE run_followups (
    run_id     uuid        NOT NULL,
    message_id uuid        NOT NULL,
    state      text        NOT NULL DEFAULT 'queued',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, message_id),
    UNIQUE (message_id),
    FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    CHECK (state IN ('queued', 'delivered', 'absorbed', 'deferred', 'discarded'))
);

-- The open follow-ups of a run at its end and the states of the run document.
CREATE INDEX run_followups_run_id_state_idx ON run_followups (run_id, state);

GRANT SELECT, INSERT, UPDATE ON run_followups TO bothy_app;

-- +goose Down
DROP TABLE run_followups;
