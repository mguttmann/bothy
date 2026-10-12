-- SPDX-License-Identifier: Apache-2.0
-- Module sync (M1-20): idempotency keys of the Idempotency-Key stage (docs/spec/03-datenmodell.md
-- table idempotency_keys, 02 "Idempotenz im Unit of Work", 04 "Alle anderen Schreibvorgänge").
-- Only the module sync reads and writes it (03 principle 12). Retention class transient: the job
-- sync.idempotency_cleanup deletes rows after expires_at, which the application sets to
-- created_at + sync.idempotency_key_ttl from kernel/clock (03 principle 2).
--
-- bothy_app inserts the row in state in_progress, completes it in the same transaction, removes
-- an expired row before it claims the key again and deletes expired rows in the cleanup job, so it
-- gets SELECT, INSERT, UPDATE and DELETE.

-- +goose Up
CREATE TABLE idempotency_keys (
    principal_id    uuid        NOT NULL,
    key             uuid        NOT NULL,
    action          text        NOT NULL,
    request_sha256  bytea       NOT NULL,
    state           text        NOT NULL DEFAULT 'in_progress'
                                CHECK (state IN ('in_progress', 'completed')),
    response_status smallint,
    response_body   jsonb,
    resource_id     uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    PRIMARY KEY (principal_id, key),
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE CASCADE
);
-- The cleanup job deletes in the order of expires_at.
CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON idempotency_keys TO bothy_app;

-- +goose Down
DROP TABLE idempotency_keys;
