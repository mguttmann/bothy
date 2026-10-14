-- SPDX-License-Identifier: Apache-2.0
-- Spaces and the personal sidebar state (M1-52): spaces and user_space_states exactly as
-- docs/spec/03-datenmodell.md section 4.5 states them. IDs come from kernel/ids (R-DM-01). Both
-- sort keys are fractional index keys compared bytewise (COLLATE "C", 03 Grundsatz 5). Retention
-- classes: content (spaces) and personal_sync (user_space_states).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update spaces, never
-- delete them (archiving is final, the data stays for retention and export); user_space_states
-- also allows delete (personal data, M7-06).
--
-- grants.target_space_id gets its foreign key here, because spaces did not exist when M1-27
-- created grants (00005). It starts NOT VALID (upgrade.md rule M5) and 00024 validates it.

-- +goose Up
CREATE TABLE spaces (
    id                        uuid        NOT NULL,
    name                      text        NOT NULL,
    icon                      text,
    color                     text,
    sort_key                  text        COLLATE "C" NOT NULL,
    default_operator_agent_id uuid,
    created_by                uuid        NOT NULL,
    created_at                timestamptz NOT NULL DEFAULT now(),
    archived_at               timestamptz,
    version                   bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (default_operator_agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT
);
-- A name is unique case-insensitively among the spaces that are not archived.
CREATE UNIQUE INDEX spaces_name_active_key ON spaces (lower(name)) WHERE archived_at IS NULL;
-- The sidebar reads the spaces that are not archived in their global order.
CREATE INDEX spaces_sort_key_idx ON spaces (sort_key) WHERE archived_at IS NULL;
-- The I1 check reads the spaces of a default operator (03 section 6.4).
CREATE INDEX spaces_default_operator_agent_id_idx ON spaces (default_operator_agent_id);

CREATE TABLE user_space_states (
    user_id              uuid        NOT NULL,
    space_id             uuid        NOT NULL,
    collapsed            boolean     NOT NULL DEFAULT false,
    sort_key             text        COLLATE "C",
    version              bigint      NOT NULL DEFAULT 1,
    updated_by_device_id uuid,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, space_id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (space_id) REFERENCES spaces (id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by_device_id) REFERENCES devices (id) ON DELETE SET NULL
);

ALTER TABLE grants ADD CONSTRAINT grants_target_space_id_fkey
    FOREIGN KEY (target_space_id) REFERENCES spaces (id) ON DELETE RESTRICT NOT VALID;

GRANT SELECT, INSERT, UPDATE ON spaces TO bothy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON user_space_states TO bothy_app;

-- +goose Down
ALTER TABLE grants DROP CONSTRAINT grants_target_space_id_fkey;
DROP TABLE user_space_states;
DROP TABLE spaces;
