-- SPDX-License-Identifier: Apache-2.0
-- Interactions of the runs module (M2-15): interactions exactly as docs/spec/03-datenmodell.md
-- section 4.8 states it, every column with its type, default and nullability. An interaction is
-- one use of a component by a principal, idempotent by (principal_id, client_interaction_id) (04
-- section 5.1). Its values are the selection or the form values; they leave the server only in the
-- answer to the person and in interaction.created to the agent that owns the component (SEC-T10).
-- Retention class: content (03 section 3).
--
-- The same file adds the foreign key runs.triggering_interaction_id -> interactions(id) ON DELETE
-- SET NULL that 00053 left out until this table exists. It starts NOT VALID
-- (docs/operations/upgrade.md rule M5); 00059 validates it. No run can name an interaction before
-- this table exists, so the validation finds nothing.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update interactions;
-- they are deleted only with their component or message (ON DELETE CASCADE).

-- +goose Up
CREATE TABLE interactions (
    id                    uuid        NOT NULL,
    component_id          uuid        NOT NULL,
    message_id            uuid        NOT NULL,
    principal_id          uuid        NOT NULL,
    client_interaction_id uuid        NOT NULL,
    "values"              jsonb       NOT NULL DEFAULT '{}',
    state                 text        NOT NULL DEFAULT 'received',
    response              jsonb,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    UNIQUE (principal_id, client_interaction_id),
    FOREIGN KEY (component_id) REFERENCES message_components (id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    CHECK (state IN ('received', 'dispatched', 'acknowledged', 'failed', 'rejected'))
);

CREATE INDEX interactions_component_id_idx ON interactions (component_id);
-- The open interactions of the delivery.
CREATE INDEX interactions_open_state_idx ON interactions (state) WHERE state IN ('received', 'dispatched');

GRANT SELECT, INSERT, UPDATE ON interactions TO bothy_app;

ALTER TABLE runs ADD CONSTRAINT runs_triggering_interaction_id_fkey
    FOREIGN KEY (triggering_interaction_id) REFERENCES interactions (id) ON DELETE SET NULL NOT VALID;

-- +goose Down
ALTER TABLE runs DROP CONSTRAINT runs_triggering_interaction_id_fkey;
DROP TABLE interactions;
