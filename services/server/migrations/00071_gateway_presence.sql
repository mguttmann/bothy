-- SPDX-License-Identifier: Apache-2.0
-- The shared presence state of the gateway (M7-17, R-DM-51): gateway_presence exactly as
-- docs/spec/03-data-model.md section 4.16 states it. UNLOGGED: the contents are worthless after a
-- crash of the database, and every api node writes its rows again within one
-- cluster.presence_flush_interval (an upsert of all its live connections). One row per counted
-- live connection of a human (gateway session); node_id is the process instance that holds it.
-- No foreign key: a row lives only as long as its connection and is not evidence. Retention class:
-- transient (the node deletes its rows when a connection ends and all of them on an orderly
-- shutdown; readers ignore rows older than cluster.presence_ttl, and the gateway deletes them).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete.

-- +goose Up
CREATE UNLOGGED TABLE gateway_presence (
    gateway_session_id uuid        NOT NULL,
    node_id            uuid        NOT NULL,
    principal_id       uuid        NOT NULL,
    device_id          uuid        NOT NULL,
    focus_thread_ids   uuid[]      NOT NULL DEFAULT '{}',
    idle               boolean     NOT NULL DEFAULT false,
    updated_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (gateway_session_id)
);
-- The worker reads the rows of the candidates of a hint by person.
CREATE INDEX gateway_presence_principal_id_idx ON gateway_presence (principal_id);
-- A node replaces and deletes its own rows.
CREATE INDEX gateway_presence_node_id_idx ON gateway_presence (node_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON gateway_presence TO bothy_app;

-- +goose Down
DROP TABLE gateway_presence;
