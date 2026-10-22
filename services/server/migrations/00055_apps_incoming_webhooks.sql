-- SPDX-License-Identifier: Apache-2.0
-- Incoming webhooks of the apps module (M2-27): incoming_webhooks exactly as
-- docs/spec/03-datenmodell.md states it. An external system writes through one of them into one
-- channel as the app app_id; the token is never stored, only its HMAC-SHA-256 with the server
-- pepper (auth.token_pepper_ref) in token_hash (08 section 12). A webhook is revoked by setting
-- revoked_at; the partial index serves the list of the active webhooks of a channel. Retention
-- class: config (03 section 3).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete.

-- +goose Up
CREATE TABLE incoming_webhooks (
    id           uuid        NOT NULL,
    app_id       uuid        NOT NULL,
    channel_id   uuid        NOT NULL,
    thread_id    uuid,
    name         text        NOT NULL,
    token_hash   bytea       NOT NULL,
    created_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (app_id) REFERENCES apps (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE RESTRICT,
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    UNIQUE (token_hash)
);
CREATE INDEX incoming_webhooks_channel_active_idx ON incoming_webhooks (channel_id) WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON incoming_webhooks TO bothy_app;

-- +goose Down
DROP TABLE incoming_webhooks;
