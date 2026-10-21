-- SPDX-License-Identifier: Apache-2.0
-- Durable read position of an agent adapter in the event log (M2-07): agent_consumer_cursors
-- exactly as docs/spec/03-data-model.md states it. One row per agent and consumer name; the
-- first connection of a consumer creates it at head_seq (04 section 15.1 item 2). holder_session_id
-- is the gateway session or REST window that holds the consumer; every save of acked_seq is fenced
-- with WHERE holder_session_id = <own holder> (R-SY-23). Retention class: security (03 section 3);
-- the rows are not personal and end with their agent (20 "Data categories per table").
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update. Rows are
-- never deleted by the server: they end with the agent through ON DELETE CASCADE.

-- +goose Up
CREATE TABLE agent_consumer_cursors (
    agent_id          uuid        NOT NULL,
    consumer_name     text        NOT NULL DEFAULT 'default',
    acked_seq         bigint      NOT NULL,
    holder_session_id uuid,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, consumer_name),
    FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE CASCADE
);

GRANT SELECT, INSERT, UPDATE ON agent_consumer_cursors TO bothy_app;

-- +goose Down
DROP TABLE agent_consumer_cursors;
