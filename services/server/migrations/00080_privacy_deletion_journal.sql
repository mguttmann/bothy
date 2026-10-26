-- SPDX-License-Identifier: Apache-2.0
-- Deletion journal (M7-07): the table deletion_journal_state with its one row and the consumer
-- name privacy.journal in the check of event_consumer_cursors, exactly as
-- docs/spec/03-data-model.md sections 4.10 and 4.12 state them (R-DM-27, R-DM-52).
--
-- deletion_journal_state numbers the journal objects in the object storage (docs/spec/20-privacy.md
-- section 4.6): the port of the object storage has no listing, so the objects carry dense numbers.
-- Only the module privacy reads and writes it (03 principle 12). The row is inserted here, because
-- every reader expects exactly one row; bothy_app gets SELECT and UPDATE only: the row is never
-- inserted or deleted in operation.
--
-- The new check of event_consumer_cursors is added NOT VALID (upgrade.md rule M5) and validated in
-- 00081; the old check goes in the same statement, so a worker of the new release can insert the
-- row privacy.journal at once. The previous binary inserts only push and webhooks, which both
-- checks allow (rule M1).

-- +goose Up
CREATE TABLE deletion_journal_state (
    id          smallint    NOT NULL DEFAULT 1,
    next_number bigint      NOT NULL DEFAULT 1,
    low_number  bigint      NOT NULL DEFAULT 1,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT deletion_journal_state_pkey PRIMARY KEY (id),
    CONSTRAINT deletion_journal_state_id_check CHECK (id = 1)
);

INSERT INTO deletion_journal_state (id) VALUES (1);

GRANT SELECT, UPDATE ON deletion_journal_state TO bothy_app;

ALTER TABLE event_consumer_cursors
    DROP CONSTRAINT event_consumer_cursors_name_check,
    ADD CONSTRAINT event_consumer_cursors_name_check CHECK (name IN ('push', 'webhooks', 'privacy.journal')) NOT VALID;

-- +goose Down
DELETE FROM event_consumer_cursors WHERE name = 'privacy.journal';
ALTER TABLE event_consumer_cursors
    DROP CONSTRAINT event_consumer_cursors_name_check,
    ADD CONSTRAINT event_consumer_cursors_name_check CHECK (name IN ('push', 'webhooks'));
DROP TABLE deletion_journal_state;
