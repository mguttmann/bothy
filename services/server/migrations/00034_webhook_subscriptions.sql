-- SPDX-License-Identifier: Apache-2.0
-- Outgoing webhook subscriptions of the apps module (M2-26): webhook_subscriptions exactly as
-- docs/spec/03-datenmodell.md section 4.11 states it. A subscription belongs to an app
-- installation or to an agent (the agent transport comes with a later ticket; the column and its
-- check exist already). The v1 secret is never stored: it is derived from the master key, the id
-- and hmac_generation (03 Grundsatz 9, docs/api/webhooks.md section 2.1). Retention class: config
-- (03 section 3).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete; an
-- owner deletes a subscription with deleteWebhookSubscription, and its deliveries go with it.

-- +goose Up
CREATE TABLE webhook_subscriptions (
    id                   uuid        NOT NULL,
    app_installation_id  uuid,
    agent_id             uuid,
    url                  text        NOT NULL,
    event_types          text[]      NOT NULL,
    signing_key_kid      text        NOT NULL,
    hmac_enabled         boolean     NOT NULL DEFAULT false,
    hmac_generation      integer     NOT NULL DEFAULT 0,
    hmac_rotated_at      timestamptz,
    state                text        NOT NULL DEFAULT 'active'
                                     CHECK (state IN ('active', 'paused', 'disabled')),
    consecutive_failures integer     NOT NULL DEFAULT 0,
    last_delivered_seq   bigint      NOT NULL DEFAULT 0,
    created_by           uuid        NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    disabled_at          timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (app_installation_id) REFERENCES app_installations (id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (signing_key_kid) REFERENCES server_signing_keys (kid) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- Exactly one owner of the subscription: an app installation or an agent.
    CHECK ((app_installation_id IS NULL) <> (agent_id IS NULL)),
    CHECK (hmac_enabled OR hmac_generation = 0)
);
-- The delivery worker and the consumer read the subscriptions by state.
CREATE INDEX webhook_subscriptions_state_idx ON webhook_subscriptions (state);
CREATE INDEX webhook_subscriptions_agent_idx ON webhook_subscriptions (agent_id) WHERE agent_id IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON webhook_subscriptions TO bothy_app;

-- +goose Down
DROP TABLE webhook_subscriptions;
