-- SPDX-License-Identifier: Apache-2.0
-- Module instance (M1-47): feature_flags exactly as docs/spec/03-datenmodell.md section 4.1 states
-- it, retention class config. Kind, description, default, module and remove_by live in the code
-- registry of internal/instance (ADR 0064), so no column holds them; a registered key without a
-- row has its default and version 0 in the API, so no row is inserted here. rollout follows the
-- schema feature_flag_rollout.v1 ({user_ids?, group_ids?}, stored normalized). version is the
-- optimistic lock (03 Grundsatz 10).
--
-- Grants (migrations/README.md, grant rule): bothy_app reads, inserts and updates; a row is deleted
-- only by the migration that removes its flag (docs/operations/environments-and-flags.md 2.5).

-- +goose Up
CREATE TABLE feature_flags (
    key        text        NOT NULL,
    enabled    boolean     NOT NULL DEFAULT false,
    rollout    jsonb       NOT NULL DEFAULT '{}',
    version    bigint      NOT NULL DEFAULT 1,
    updated_by uuid,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (key),
    FOREIGN KEY (updated_by) REFERENCES principals (id) ON DELETE RESTRICT
);

GRANT SELECT, INSERT, UPDATE ON feature_flags TO bothy_app;

-- +goose Down
DROP TABLE feature_flags;
