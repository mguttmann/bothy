-- SPDX-License-Identifier: Apache-2.0
-- The run an agent's message answers (FEAT-TEAM-CHANNEL-SERVER, red team round 6, R-DM-64,
-- docs/spec/09-agents.md 3.9.4): run_answers exactly as docs/spec/03-data-model.md section 4.8
-- states it. The acceptance of runs writes one row in the unit of work of every agent message
-- whose run_id the answer check accepted, and of every live box anchor. The run context reads it:
-- an agent's message in a context carries the record (run_feeders) of the run it answers, and an
-- agent's message without a row (written outside a run of its thread, or as no answer) speaks for
-- no person. messages.run_id holds only the anchor (kind run), and an answer's run_id is never
-- stored on the message, so this is the only link.
--
-- Expand only (upgrade.md rule M1): a new table that the previous binary never reads; it starts
-- empty and is not backfilled. An answer older than this migration has no row and counts as an
-- agent message without a run, which the check of a member's reply refuses (fail closed); team
-- channels come with 00106 of the same release.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read and insert, never update or
-- delete: a link never changes; rows go with their run (ON DELETE CASCADE). message_id has no
-- foreign key: a deleted message keeps the link, which no read needs any more.

-- +goose Up
CREATE TABLE run_answers (
    message_id uuid        NOT NULL,
    run_id     uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT run_answers_pkey PRIMARY KEY (message_id),
    CONSTRAINT run_answers_run_id_fkey FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE CASCADE
);

-- The foreign key to runs needs an index on the referencing side (migrations/README.md).
CREATE INDEX run_answers_run_id_idx ON run_answers (run_id);

GRANT SELECT, INSERT ON run_answers TO bothy_app;

-- +goose Down
DROP TABLE run_answers;
