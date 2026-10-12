-- SPDX-License-Identifier: Apache-2.0
-- Module sync (M2-01): the cursors of the worker consumers of the event log
-- (docs/spec/03-datenmodell.md 4.10 event_consumer_cursors, R-DM-27, ADR 0051). Only the module
-- sync reads and writes it (03 principle 12).
--
-- One row per consumer name; a new name is an additive migration of the check (upgrade.md rule
-- M8). No row is inserted here: a worker inserts the row of every registered consumer at the
-- head of the log when it starts, so a consumer that arrives in a later release never replays
-- the events retained since this migration (docs/DECISIONS.md D-M2-01-1). The column default 0
-- stays as 03 writes it. bothy_app has SELECT, INSERT and UPDATE and no DELETE: a cursor row is
-- never removed in operation, and a cursor moved past unprocessed events loses them for good.

-- +goose Up
CREATE TABLE event_consumer_cursors (
    name text NOT NULL,
    last_seq bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT event_consumer_cursors_pkey PRIMARY KEY (name),
    CONSTRAINT event_consumer_cursors_name_check CHECK (name IN ('push', 'webhooks'))
);

GRANT SELECT, INSERT, UPDATE ON event_consumer_cursors TO bothy_app;

-- +goose Down
DROP TABLE event_consumer_cursors;
