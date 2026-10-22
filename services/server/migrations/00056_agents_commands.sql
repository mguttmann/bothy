-- SPDX-License-Identifier: Apache-2.0
-- Slash commands of the agents module (M2-16): commands exactly as docs/spec/03-data-model.md
-- section 4.4 states it, every column with its type, default and nullability, the checks, the
-- foreign keys with their ON DELETE, the unique partial index and the two indexes. Agents and apps
-- register their commands here (09 section 10.2); a removal sets removed_at and keeps the row,
-- because runs reference it (runs.triggering_command_id). Retention class: config (03 section 3).
--
-- One point the text of 03 leaves implicit: the unique partial index is NULLS NOT DISTINCT.
-- coalesce(scope_space_id, scope_channel_id) is NULL for an instance command, and PostgreSQL
-- treats NULLs as distinct, so without the clause two live instance commands with the same owner
-- and name would pass (docs/DECISIONS.md D-M2-16-1; 03 uses the clause for stats_minutes for the
-- same reason).
--
-- The foreign key runs.triggering_command_id -> commands(id) ON DELETE SET NULL, which 00053
-- deferred to this ticket, joins this file; it starts NOT VALID (docs/operations/upgrade.md rule
-- M5), and 00057 validates it.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update commands, not
-- delete them: the product removes a command by removed_at.

-- +goose Up
CREATE TABLE commands (
    id                 uuid        NOT NULL,
    owner_principal_id uuid        NOT NULL,
    name               text        NOT NULL,
    description        text        NOT NULL,
    scope_kind         text        NOT NULL DEFAULT 'instance',
    scope_space_id     uuid,
    scope_channel_id   uuid,
    options            jsonb       NOT NULL DEFAULT '[]',
    required_level     text        NOT NULL DEFAULT 'trigger',
    control            boolean     NOT NULL DEFAULT false,
    version            bigint      NOT NULL DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    removed_at         timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (owner_principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (scope_space_id) REFERENCES spaces (id) ON DELETE CASCADE,
    FOREIGN KEY (scope_channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    CHECK (name ~ '^[a-z0-9][a-z0-9_-]*$'),
    CHECK (scope_kind IN ('instance', 'space', 'channel')),
    CHECK ((scope_kind = 'instance' AND scope_space_id IS NULL AND scope_channel_id IS NULL)
        OR (scope_kind = 'space' AND scope_space_id IS NOT NULL AND scope_channel_id IS NULL)
        OR (scope_kind = 'channel' AND scope_channel_id IS NOT NULL AND scope_space_id IS NULL)),
    CHECK (required_level IN ('write', 'trigger'))
);

-- One live command per owner, name (without regard to case) and scope target (03 commands).
CREATE UNIQUE INDEX commands_owner_name_scope_key
    ON commands (owner_principal_id, lower(name), scope_kind, coalesce(scope_space_id, scope_channel_id)) NULLS NOT DISTINCT
    WHERE removed_at IS NULL;
-- The command list of an agent or app.
CREATE INDEX commands_owner_live_idx ON commands (owner_principal_id) WHERE removed_at IS NULL;
-- The suggestion list while typing (M2-17).
CREATE INDEX commands_name_prefix_idx ON commands (lower(name) text_pattern_ops) WHERE removed_at IS NULL;

ALTER TABLE runs ADD CONSTRAINT runs_triggering_command_id_fkey
    FOREIGN KEY (triggering_command_id) REFERENCES commands (id) ON DELETE SET NULL NOT VALID;

GRANT SELECT, INSERT, UPDATE ON commands TO bothy_app;

-- +goose Down
ALTER TABLE runs DROP CONSTRAINT runs_triggering_command_id_fkey;
DROP TABLE commands;
