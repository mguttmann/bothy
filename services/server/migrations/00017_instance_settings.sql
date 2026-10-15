-- SPDX-License-Identifier: Apache-2.0
-- Module instance (M1-46): instance_settings exactly as docs/spec/03-data-model.md section 4.1
-- states it, retention class config. Typed key-value pairs whose keys, value schemas and defaults
-- live in the code registry of internal/instance/domain (docs/operations/configuration.md
-- section 6); a registered key without a row has its default and version 0 in the API, so no row
-- is inserted here. version is the optimistic lock (03 principle 10); updated_by is NULL only for
-- a value written at setup.
--
-- Grants (migrations/README.md, grant rule): bothy_app reads, inserts and updates; it never
-- deletes a row, because a deleted branding row would lower the branding version, the sum of the
-- row versions (docs/DECISIONS.md M1-46/2).

-- +goose Up
CREATE TABLE instance_settings (
    key        text        NOT NULL,
    value      jsonb       NOT NULL,
    version    bigint      NOT NULL DEFAULT 1,
    updated_by uuid,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (key),
    FOREIGN KEY (updated_by) REFERENCES principals (id) ON DELETE RESTRICT
);

GRANT SELECT, INSERT, UPDATE ON instance_settings TO bothy_app;

-- +goose Down
DROP TABLE instance_settings;
