-- SPDX-License-Identifier: Apache-2.0
-- Components of the runs module (M2-15): message_components exactly as
-- docs/spec/03-data-model.md section 4.8 states it, every column with its type, default and
-- nullability. A component is a button, select or form on a card message (09 section 11); its
-- definition is the JSON object component.v1, checked by runs before the insert. The single-use
-- rule per message locks the active single_use rows of one message in ascending id (03
-- "Single-use rule per message"). Retention class: content (03 section 3).
--
-- Indexes of 03: (expires_at) WHERE state = 'active' for the expiry job, (run_id) WHERE state =
-- 'active' for disabling at the end of the run.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete
-- components; runs deletes them with their message (03 messages "Deletion ... deletes ...
-- components", the subscriber of messaging.MessageDeleted), because a tombstone keeps the message
-- row and the cascade of message_id never fires for it.

-- +goose Up
CREATE TABLE message_components (
    id                  uuid        NOT NULL,
    message_id          uuid        NOT NULL,
    custom_id           text        NOT NULL,
    type                text        NOT NULL,
    definition          jsonb       NOT NULL,
    position            smallint    NOT NULL,
    single_use          boolean     NOT NULL DEFAULT true,
    required_capability text,
    restricted_to       text        NOT NULL DEFAULT 'any',
    run_id              uuid,
    state               text        NOT NULL DEFAULT 'active',
    expires_at          timestamptz,
    used_by             uuid,
    used_at             timestamptz,
    version             bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    UNIQUE (message_id, custom_id),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE SET NULL,
    FOREIGN KEY (used_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CHECK (type IN ('button', 'select', 'form')),
    CHECK (restricted_to IN ('any', 'run_starter', 'owner')),
    CHECK (state IN ('active', 'used', 'expired', 'disabled'))
);

-- The expiry job runs.component_expiry (02 12.3).
CREATE INDEX message_components_expires_active_idx ON message_components (expires_at) WHERE state = 'active';
-- Disabling the open components of a run at its end (09 8 point 5).
CREATE INDEX message_components_run_active_idx ON message_components (run_id) WHERE state = 'active';

GRANT SELECT, INSERT, UPDATE, DELETE ON message_components TO bothy_app;

-- +goose Down
DROP TABLE message_components;
