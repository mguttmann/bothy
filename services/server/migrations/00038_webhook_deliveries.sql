-- SPDX-License-Identifier: Apache-2.0
-- Delivery attempts of the outgoing webhooks (M2-26): webhook_deliveries exactly as
-- docs/spec/03-data-model.md section 4.11 states it. One row per event and subscription, created
-- by the worker consumer webhooks; last_error is a code of a closed list, never a response body
-- (docs/api/webhooks.md section 4). Retention class: transient (03 section 3); retention deletes
-- only finished rows (succeeded, failed, dead), never pending ones (M7-05).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete; the
-- rows go with their subscription (ON DELETE CASCADE).

-- +goose Up
CREATE TABLE webhook_deliveries (
    id              uuid        NOT NULL,
    subscription_id uuid        NOT NULL,
    event_seq       bigint      NOT NULL,
    event_id        uuid        NOT NULL,
    attempt         integer     NOT NULL DEFAULT 0,
    status          text        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'succeeded', 'failed', 'dead')),
    response_code   smallint,
    last_error      text,
    next_attempt_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    delivered_at    timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (subscription_id) REFERENCES webhook_subscriptions (id) ON DELETE CASCADE,
    -- One row per event and subscription; the key also serves the head of a subscription.
    UNIQUE (subscription_id, event_seq)
);
-- The worker reads the due pending rows.
CREATE INDEX webhook_deliveries_pending_next_idx ON webhook_deliveries (next_attempt_at) WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE, DELETE ON webhook_deliveries TO bothy_app;

-- +goose Down
DROP TABLE webhook_deliveries;
