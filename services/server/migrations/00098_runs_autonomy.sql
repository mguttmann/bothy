-- SPDX-License-Identifier: Apache-2.0
-- The counts of autonomous inputs of the runs module (AR-04): run_autonomy exactly as
-- docs/spec/03-data-model.md section 4.8 states it (R-DM-60, docs/spec/09-agents.md 5.8.6): the
-- chain and idle counts per thread and person. A run of the person that is not a wake run, or a
-- follow-up of the person, sets both counts to 0 and clears ended_reason in its own transaction.
-- Retention class: content (03 section 3).
--
-- Expand only (upgrade.md rule M1): a new table that the previous binary never reads.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update run_autonomy; a
-- reset is an update, and the rows go with their thread or person (ON DELETE CASCADE).

-- +goose Up
CREATE TABLE run_autonomy (
    thread_id    uuid        NOT NULL,
    person_id    uuid        NOT NULL,
    chain_count  integer     NOT NULL DEFAULT 0,
    idle_count   integer     NOT NULL DEFAULT 0,
    ended_reason text,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT run_autonomy_pkey PRIMARY KEY (thread_id, person_id),
    CONSTRAINT run_autonomy_thread_id_fkey FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    CONSTRAINT run_autonomy_person_id_fkey FOREIGN KEY (person_id) REFERENCES principals (id) ON DELETE CASCADE,
    CONSTRAINT run_autonomy_counts_check CHECK (chain_count >= 0 AND idle_count >= 0)
);

GRANT SELECT, INSERT, UPDATE ON run_autonomy TO bothy_app;

-- +goose Down
DROP TABLE run_autonomy;
