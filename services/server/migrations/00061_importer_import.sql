-- SPDX-License-Identifier: Apache-2.0
-- Import sources and imported items of the importer module (M8-03): import_sources and
-- import_items exactly as docs/spec/03-datenmodell.md section 4.11 states them. An item is written
-- pending before its target is created and posted in the same transaction as the target (22 7.2);
-- (source_id, origin_key) makes a repeated batch return the stored target instead of a duplicate
-- (R-IM-04). (target_kind, target_id) answers "where does this object come from". Retention class:
-- import (03 section 3).
--
-- messages.import_item_id gets its foreign key here, because import_items did not exist when
-- M1-56 created messages (00032). It is DEFERRABLE INITIALLY DEFERRED like messages.run_id and
-- starts NOT VALID (upgrade.md rule M5); 00062 validates it.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete both
-- tables (the deletion by retention class import, M7).

-- +goose Up
CREATE TABLE import_sources (
    id          uuid        NOT NULL,
    kind        text        NOT NULL,
    name        text        NOT NULL,
    config      jsonb       NOT NULL,
    state       text        NOT NULL DEFAULT 'draft',
    cursor      jsonb       NOT NULL DEFAULT '{}',
    stats       jsonb       NOT NULL DEFAULT '{}',
    created_by  uuid        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    started_at  timestamptz,
    finished_at timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT import_sources_kind_check CHECK (kind IN ('discord', 'archive')),
    CONSTRAINT import_sources_state_check CHECK (state IN ('draft', 'running', 'paused', 'completed', 'failed'))
);
CREATE INDEX import_sources_state_idx ON import_sources (state);

CREATE TABLE import_items (
    id             uuid        NOT NULL,
    source_id      uuid        NOT NULL,
    origin_key     text        NOT NULL,
    item_kind      text        NOT NULL,
    target_kind    text,
    target_id      uuid,
    state          text        NOT NULL DEFAULT 'pending',
    content_sha256 bytea,
    error          text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    FOREIGN KEY (source_id) REFERENCES import_sources (id) ON DELETE CASCADE,
    UNIQUE (source_id, origin_key),
    CONSTRAINT import_items_item_kind_check CHECK (item_kind IN
        ('space', 'channel', 'forum_tag', 'thread', 'message', 'file', 'user', 'reaction', 'pin')),
    CONSTRAINT import_items_state_check CHECK (state IN ('pending', 'posted', 'skipped', 'failed'))
);
-- Resuming a source by state.
CREATE INDEX import_items_source_state_idx ON import_items (source_id, state);
-- "Where does this object come from".
CREATE INDEX import_items_target_idx ON import_items (target_kind, target_id);

ALTER TABLE messages ADD CONSTRAINT messages_import_item_id_fkey
    FOREIGN KEY (import_item_id) REFERENCES import_items (id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED NOT VALID;

GRANT SELECT, INSERT, UPDATE, DELETE ON import_sources TO bothy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON import_items TO bothy_app;

-- +goose Down
ALTER TABLE messages DROP CONSTRAINT messages_import_item_id_fkey;
DROP TABLE import_items;
DROP TABLE import_sources;
