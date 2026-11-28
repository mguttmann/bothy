-- SPDX-License-Identifier: Apache-2.0
-- status_component_probe (docs/spec/03-data-model.md 4.12, docs/spec/16-operations.md 12 item 4,
-- AUD srv-stats-status-11): the memory of the job status.probe about each automatic component,
-- which lived in the worker process before. With several workers each process held its own
-- memory, so a passing round seen by one worker did not end the run of failing rounds another
-- worker remembered, and a flapping check could become degraded too early. The rows are written by
-- the job in the transaction of its round, after it has locked every component, so the rounds of
-- several workers serialize on them. A row goes with its component.
--
-- A new, empty table: a missing row means no memory, as after a restart before, so no backfill is
-- needed.

-- +goose Up
CREATE TABLE status_component_probe (
    component_id  uuid        NOT NULL,
    state         text        NOT NULL
                              CHECK (state IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    entered_at    timestamptz NOT NULL,
    out_since     timestamptz,
    failing_since timestamptz,
    last_round    timestamptz NOT NULL,
    PRIMARY KEY (component_id),
    FOREIGN KEY (component_id) REFERENCES status_components (id) ON DELETE CASCADE
);

GRANT SELECT, INSERT, UPDATE, DELETE ON status_component_probe TO bothy_app;

-- +goose Down
DROP TABLE status_component_probe;
